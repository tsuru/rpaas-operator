package rpaas

import (
	"context"
	"fmt"
	"text/template"

	"github.com/stern/stern/stern"
	nginxv1alpha1 "github.com/tsuru/nginx-operator/api/v1alpha1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/labels"
	v1 "k8s.io/client-go/kubernetes/typed/core/v1"
)

func addTail(ctx context.Context, added chan *stern.Target, client v1.CoreV1Interface, template *template.Template, args LogArgs, tails map[string]*stern.Tail) {
	for p := range added {
		tail := stern.NewTail(client, p.Node, p.Namespace, p.Pod, p.Container, template, args.Buffer, args.Buffer, &stern.TailOptions{
			Timestamps:   args.WithTimestamp,
			SinceSeconds: int64(172800),
			Namespace:    false,
			TailLines:    args.Lines,
			Follow:       args.Follow,
		})

		tails[p.GetID()] = tail
		go func(tail *stern.Tail) {
			if err := tail.Start(ctx); err != nil {
				fmt.Fprintf(args.Buffer, "unexpected error: %v\n", err)
			}
		}(tail)
	}
}

func removeTail(removed chan *stern.Target, tails map[string]*stern.Tail) {
	for p := range removed {
		targetID := p.GetID()
		if tail, ok := tails[targetID]; ok {
			tail.Close()
		}
	}
}

func updateChannels(ctx context.Context, wAdded, wRemoved, toAdd, toRemove chan *stern.Target, errCh chan error) {
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

func startTailing(ctx context.Context, args LogArgs, nginx *nginxv1alpha1.Nginx, client v1.CoreV1Interface, sternStates []stern.ContainerState, template *template.Template) error {
	added := make(chan *stern.Target)
	removed := make(chan *stern.Target)
	errCh := make(chan error)
	defer close(added)
	defer close(errCh)
	defer close(removed)
	tails := make(map[string]*stern.Tail)

	var a, r chan *stern.Target
	var err error
	switch args.Follow {
	case true:
		a, r, err = stern.Watch(ctx,
			client.Pods(nginx.Namespace),
			args.Pod,
			nil,
			args.Container,
			nil,
			false,
			false,
			sternStates,
			labels.SelectorFromSet(nginx.Spec.PodTemplate.Labels),
			fields.Everything(),
		)
	case false:
		fmt.Printf("TODO\n")
	}
	if err != nil {
		return err
	}

	go updateChannels(ctx, a, r, added, removed, errCh)
	go addTail(ctx, added, client, template, args, tails)
	go removeTail(removed, tails)

	select {
	case e := <-errCh:
		return e
	case <-ctx.Done():
		return nil
	}
}
