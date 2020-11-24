// Copyright 2019 tsuru authors. All rights reserved.
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
	clientTypes "github.com/tsuru/rpaas-operator/pkg/rpaas/client/types"
)

func TestGetAutoscale(t *testing.T) {
	tests := []struct {
		name          string
		args          []string
		expected      string
		expectedError string
		client        client.Client
	}{
		{
			name:          "when GetAutoscale does not find the instance",
			args:          []string{"./rpaasv2", "autoscale", "info", "-s", "my-service", "-i", "my-instance"},
			expectedError: "not found error",
			client: &fake.FakeClient{
				FakeGetAutoscale: func(args client.GetAutoscaleArgs) (*clientTypes.Autoscale, error) {
					require.Equal(t, args.Instance, "my-instance")
					return nil, fmt.Errorf("not found error")
				},
			},
		},
		{
			name: "when get autoscale route is successful",
			args: []string{"./rpaasv2", "autoscale", "info", "-s", "my-service", "-i", "my-instance"},
			client: &fake.FakeClient{
				FakeGetAutoscale: func(args client.GetAutoscaleArgs) (*clientTypes.Autoscale, error) {
					require.Equal(t, args.Instance, "my-instance")
					return &clientTypes.Autoscale{
						MaxReplicas: int32Ptr(5),
						MinReplicas: int32Ptr(2),
						CPU:         int32Ptr(50),
						Memory:      int32Ptr(55),
					}, nil
				},
			},
			expected: `+----------+--------------------+
| Replicas | Target Utilization |
+----------+--------------------+
| Max: 5   | CPU: 50%           |
| Min: 2   | Memory: 55%        |
+----------+--------------------+
`,
		},
		{
			name: "when get autoscale route is successful on JSON format",
			args: []string{"./rpaasv2", "autoscale", "info", "-s", "my-service", "-i", "my-instance", "--raw"},
			client: &fake.FakeClient{
				FakeGetAutoscale: func(args client.GetAutoscaleArgs) (*clientTypes.Autoscale, error) {
					require.Equal(t, args.Instance, "my-instance")
					return &clientTypes.Autoscale{
						MaxReplicas: int32Ptr(5),
						MinReplicas: int32Ptr(2),
						CPU:         int32Ptr(50),
						Memory:      int32Ptr(55),
					}, nil
				},
			},
			expected: "{\n\t\"minReplicas\": 2,\n\t\"maxReplicas\": 5,\n\t\"cpu\": 50,\n\t\"memory\": 55\n}\n",
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
			assert.Equal(t, tt.expected, stdout.String())
			assert.Empty(t, stderr.String())
		})
	}
}

func TestRemoveAutoscale(t *testing.T) {
	tests := []struct {
		name          string
		args          []string
		expected      string
		expectedError string
		client        client.Client
	}{
		{
			name:          "when Remove Autoscale does not find the instance",
			args:          []string{"./rpaasv2", "autoscale", "remove", "-s", "my-service", "-i", "my-instance"},
			expectedError: "not found error",
			client: &fake.FakeClient{
				FakeRemoveAutoscale: func(args client.RemoveAutoscaleArgs) error {
					require.Equal(t, args.Instance, "my-instance")
					return fmt.Errorf("not found error")
				},
			},
		},
		{
			name: "when remove autoscale route is successful",
			args: []string{"./rpaasv2", "autoscale", "remove", "-s", "my-service", "-i", "my-instance"},
			client: &fake.FakeClient{
				FakeRemoveAutoscale: func(args client.RemoveAutoscaleArgs) error {
					require.Equal(t, args.Instance, "my-instance")
					return nil
				},
			},
			expected: "Autoscale of my-service/my-instance successfully removed\n",
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
			assert.Equal(t, tt.expected, stdout.String())
			assert.Empty(t, stderr.String())
		})
	}
}
