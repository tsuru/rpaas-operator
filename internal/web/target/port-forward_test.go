package target

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	fakekubernetes "k8s.io/client-go/kubernetes/fake"

	corev1 "k8s.io/api/core/v1"
)

func TestPortForwardFindPodLabel(t *testing.T) {
	pf := PortForward{
		Clientset: fakekubernetes.NewSimpleClientset(
			execPod("my-pod", map[string]string{
				"name": "my-pod",
			}),
		),
		Labels: metav1.LabelSelector{MatchLabels: map[string]string{"name": "my-pod"}},
	}
	pod, err := pf.findPodByLabels(context.Background())
	assert.Nil(t, err)
	assert.Equal(t, "my-pod", pod)
}

func TestPortForwardNotFoundPodLabel(t *testing.T) {
	pf := PortForward{
		Clientset: fakekubernetes.NewSimpleClientset(
			execPod("my-pod", map[string]string{
				"name": "my-pod",
			}),
		),
		Labels: metav1.LabelSelector{MatchExpressions: []metav1.LabelSelectorRequirement{{
			Key:      "name",
			Operator: metav1.LabelSelectorOpIn,
			Values:   []string{"flux", "flou"},
		}}},
	}
	_, err := pf.findPodByLabels(context.Background())
	assert.NotNil(t, err)
	assert.Equal(t, "Could not find running pod for selector: labels \"name in (flou,flux)\"", err.Error())
}

func execPod(name string, labels map[string]string) *corev1.Pod {
	return &corev1.Pod{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Pod",
			APIVersion: "v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Labels: labels,
			Name:   name,
		},
	}
}
