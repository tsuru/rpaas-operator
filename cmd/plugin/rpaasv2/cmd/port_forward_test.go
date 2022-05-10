// Copyright 2020 tsuru authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package cmd

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tsuru/rpaas-operator/pkg/rpaas/client"
	"github.com/tsuru/rpaas-operator/pkg/rpaas/client/fake"

	corev1 "k8s.io/api/core/v1"
)

func TestPortForward(t *testing.T) {
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
			args:     []string{"./rpaasv2", "port-forward", "-s", "some-service", "-p", "my-pod", "-l", "127.0.0.1", "-dp", "8080", "-rp", "80"},
			expected: "sucessul",
			client:   &fake.FakeClient{},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
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
