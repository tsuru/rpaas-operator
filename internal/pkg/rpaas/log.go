package rpaas

import (
	"context"
	"fmt"
	"text/template"

	"github.com/stern/stern/stern"
	nginxv1alpha1 "github.com/tsuru/nginx-operator/api/v1alpha1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/labels"

	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/client-go/kubernetes/typed/core/v1"
)

func addTail(ctx context.Context, added chan *stern.Target, client v1.CoreV1Interface, template *template.Template, args LogArgs, tails map[string]*stern.Tail) {
	for p := range added {
		tail := stern.NewTail(client, p.Node, p.Namespace, p.Pod, p.Container, template, args.Buffer, args.Buffer, &stern.TailOptions{
			Timestamps:   args.WithTimestamp,
			SinceSeconds: args.Since,
			Namespace:    false,
			TailLines:    args.Lines,
			Follow:       true,
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

func (m *k8sRpaasManager) tail(ctx context.Context, args LogArgs, nginx *nginxv1alpha1.Nginx, sternStates []stern.ContainerState, template *template.Template) error {
	added := make(chan *stern.Target)
	removed := make(chan *stern.Target)
	errCh := make(chan error)
	defer close(added)
	defer close(errCh)
	defer close(removed)
	tails := make(map[string]*stern.Tail)

	var a, r chan *stern.Target
	var err error
	a, r, err = stern.Watch(ctx,
		m.kcs.CoreV1().Pods(nginx.Namespace),
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
	if err != nil {
		return err
	}

	go updateChannels(ctx, a, r, added, removed, errCh)
	go addTail(ctx, added, m.kcs.CoreV1(), template, args, tails)
	go removeTail(removed, tails)

	select {
	case e := <-errCh:
		return e
	case <-ctx.Done():
		return nil
	}
}

func matchState(states []string, container corev1.ContainerStatus) bool {
	for _, state := range states {
		switch state {
		case "running":
			if container.State.Running != nil {
				return true
			}
		case "terminated":
			if container.State.Terminated != nil {
				return true
			}
		case "waiting":
			if container.State.Waiting != nil {
				return true
			}
		}
	}
	return false
}

func (m *k8sRpaasManager) listLogs(ctx context.Context, args LogArgs, nginx *nginxv1alpha1.Nginx, sternStates []stern.ContainerState, template *template.Template) error {
	pods, err := m.getPods(ctx, nginx)
	if err != nil {
		return err
	}

	tailQueue := []*stern.Tail{}

	for _, pod := range pods {
		if args.Pod.MatchString(pod.Name) {
			for _, c := range pod.Status.ContainerStatuses {
				t := stern.NewTail(m.kcs.CoreV1(), pod.Spec.NodeName, pod.Namespace, pod.Name, c.Name, template, args.Buffer, args.Buffer, &stern.TailOptions{
					Timestamps:   args.WithTimestamp,
					SinceSeconds: args.Since,
					Namespace:    false,
					TailLines:    args.Lines,
					Follow:       false,
				})
				if args.Container.MatchString(c.Name) {
					if matchState(args.ContainerStates, c) {
						tailQueue = append(tailQueue, t)
					}
				}
			}
		}
	}

	for _, tail := range tailQueue {
		if err := tail.Start(ctx); err != nil {
			fmt.Fprintf(args.Buffer, "unexpected error: %v\n", err)
		}
	}
	return nil
}

func (m *k8sRpaasManager) log(ctx context.Context, args LogArgs, nginx *nginxv1alpha1.Nginx, sternStates []stern.ContainerState, template *template.Template) error {
	switch args.Follow {
	case true:
		return m.tail(ctx, args, nginx, sternStates, template)
	default:
		return m.listLogs(ctx, args, nginx, sternStates, template)
	}
}
