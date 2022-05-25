// Copyright 2020 tsuru authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package cmd

import (
	"bytes"
	"context"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tsuru/rpaas-operator/pkg/rpaas/client"
	"github.com/tsuru/rpaas-operator/pkg/rpaas/client/fake"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	sigsk8sconfig "sigs.k8s.io/controller-runtime/pkg/client/config"
)

func TestPortForward(t *testing.T) {
	var called bool
	tests := []struct {
		name           string
		args           []string
		container      string
		pod            *corev1.Pod
		pfErr          bool
		expected       string
		expectedError  string
		expectedCalled bool
		client         client.Client
	}{
		{
			name:     "when port forward is successful",
			args:     []string{"./rpaasv2", "port-forward", "-s", "rpaasv2", "-i", "default", "-p", "my-pod", "-l", "127.0.0.1", "-dp", "8080", "-rp", "0", "--tty", "--interactive"},
			expected: "sucessul",
			client: &fake.FakeClient{
				FakeStartPortForward: func(ctx context.Context, args client.PortForwardArgs) (*client.PortForward, error) {
					called = true
					expected := client.PortForwardArgs{
						In:              os.Stdin,
						Address:         "127.0.0.1",
						Pod:             "my-pod",
						Instance:        "default",
						DestinationPort: 8080,
						ListenPort:      0,
						TTY:             true,
						Interactive:     true,
					}
					assert.Equal(t, expected, args)
					pf := &client.PortForward{
						DestinationPort: 8080,
						ListenPort:      0,
						Namespace:       "default",
						Name:            "my-pod",
						Labels:          metav1.LabelSelector{MatchLabels: map[string]string{"name": "flux"}},
					}
					Config, err := sigsk8sconfig.GetConfig()
					if err != nil {
						return nil, err
					}
					pf.Config = Config
					pf.Clientset, err = kubernetes.NewForConfig(pf.Config)
					if err != nil {
						return nil, err
					}
					return pf, nil
				},
			},
			expectedCalled: true,
			expectedError:  "another error",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			called = false
			stdout := &bytes.Buffer{}
			stderr := &bytes.Buffer{}
			app := NewApp(stdout, stderr, tt.client)
			err := app.Run(tt.args)
			if tt.expectedError != "" {
				assert.EqualError(t, err, tt.expectedError)
				return
			}
			assert.Equal(t, tt.expectedCalled, called)
			require.NoError(t, err)
		})
	}
}

// func newPod() *corev1.Pod {
// 	return &corev1.Pod{
// 		TypeMeta: metav1.TypeMeta{
// 			Kind:       "Pod",
// 			APIVersion: "v1",
// 		},
// 		ObjectMeta: metav1.ObjectMeta{
// 			Name:      "my-pod",
// 			Namespace: "default",
// 		},
// 		Status: corev1.PodStatus{
// 			Phase: corev1.PodRunning,
// 		},
// 		Spec: corev1.PodSpec{
// 			Containers: []corev1.Container{
// 				{
// 					Name:            "nginx",
// 					Image:           "nginx",
// 					ImagePullPolicy: "Always",
// 				},
// 			},
// 		},
// 	}
// }
