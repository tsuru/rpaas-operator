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
				FakeInfo: func(args client.InfoArgs) (*rpaas.InfoBuilder, *http.Response, error) {
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
				FakeInfo: func(args client.InfoArgs) (*rpaas.InfoBuilder, *http.Response, error) {
					require.Equal(t, args.Instance, "my-instance")
					require.Equal(t, args.Service, "my-service")

					return &rpaas.InfoBuilder{
						Name: "my-instance",
						Address: &rpaas.InstanceAddress{
							Hostname: "some-host",
							Ip:       "0.0.0.0",
						},
						Plan: "basic",
						Binds: []v1alpha1.Bind{
							v1alpha1.Bind{
								Name: "some-name",
								Host: "some-host",
							},
							v1alpha1.Bind{
								Name: "some-name2",
								Host: "some-host2",
							},
						},
						Replicas: int32Ptr(5),
						Locations: []v1alpha1.Location{
							v1alpha1.Location{
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
			expected: `{"name":"my-instance"}`,
		},
		{
			name: "when info route is successful and on json format",
			args: []string{"./rpaasv2", "info", "-s", "my-service", "-i", "my-instance", "--raw-output"},
			client: &fake.FakeClient{
				FakeInfo: func(args client.InfoArgs) (*rpaas.InfoBuilder, *http.Response, error) {
					require.Equal(t, args.Instance, "my-instance")
					require.Equal(t, args.Service, "my-service")

					return &rpaas.InfoBuilder{
						Name: "my-instance",
						Address: &rpaas.InstanceAddress{
							Hostname: "some-host",
							Ip:       "0.0.0.0",
						},
						Plan: "basic",
						Binds: []v1alpha1.Bind{
							v1alpha1.Bind{
								Name: "some-name",
								Host: "some-host",
							},
							v1alpha1.Bind{
								Name: "some-name2",
								Host: "some-host2",
							},
						},
						Replicas: int32Ptr(5),
						Locations: []v1alpha1.Location{
							v1alpha1.Location{
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
			expected: `{"name":"my-instance"}`,
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
