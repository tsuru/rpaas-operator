package purge

import (
	"context"
	"fmt"
	"time"

	"github.com/pkg/errors"
	"github.com/tsuru/rpaas-operator/internal/pkg/rpaas"
	"github.com/tsuru/rpaas-operator/internal/pkg/rpaas/nginx"
	"github.com/tsuru/rpaas-operator/pkg/util"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/client-go/informers"
	v1informers "k8s.io/client-go/informers/core/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"
)

// Should be exported from rpaas/k8s.go
const (
	defaultInstanceLabel = "rpaas.extensions.tsuru.io/instance-name"
	informerSyncTimeout  = 10 * time.Second
)

type Watcher struct {
	Client   kubernetes.Interface
	Informer v1informers.PodInformer

	stopCh chan struct{}
}

func NewWatcher(c kubernetes.Interface) (*Watcher, error) {
	return &Watcher{
		Client: c,
		stopCh: make(chan struct{}),
	}, nil
}

func (w *Watcher) Watch() {
	informerFactory := informers.NewFilteredSharedInformerFactory(w.Client, time.Minute, metav1.NamespaceAll, nil)

	w.Informer = informerFactory.Core().V1().Pods()
	w.Informer.Informer()

	informerFactory.Start(w.stopCh)

	w.waitForSync(w.Informer.Informer())
}

func (w *Watcher) Stop() {
	close(w.stopCh)
}

func (w *Watcher) waitForSync(informer cache.SharedInformer) error {
	if informer.HasSynced() {
		return nil
	}
	ctx, cancel := contextWithCancelByChannel(context.Background(), w.stopCh, informerSyncTimeout)
	defer cancel()
	cache.WaitForCacheSync(ctx.Done(), informer.HasSynced)
	return errors.Wrap(ctx.Err(), "error waiting for informer sync")
}

func contextWithCancelByChannel(ctx context.Context, ch chan struct{}, timeout time.Duration) (context.Context, func()) {
	ctx, cancel := context.WithTimeout(ctx, timeout)
	go func() {
		select {
		case <-ch:
			cancel()
		case <-ctx.Done():
			return
		}
	}()
	return ctx, cancel
}

func (w *Watcher) ListPods(instance string) ([]rpaas.PodStatus, int32, error) {
	labelSet := labels.Set{
		defaultInstanceLabel: instance,
	}
	sel := labels.SelectorFromSet(labelSet)

	pods := []rpaas.PodStatus{}

	list, err := w.Informer.Lister().List(sel)
	if err != nil || len(list) == 0 {
		return pods, -1, rpaas.NotFoundError{Msg: fmt.Sprintf("Failed to find pods for %s: %v", instance, err)}
	}

	port := util.PortByName(list[0].Spec.Containers[0].Ports, nginx.PortNameManagement)
	for _, p := range list {
		ps, err := w.podStatus(p)
		if err != nil {
			continue
		}
		pods = append(pods, ps)
	}
	return pods, port, nil
}

func (w *Watcher) podStatus(pod *v1.Pod) (rpaas.PodStatus, error) {
	allRunning := true
	for _, cs := range pod.Status.ContainerStatuses {
		allRunning = allRunning && cs.Ready
	}
	return rpaas.PodStatus{
		Address: pod.Status.PodIP,
		Running: allRunning,
	}, nil
}
