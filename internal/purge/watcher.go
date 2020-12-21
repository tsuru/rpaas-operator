package purge

import (
	"time"

	"github.com/tsuru/rpaas-operator/internal/pkg/rpaas"
	"github.com/tsuru/rpaas-operator/internal/pkg/rpaas/nginx"
	"github.com/tsuru/rpaas-operator/pkg/util"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/client-go/informers"
	v1informers "k8s.io/client-go/informers/core/v1"
	"k8s.io/client-go/kubernetes"
)

// Should be exported from rpaas/k8s.go
const (
	defaultNamespace      = "rpaasv2"
	defaultKeyLabelPrefix = "rpaas.extensions.tsuru.io"

	nginxContainerName = "nginx"
)

type Watcher struct {
	Client   kubernetes.Interface
	Informer v1informers.PodInformer

	stopCh chan struct{}
}

func NewWithClient(c kubernetes.Interface) (*Watcher, error) {
	return &Watcher{
		Client: c,
		stopCh: make(chan struct{}),
	}, nil
}

func (w *Watcher) Watch() {
	informerFactory := informers.NewFilteredSharedInformerFactory(w.Client, time.Minute, metav1.NamespaceAll, nil)
	informerFactory.Start(w.stopCh)

	w.Informer = informerFactory.Core().V1().Pods()
	w.Informer.Informer()
}

func (w *Watcher) ListPods(instance string) ([]rpaas.PodStatus, int32, error) {
	labelSet := labels.Set{
		"rpaas.extensions.tsuru.io/instance-name": instance,
	}
	sel := labels.SelectorFromSet(labelSet)

	pods := []rpaas.PodStatus{}

	list, err := w.Informer.Lister().List(sel)
	if err != nil {
		// Todo
		return pods, -1, err
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
