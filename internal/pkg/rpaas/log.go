// Copyright 2021 tsuru authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package rpaas

import (
	"context"
	"fmt"
	"regexp"
	"sort"
	"strings"
	"text/template"
	"time"

	"github.com/fatih/color"
	"github.com/stern/stern/stern"
	nginxv1alpha1 "github.com/tsuru/nginx-operator/api/v1alpha1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/labels"
	v1 "k8s.io/client-go/kubernetes/typed/core/v1"
)

var (
	logFuncs = map[string]interface{}{
		"color": func(color color.Color, text string) string {
			return color.SprintFunc()(text)
		},

		"time": func(msg string) string {
			idx := strings.IndexRune(msg, ' ')
			return msg[:idx]
		},

		"message": func(msg string) string {
			idx := strings.IndexRune(msg, ' ')
			return msg[idx+1:]
		},
	}

	logsTemplate = template.Must(template.New("rpaasv2.log").
			Funcs(logFuncs).
			Parse("{{ printf `%s [%s][%s]` (time .Message) .PodName .ContainerName }}: {{ message .Message }}"))

	logsWithColor = template.Must(template.New("rpaasv2.log").
			Funcs(logFuncs).
			Parse("{{ color .PodColor (printf `%s [%s][%s]` (time .Message) .PodName .ContainerName) }}: {{ message .Message }}"))
)

func (m *k8sRpaasManager) Log(ctx context.Context, instanceName string, args LogArgs) error {
	instance, err := m.GetInstance(ctx, instanceName)
	if err != nil {
		return err
	}

	nginx, err := m.getNginx(ctx, instance)
	if err != nil {
		return err
	}

	if args.Since == nil {
		args.Since = func(n int64) *int64 { return &n }(60 * 60 * 24) // last 24 hours
	}

	args.template = logsTemplate
	if args.Color {
		args.template = logsWithColor
	}

	if args.Follow {
		return m.watchLogs(ctx, nginx, args)
	}

	return m.listLogs(ctx, nginx, args)
}

func (m *k8sRpaasManager) listLogs(ctx context.Context, nginx *nginxv1alpha1.Nginx, args LogArgs) error {
	pods, err := m.getPods(ctx, nginx)
	if err != nil {
		return err
	}

	sort.Slice(pods, func(i, j int) bool { return pods[i].Name < pods[j].Name })

	var tails []*stern.Tail
	for _, p := range pods {
		if args.Pod != "" && args.Pod != p.Name {
			continue
		}

		for _, c := range p.Spec.Containers {
			if args.Container != "" && args.Container != c.Name {
				continue
			}

			tails = append(tails, stern.NewTail(m.kcs.CoreV1(), p.Spec.NodeName, p.Namespace, p.Name, c.Name, args.template, args.Stdout, args.Stderr, &stern.TailOptions{
				Timestamps:   true,
				SinceSeconds: *args.Since,
				TailLines:    args.Lines,
				Location:     time.UTC,
			}))
		}
	}

	for _, t := range tails {
		t.Start(ctx)
	}

	return nil
}

func (m *k8sRpaasManager) watchLogs(ctx context.Context, nginx *nginxv1alpha1.Nginx, args LogArgs) error {
	added := make(chan *stern.Target)
	removed := make(chan *stern.Target)
	errCh := make(chan error)
	defer close(added)
	defer close(errCh)
	defer close(removed)

	podRegexp := regexp.MustCompile(`.*`)
	if args.Pod != "" {
		podRegexp = regexp.MustCompile(fmt.Sprintf("^%s$", regexp.QuoteMeta(args.Pod)))
	}

	containerRegexp := regexp.MustCompile(`.*`)
	if args.Container != "" {
		containerRegexp = regexp.MustCompile(fmt.Sprintf("^%s$", regexp.QuoteMeta(args.Container)))
	}

	labelSet, err := labels.ConvertSelectorToLabelsMap(nginx.Status.PodSelector)
	if err != nil {
		return err
	}

	tails := make(map[string]*stern.Tail)
	a, r, err := stern.Watch(ctx, m.kcs.CoreV1().Pods(nginx.Namespace),
		podRegexp, nil,
		containerRegexp, nil,
		false, false,
		[]stern.ContainerState{stern.RUNNING, stern.WAITING, stern.TERMINATED},
		labelSet.AsSelector(),
		fields.Everything(),
	)
	if err != nil {
		return err
	}

	go routeTarget(ctx, a, r, added, removed, errCh)
	go addTail(ctx, m.kcs.CoreV1(), added, tails, args)
	go removeTail(removed, tails)

	select {
	case e := <-errCh:
		return e

	case <-ctx.Done():
		return nil
	}
}

func addTail(ctx context.Context, client v1.CoreV1Interface, added chan *stern.Target, tails map[string]*stern.Tail, args LogArgs) {
	for p := range added {
		tail := stern.NewTail(client, p.Node, p.Namespace, p.Pod, p.Container, args.template, args.Stdout, args.Stderr, &stern.TailOptions{
			Timestamps:   true,
			SinceSeconds: *args.Since,
			TailLines:    args.Lines,
			Follow:       true,
			Location:     time.UTC,
		})

		tails[p.GetID()] = tail

		go func(t *stern.Tail) { t.Start(ctx) }(tail)
	}
}

func removeTail(removed chan *stern.Target, tails map[string]*stern.Tail) {
	for p := range removed {
		t, ok := tails[p.GetID()]
		if !ok {
			continue
		}

		t.Close()
		delete(tails, p.GetID())
	}
}

func routeTarget(ctx context.Context, wAdded, wRemoved, toAdd, toRemove chan *stern.Target, errCh chan error) {
	for {
		select {
		case v, ok := <-wAdded:
			if !ok {
				errCh <- fmt.Errorf("lost watch connection")
				return
			}
			toAdd <- v

		case v, ok := <-wRemoved:
			if !ok {
				errCh <- fmt.Errorf("lost watch connection")
				return
			}
			toRemove <- v

		case <-ctx.Done():
			return
		}
	}
}
