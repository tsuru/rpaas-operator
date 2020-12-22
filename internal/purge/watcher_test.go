package purge

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/tsuru/rpaas-operator/internal/pkg/rpaas"

	apiv1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/watch"
	fakek8s "k8s.io/client-go/kubernetes/fake"
	ktesting "k8s.io/client-go/testing"
)

var (
	k8sClient = fakek8s.NewSimpleClientset()
)

func TestCanListPods(t *testing.T) {

	watchFake := watch.NewFake()
	k8sClient.Fake.PrependWatchReactor("pods", ktesting.DefaultWatchReactor(watchFake, nil))

	watcher, err := NewWatcher(k8sClient)
	assert.NoError(t, err)

	basePod := &apiv1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name: "pod0-sample-rpaasv2",
			Labels: map[string]string{
				defaultInstanceLabel: "sample-rpaasv2",
			},
			ResourceVersion: "0",
		},
		Spec: apiv1.PodSpec{
			Containers: []apiv1.Container{
				{
					Ports: []apiv1.ContainerPort{
						{Name: "nginx-metrics", ContainerPort: 8889},
					},
				},
			},
		},
		Status: apiv1.PodStatus{
			PodIP:             "172.0.2.1",
			ContainerStatuses: []apiv1.ContainerStatus{{Ready: true}},
		},
	}
	watchFake.Add(basePod.DeepCopy())

	pods, port, err := watcher.ListPods("sample-rpaasv2")
	assert.NoError(t, err)

	assert.Equal(t, 8889, port)
	assert.Equal(t, []rpaas.PodStatus{}, pods)

}
