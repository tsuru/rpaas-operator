package purge

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/tsuru/rpaas-operator/internal/pkg/rpaas"
	extensionsruntime "github.com/tsuru/rpaas-operator/pkg/runtime"

	apiv1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func TestCanListPods(t *testing.T) {
	tests := []struct {
		name         string
		instanceName string
		resources    func() []runtime.Object
		assertion    func(t *testing.T, err error, port int32, pods []rpaas.PodStatus)
	}{
		{
			name:         "No pods for instance-not-available",
			instanceName: "instance-not-available",
			resources:    func() []runtime.Object { return []runtime.Object{} },
			assertion: func(t *testing.T, err error, port int32, pods []rpaas.PodStatus) {
				assert.Error(t, err)
				assert.Equal(t, int32(-1), port)
				assert.Equal(t, []rpaas.PodStatus{}, pods)
			},
		},
		{
			name:         "One pod for sample-rpaasv2",
			instanceName: "sample-rpaasv2",
			resources: func() []runtime.Object {
				pod := &apiv1.Pod{
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
				return []runtime.Object{pod}
			},
			assertion: func(t *testing.T, err error, port int32, pods []rpaas.PodStatus) {
				assert.NoError(t, err)
				assert.Equal(t, int32(8889), port)
				assert.Equal(t, []rpaas.PodStatus{{Running: true, Status: "", Address: "172.0.2.1"}}, pods)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			watcher, err := NewWatcher(fake.NewClientBuilder().WithScheme(extensionsruntime.NewScheme()).WithRuntimeObjects(tt.resources()...).Build())
			assert.NoError(t, err)

			pods, port, err := watcher.ListPods(tt.instanceName)
			tt.assertion(t, err, port, pods)
		})
	}
}
