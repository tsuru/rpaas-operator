// Copyright 2025 tsuru authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package cmd

import (
	"bytes"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/tsuru/rpaas-operator/api/v1alpha1"
	rpaasclient "github.com/tsuru/rpaas-operator/pkg/rpaas/client"
	"github.com/tsuru/rpaas-operator/pkg/rpaas/client/fake"
	clientTypes "github.com/tsuru/rpaas-operator/pkg/rpaas/client/types"
)

func TestListUpstreamOptions(t *testing.T) {
	tests := []struct {
		name          string
		args          []string
		expected      string
		expectedError string
		client        rpaasclient.Client
	}{
		{
			name:          "when ListUpstreamOptions returns an error",
			args:          []string{"./rpaasv2", "upstream", "list", "-i", "my-instance"},
			expectedError: "some error",
			client: &fake.FakeClient{
				FakeListUpstreamOptions: func(args rpaasclient.ListUpstreamOptionsArgs) ([]clientTypes.UpstreamOptions, error) {
					expected := rpaasclient.ListUpstreamOptionsArgs{Instance: "my-instance"}
					assert.Equal(t, expected, args)
					return nil, fmt.Errorf("some error")
				},
			},
		},
		{
			name: "when ListUpstreamOptions returns no error with empty result",
			args: []string{"./rpaasv2", "upstream", "list", "-i", "my-instance"},
			expected: `+-------------+------------+--------------+------------------+
| Primary App | Canary App | Load Balance | Traffic Policies |
+-------------+------------+--------------+------------------+
+-------------+------------+--------------+------------------+
`,
			client: &fake.FakeClient{
				FakeListUpstreamOptions: func(args rpaasclient.ListUpstreamOptionsArgs) ([]clientTypes.UpstreamOptions, error) {
					expected := rpaasclient.ListUpstreamOptionsArgs{Instance: "my-instance"}
					assert.Equal(t, expected, args)
					return []clientTypes.UpstreamOptions{}, nil
				},
			},
		},
		{
			name: "when ListUpstreamOptions returns upstream options",
			args: []string{"./rpaasv2", "upstream", "list", "-i", "my-instance"},
			expected: `+-------------+------------+--------------+------------------+
| Primary App | Canary App | Load Balance | Traffic Policies |
+-------------+------------+--------------+------------------+
| app1        | app2       | round_robin  | Weight: 80/100   |
+-------------+------------+--------------+------------------+
`,
			client: &fake.FakeClient{
				FakeListUpstreamOptions: func(args rpaasclient.ListUpstreamOptionsArgs) ([]clientTypes.UpstreamOptions, error) {
					expected := rpaasclient.ListUpstreamOptionsArgs{Instance: "my-instance"}
					assert.Equal(t, expected, args)
					return []clientTypes.UpstreamOptions{
						{
							PrimaryBind: "app1",
							CanaryBinds: []string{"app2"},
							LoadBalance: v1alpha1.LoadBalanceRoundRobin,
							TrafficShapingPolicy: v1alpha1.TrafficShapingPolicy{
								Weight:      80,
								WeightTotal: 100,
							},
						},
					}, nil
				},
			},
		},
		{
			name: "when ListUpstreamOptions returns upstream options with multiple traffic policies",
			args: []string{"./rpaasv2", "upstream", "list", "-i", "my-instance"},
			expected: `+-------------+------------+--------------+----------------------------+
| Primary App | Canary App | Load Balance | Traffic Policies           |
+-------------+------------+--------------+----------------------------+
| app1        | app2       | round_robin  | Header: X-test=v1 (exact); |
|             |            |              | Weight: 50/100             |
+-------------+------------+--------------+----------------------------+
`,
			client: &fake.FakeClient{
				FakeListUpstreamOptions: func(args rpaasclient.ListUpstreamOptionsArgs) ([]clientTypes.UpstreamOptions, error) {
					expected := rpaasclient.ListUpstreamOptionsArgs{Instance: "my-instance"}
					assert.Equal(t, expected, args)
					return []clientTypes.UpstreamOptions{
						{
							PrimaryBind: "app1",
							CanaryBinds: []string{"app2"},
							LoadBalance: v1alpha1.LoadBalanceRoundRobin,
							TrafficShapingPolicy: v1alpha1.TrafficShapingPolicy{
								Weight:        50,
								WeightTotal:   100,
								Header:        "X-test",
								HeaderValue:   "v1",
								HeaderPattern: "", // Empty pattern means exact match
							},
						},
					}, nil
				},
			},
		},
		{
			name: "when ListUpstreamOptions returns JSON format",
			args: []string{"./rpaasv2", "upstream", "list", "-i", "my-instance", "--raw-output"},
			expected: `[
	{
		"app": "app1",
		"canary": [
			"app2"
		],
		"trafficShapingPolicy": {
			"weight": 80,
			"weightTotal": 100
		},
		"loadBalance": "round_robin"
	}
]
`,
			client: &fake.FakeClient{
				FakeListUpstreamOptions: func(args rpaasclient.ListUpstreamOptionsArgs) ([]clientTypes.UpstreamOptions, error) {
					expected := rpaasclient.ListUpstreamOptionsArgs{Instance: "my-instance"}
					assert.Equal(t, expected, args)
					return []clientTypes.UpstreamOptions{
						{
							PrimaryBind: "app1",
							CanaryBinds: []string{"app2"},
							LoadBalance: v1alpha1.LoadBalanceRoundRobin,
							TrafficShapingPolicy: v1alpha1.TrafficShapingPolicy{
								Weight:      80,
								WeightTotal: 100,
							},
						},
					}, nil
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			stdout := &bytes.Buffer{}
			stderr := &bytes.Buffer{}
			app := NewApp(stdout, stderr, tt.client)
			err := app.Run(tt.args)
			if tt.expectedError != "" {
				assert.Error(t, err)
				assert.EqualError(t, err, tt.expectedError)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.expected, stdout.String())
			assert.Empty(t, stderr.String())
		})
	}
}

func TestAddUpstreamOptions(t *testing.T) {
	tests := []struct {
		name          string
		args          []string
		expected      string
		expectedError string
		client        rpaasclient.Client
	}{
		{
			name:          "when AddUpstreamOptions returns an error",
			args:          []string{"./rpaasv2", "upstream", "add", "-i", "my-instance", "-a", "app1"},
			expectedError: "some error",
			client: &fake.FakeClient{
				FakeAddUpstreamOptions: func(args rpaasclient.AddUpstreamOptionsArgs) error {
					expected := rpaasclient.AddUpstreamOptionsArgs{
						Instance:    "my-instance",
						PrimaryBind: "app1",
					}
					assert.Equal(t, expected, args)
					return fmt.Errorf("some error")
				},
			},
		},
		{
			name:     "when AddUpstreamOptions returns no error",
			args:     []string{"./rpaasv2", "upstream", "add", "-i", "my-instance", "-a", "app1"},
			expected: "Upstream options added for app \"app1\".\n",
			client: &fake.FakeClient{
				FakeAddUpstreamOptions: func(args rpaasclient.AddUpstreamOptionsArgs) error {
					expected := rpaasclient.AddUpstreamOptionsArgs{
						Instance:    "my-instance",
						PrimaryBind: "app1",
					}
					assert.Equal(t, expected, args)
					return nil
				},
			},
		},
		{
			name: "when AddUpstreamOptions with all options",
			args: []string{"./rpaasv2", "upstream", "add", "-i", "my-instance", "-a", "app1",
				"--canary", "app2", "--canary", "app3", "--load-balance", "round_robin",
				"--weight", "80", "--weight-total", "100", "--header", "X-Version",
				"--header-value", "v2", "--cookie", "session"},
			expected: "Upstream options added for app \"app1\".\n",
			client: &fake.FakeClient{
				FakeAddUpstreamOptions: func(args rpaasclient.AddUpstreamOptionsArgs) error {
					expected := rpaasclient.AddUpstreamOptionsArgs{
						Instance:    "my-instance",
						PrimaryBind: "app1",
						CanaryBinds: []string{"app2", "app3"},
						LoadBalance: "round_robin",
						TrafficShapingPolicy: rpaasclient.TrafficShapingPolicy{
							Weight:        80,
							WeightTotal:   100,
							Header:        "X-Version",
							HeaderValue:   "v2",
							HeaderPattern: "", // Removed as it's mutually exclusive with HeaderValue
							Cookie:        "session",
						},
					}
					assert.Equal(t, expected, args)
					return nil
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			stdout := &bytes.Buffer{}
			stderr := &bytes.Buffer{}
			app := NewApp(stdout, stderr, tt.client)
			err := app.Run(tt.args)
			if tt.expectedError != "" {
				assert.Error(t, err)
				assert.EqualError(t, err, tt.expectedError)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.expected, stdout.String())
			assert.Empty(t, stderr.String())
		})
	}
}

func TestUpdateUpstreamOptions(t *testing.T) {
	tests := []struct {
		name          string
		args          []string
		expected      string
		expectedError string
		client        rpaasclient.Client
	}{
		{
			name:          "when UpdateUpstreamOptions returns an error",
			args:          []string{"./rpaasv2", "upstream", "update", "-i", "my-instance", "-a", "app1"},
			expectedError: "some error",
			client: &fake.FakeClient{
				FakeUpdateUpstreamOptions: func(args rpaasclient.UpdateUpstreamOptionsArgs) error {
					expected := rpaasclient.UpdateUpstreamOptionsArgs{
						Instance:    "my-instance",
						PrimaryBind: "app1",
					}
					assert.Equal(t, expected, args)
					return fmt.Errorf("some error")
				},
			},
		},
		{
			name:     "when UpdateUpstreamOptions returns no error",
			args:     []string{"./rpaasv2", "upstream", "update", "-i", "my-instance", "-a", "app1"},
			expected: "Upstream options updated for app \"app1\".\n",
			client: &fake.FakeClient{
				FakeUpdateUpstreamOptions: func(args rpaasclient.UpdateUpstreamOptionsArgs) error {
					expected := rpaasclient.UpdateUpstreamOptionsArgs{
						Instance:    "my-instance",
						PrimaryBind: "app1",
					}
					assert.Equal(t, expected, args)
					return nil
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			stdout := &bytes.Buffer{}
			stderr := &bytes.Buffer{}
			app := NewApp(stdout, stderr, tt.client)
			err := app.Run(tt.args)
			if tt.expectedError != "" {
				assert.Error(t, err)
				assert.EqualError(t, err, tt.expectedError)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.expected, stdout.String())
			assert.Empty(t, stderr.String())
		})
	}
}

func TestDeleteUpstreamOptions(t *testing.T) {
	tests := []struct {
		name          string
		args          []string
		expected      string
		expectedError string
		client        rpaasclient.Client
	}{
		{
			name:          "when DeleteUpstreamOptions returns an error",
			args:          []string{"./rpaasv2", "upstream", "delete", "-i", "my-instance", "-a", "app1"},
			expectedError: "some error",
			client: &fake.FakeClient{
				FakeDeleteUpstreamOptions: func(args rpaasclient.DeleteUpstreamOptionsArgs) error {
					expected := rpaasclient.DeleteUpstreamOptionsArgs{
						Instance:    "my-instance",
						PrimaryBind: "app1",
					}
					assert.Equal(t, expected, args)
					return fmt.Errorf("some error")
				},
			},
		},
		{
			name:     "when DeleteUpstreamOptions returns no error",
			args:     []string{"./rpaasv2", "upstream", "delete", "-i", "my-instance", "-a", "app1"},
			expected: "Upstream options removed for app \"app1\".\n",
			client: &fake.FakeClient{
				FakeDeleteUpstreamOptions: func(args rpaasclient.DeleteUpstreamOptionsArgs) error {
					expected := rpaasclient.DeleteUpstreamOptionsArgs{
						Instance:    "my-instance",
						PrimaryBind: "app1",
					}
					assert.Equal(t, expected, args)
					return nil
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			stdout := &bytes.Buffer{}
			stderr := &bytes.Buffer{}
			app := NewApp(stdout, stderr, tt.client)
			err := app.Run(tt.args)
			if tt.expectedError != "" {
				assert.Error(t, err)
				assert.EqualError(t, err, tt.expectedError)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.expected, stdout.String())
			assert.Empty(t, stderr.String())
		})
	}
}
