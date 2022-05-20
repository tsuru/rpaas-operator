package client

import (
	"context"
	"fmt"
	"net/http"
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

func TestSometest(t *testing.T) {
	tests := []struct {
		name          string
		args          PortForwardArgs
		expectedError string
		handler       http.HandlerFunc
	}{

		{
			name: "all arguments port-forward request",
			args: PortForwardArgs{
				Instance:        "default",
				Pod:             "my-pod",
				DestinationPort: 8080,
				ListenPort:      0,
				Address:         "localhost",
			},
			handler: func(w http.ResponseWriter, r *http.Request) {
				assert.Equal(t, r.Method, "GET")
				assert.Equal(t, fmt.Sprintf("/services/%s/proxy/%s?callback=%s", FakeTsuruService, "my-instance", "%2Fresources%2Fmy-instance%2Flog&color=false&container=some-container&follow=true&lines=10&pod=some-pod"), r.URL.RequestURI())
				assert.Equal(t, "Bearer f4k3t0k3n", r.Header.Get("Authorization"))
				w.WriteHeader(http.StatusOK)
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client, server := newClientThroughTsuru(t, tt.handler)
			defer server.Close()
			err := client.StartPortForward(context.TODO(), tt.args)
			if tt.expectedError != "" {
				assert.EqualError(t, err, tt.expectedError)
				return
			}
			assert.NoError(t, err)
		})
	}
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
