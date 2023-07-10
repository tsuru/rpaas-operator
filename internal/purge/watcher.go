package purge

import (
	"context"
	"fmt"

	v1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/tsuru/rpaas-operator/internal/pkg/rpaas"
	"github.com/tsuru/rpaas-operator/internal/pkg/rpaas/nginx"
	"github.com/tsuru/rpaas-operator/pkg/util"
)

// Should be exported from rpaas/k8s.go
const (
	defaultInstanceLabel = "rpaas.extensions.tsuru.io/instance-name"
)

type Watcher struct {
	Client client.Client
}

func NewWatcher(c client.Client) (*Watcher, error) {
	return &Watcher{
		Client: c,
	}, nil
}

func (w *Watcher) ListPods(instance string) ([]rpaas.PodStatus, int32, error) {
	pods := []rpaas.PodStatus{}

	list := &v1.PodList{}
	err := w.Client.List(context.Background(), list, client.MatchingLabels{defaultInstanceLabel: instance})
	if err != nil || len(list.Items) == 0 {
		return pods, -1, rpaas.NotFoundError{Msg: fmt.Sprintf("Failed to find pods for %s: %v", instance, err)}
	}

	port := util.PortByName(list.Items[0].Spec.Containers[0].Ports, nginx.PortNameManagement)
	for _, p := range list.Items {
		if p.Status.PodIP == "" {
			continue
		}
		ps, err := w.podStatus(&p)
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
