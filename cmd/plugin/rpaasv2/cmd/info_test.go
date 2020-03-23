// Copyright 2020 tsuru authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package cmd

import (
	"bytes"
	"fmt"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tsuru/rpaas-operator/pkg/apis/extensions/v1alpha1"
	"github.com/tsuru/rpaas-operator/pkg/rpaas/client"
	"github.com/tsuru/rpaas-operator/pkg/rpaas/client/fake"
	"github.com/tsuru/rpaas-operator/pkg/rpaas/client/types"
	clientTypes "github.com/tsuru/rpaas-operator/pkg/rpaas/client/types"
)

func int32Ptr(n int32) *int32 {
	return &n
}

func TestInfo(t *testing.T) {
	tests := []struct {
		name          string
		args          []string
		expected      string
		expectedError string
		client        client.Client
	}{
		{
			name:          "when info route does not find the instance",
			args:          []string{"./rpaasv2", "info", "-s", "my-service", "-i", "my-instance"},
			expectedError: "not found error",
			client: &fake.FakeClient{
				FakeInfo: func(args client.InfoArgs) (*clientTypes.InstanceInfo, *http.Response, error) {
					require.Equal(t, args.Instance, "my-instance")
					return nil, nil, fmt.Errorf("not found error")
				},
			},
		},
		{
			name: "when info route is successful",
			args: []string{"./rpaasv2", "info", "-s", "my-service", "-i", "my-instance"},
			client: &fake.FakeClient{
				FakeInfo: func(args client.InfoArgs) (*clientTypes.InstanceInfo, *http.Response, error) {
					require.Equal(t, args.Instance, "my-instance")
					return &clientTypes.InstanceInfo{
						Name: "my-instance",
						Addresses: []clientTypes.InstanceAddress{
							{
								Hostname: "some-host",
								IP:       "0.0.0.0",
							},
							{
								Hostname: "some-host2",
								IP:       "0.0.0.1",
							},
						},
						Plan: "basic",
						Binds: []v1alpha1.Bind{
							{
								Name: "some-name",
								Host: "some-host",
							},
							{
								Name: "some-name2",
								Host: "some-host2",
							},
						},
						Replicas: int32Ptr(5),
						Routes: []types.Route{
							{
								Path:        "some-path",
								Destination: "some-destination",
							},
						},
						Team:        "some-team",
						Description: "some description",
						Tags:        []string{"tag1", "tag2", "tag3"},
						Autoscale: &clientTypes.Autoscale{
							MaxReplicas: int32Ptr(5),
							MinReplicas: int32Ptr(2),
							CPU:         int32Ptr(55),
							Memory:      int32Ptr(77),
						},
					}, nil, nil
				},
			},
			expected: `
Name: my-instance
Team: some-team
Description: some description
Replicas: 5
Plan: basic
Tags: tag1, tag2, tag3

Binds:
+------------+------------+
|    APP     |  ADDRESS   |
+------------+------------+
| some-name  | some-host  |
+------------+------------+
| some-name2 | some-host2 |
+------------+------------+

Addresses:
+------------+---------+
|  HOSTNAME  |   IP    |
+------------+---------+
| some-host  | 0.0.0.0 |
+------------+---------+
| some-host2 | 0.0.0.1 |
+------------+---------+

Routes:
+-----------+------------------+
|   PATH    |   DESTINATION    |
+-----------+------------------+
| some-path | some-destination |
+-----------+------------------+

Autoscale:
+----------+--------------------+
| REPLICAS | TARGET UTILIZATION |
+----------+--------------------+
| Max: 5   | CPU: 55%           |
| Min: 2   | Memory: 77%        |
+----------+--------------------+

`,
		},
		{
			name: "when info route is successful and on json format",
			args: []string{"./rpaasv2", "info", "-s", "my-service", "-i", "my-instance", "--raw-output"},
			client: &fake.FakeClient{
				FakeInfo: func(args client.InfoArgs) (*clientTypes.InstanceInfo, *http.Response, error) {
					require.Equal(t, args.Instance, "my-instance")

					return &clientTypes.InstanceInfo{
						Name: "my-instance",
						Addresses: []clientTypes.InstanceAddress{
							{
								Hostname: "some-host",
								IP:       "0.0.0.0",
							},
							{
								Hostname: "some-host2",
								IP:       "0.0.0.1",
							},
						},
						Plan: "basic",
						Binds: []v1alpha1.Bind{
							{
								Name: "some-name",
								Host: "some-host",
							},
							{
								Name: "some-name2",
								Host: "some-host2",
							},
						},
						Replicas: int32Ptr(5),
						Routes: []types.Route{
							{
								Path:        "some-path",
								Destination: "some-destination",
							},
						},
						Team:        "some team",
						Description: "some description",
						Tags:        []string{"tag1", "tag2", "tag3"},
					}, nil, nil
				},
			},
			expected: `{"address":[{"hostname":"some-host","ip":"0.0.0.0"},{"hostname":"some-host2","ip":"0.0.0.1"}],"replicas":5,"plan":"basic","routes":[{"path":"some-path","destination":"some-destination"}],"binds":[{"name":"some-name","host":"some-host"},{"name":"some-name2","host":"some-host2"}],"team":"some team","name":"my-instance","description":"some description","tags":["tag1","tag2","tag3"]}
`,
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
