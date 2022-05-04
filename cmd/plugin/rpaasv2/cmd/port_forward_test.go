// Copyright 2020 tsuru authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package cmd

import (
	"bytes"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tsuru/rpaas-operator/pkg/rpaas/client"
	"github.com/tsuru/rpaas-operator/pkg/rpaas/client/fake"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// type fakePortForwarder struct {
// 	method string
// 	url    *url.URL
// 	pfErr  error
// }

type PortForwardArgs struct {
	Pod     string
	Address string
	Port    string
}

func TestPortForward(t *testing.T) {
	version := "v1"
	tests := []struct {
		name                       string
		args                       []string
		podPath, pfPath, container string
		pod                        *corev1.Pod
		pfErr                      bool
		expected                   string
		expectedError              string
		expectedCalled             bool
		client                     client.Client
	}{
		{
			name: "when port forward is successful",
			args: []string{"./rpaasv2", "port-forward", "-s", "some-service", "-p", "my-pod", "localhost", "127.0.0.1", "-l", "8080"},
			//expected: "sucessul",
			podPath: "/api/" + version + "/pods/my-pods",
			pfPath:  "/api/" + version + "/pods/my-pods/portforward",
			pod:     execPod(),
			client:  &fake.FakeClient{},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fmt.Printf("tt.pod.TypeMeta: %v\n", tt.pod.TypeMeta)
			stdout := &bytes.Buffer{}
			stderr := &bytes.Buffer{}
			app := NewApp(stdout, stderr, tt.client)
			err := app.Run(tt.args)
			if tt.expectedError != "" {
				assert.EqualError(t, err, tt.expectedError)
				return
			}
			require.NoError(t, err)
		})
	}
}

func execPod() *corev1.Pod {
	return &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{ResourceVersion: "1"},
		Spec: corev1.PodSpec{
			RestartPolicy: corev1.RestartPolicyAlways,
			DNSPolicy:     corev1.DNSClusterFirst,
			Containers: []corev1.Container{
				{},
			},
		},
		Status: corev1.PodStatus{
			Phase: corev1.PodRunning,
		},
	}
}
