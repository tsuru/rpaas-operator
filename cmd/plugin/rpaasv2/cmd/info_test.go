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
	"github.com/tsuru/rpaas-operator/internal/pkg/rpaas"
	"github.com/tsuru/rpaas-operator/pkg/apis/extensions/v1alpha1"
	"github.com/tsuru/rpaas-operator/pkg/rpaas/client"
	"github.com/tsuru/rpaas-operator/pkg/rpaas/client/fake"
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
				FakeInfo: func(args client.InfoArgs) (*rpaas.InstanceInfo, *http.Response, error) {
					require.Equal(t, args.Instance, "my-instance")
					require.Equal(t, args.Service, "my-service")
					return nil, nil, fmt.Errorf("not found error")
				},
			},
		},
		{
			name: "when info route is successful",
			args: []string{"./rpaasv2", "info", "-s", "my-service", "-i", "my-instance"},
			client: &fake.FakeClient{
				FakeInfo: func(args client.InfoArgs) (*rpaas.InstanceInfo, *http.Response, error) {
					require.Equal(t, args.Instance, "my-instance")
					require.Equal(t, args.Service, "my-service")
					return &rpaas.InstanceInfo{
						Name: "my-instance",
						Address: []rpaas.InstanceAddress{
							{
								Hostname: "some-host",
								Ip:       "0.0.0.0",
							},
							{
								Hostname: "some-host2",
								Ip:       "0.0.0.1",
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
						Locations: []v1alpha1.Location{
							{
								Path:        "some-path",
								Destination: "some-destination",
							},
						},
						Team:        "some-team",
						Description: "some description",
						Tags:        []string{"tag1", "tag2", "tag3"},
						Autoscale: &v1alpha1.RpaasInstanceAutoscaleSpec{
							MaxReplicas:                       5,
							MinReplicas:                       int32Ptr(2),
							TargetCPUUtilizationPercentage:    int32Ptr(55),
							TargetMemoryUtilizationPercentage: int32Ptr(77),
						},
					}, nil, nil
				},
			},
			expected: `
Name: my-instance
Team: some-team
Description: some description
Binds:
    #Bind 0
    App: some-name
    Host: some-host
    #Bind 1
    App: some-name2
    Host: some-host2
Tags:
    tag1
    tag2
    tag3
Adresses:
    #Address 0:
        Hostname: some-host
        Ip: 0.0.0.0
    #Address 1:
        Hostname: some-host2
        Ip: 0.0.0.1
Replicas: 5
Plan: basic
Locations:
    #Location 0
    Path: some-path
    Destination: some-destination
Autoscale:
    MaxReplicas: 5
    MinReplicas: 2
    TargetCPUUtilizationPercentage: 55
    TargetMemoryUtilizationPercentage: 77
`,
		},
		{
			name: "when info route is successful and on json format",
			args: []string{"./rpaasv2", "info", "-s", "my-service", "-i", "my-instance", "--raw-output"},
			client: &fake.FakeClient{
				FakeInfo: func(args client.InfoArgs) (*rpaas.InstanceInfo, *http.Response, error) {
					require.Equal(t, args.Instance, "my-instance")
					require.Equal(t, args.Service, "my-service")

					return &rpaas.InstanceInfo{
						Name: "my-instance",
						Address: []rpaas.InstanceAddress{
							{
								Hostname: "some-host",
								Ip:       "0.0.0.0",
							},
							{
								Hostname: "some-host2",
								Ip:       "0.0.0.1",
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
						Locations: []v1alpha1.Location{
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
			expected: `{"address":[{"hostname":"some-host","ip":"0.0.0.0"},{"hostname":"some-host2","ip":"0.0.0.1"}],"replicas":5,"plan":"basic","locations":[{"path":"some-path","destination":"some-destination"}],"binds":[{"name":"some-name","host":"some-host"},{"name":"some-name2","host":"some-host2"}],"team":"some team","name":"my-instance","description":"some description","tags":["tag1","tag2","tag3"]}
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
