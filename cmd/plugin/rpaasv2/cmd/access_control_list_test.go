// Copyright 2021 tsuru authors. All rights reserved.
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
	"github.com/tsuru/rpaas-operator/pkg/rpaas/client/types"
)

func TestAddAcl(t *testing.T) {
	tests := []struct {
		name          string
		args          []string
		expected      string
		expectedError string
		client        client.Client
	}{
		{
			name:          "when acl add method returns an error",
			args:          []string{"./rpaasv2", "acl", "add", "-s", "some-service", "-i", "my-instance", "--host", "some-host", "--port", "80"},
			expectedError: "some error",
			client: &fake.FakeClient{
				FakeAddAccessControlList: func(instance, host string, port int) error {
					require.Equal(t, instance, "my-instance")
					require.Equal(t, host, "some-host")
					require.Equal(t, port, 80)
					return fmt.Errorf("some error")
				},
			},
		},
		{
			name:     "when acl add is successfull",
			args:     []string{"./rpaasv2", "acl", "add", "-s", "some-service", "-i", "my-instance", "-host", "some-host.com", "--port", "80"},
			expected: "Successfully added some-host.com:80 to some-service/my-instance ACL.\n",
			client: &fake.FakeClient{
				FakeAddAccessControlList: func(instance, host string, port int) error {
					require.Equal(t, instance, "my-instance")
					require.Equal(t, host, "some-host.com")
					require.Equal(t, port, 80)
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
				assert.EqualError(t, err, tt.expectedError)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.expected, stdout.String())
			assert.Empty(t, stderr.String())
		})
	}
}

func TestRemoveAcl(t *testing.T) {
	tests := []struct {
		name          string
		args          []string
		expected      string
		expectedError string
		client        client.Client
	}{
		{
			name:          "when acl remove method returns an error",
			args:          []string{"./rpaasv2", "acl", "remove", "-s", "some-service", "-i", "my-instance", "--host", "some-host", "--port", "80"},
			expectedError: "some error",
			client: &fake.FakeClient{
				FakeRemoveAccessControlList: func(instance, host string, port int) error {
					require.Equal(t, instance, "my-instance")
					require.Equal(t, host, "some-host")
					require.Equal(t, port, 80)
					return fmt.Errorf("some error")
				},
			},
		},
		{
			name:     "when acl remove is successfull",
			args:     []string{"./rpaasv2", "acl", "remove", "-s", "some-service", "-i", "my-instance", "-host", "some-host.com", "--port", "80"},
			expected: "Successfully removed some-host.com:80 from some-service/my-instance ACL.\n",
			client: &fake.FakeClient{
				FakeRemoveAccessControlList: func(instance, host string, port int) error {
					require.Equal(t, instance, "my-instance")
					require.Equal(t, host, "some-host.com")
					require.Equal(t, port, 80)
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
				assert.EqualError(t, err, tt.expectedError)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.expected, stdout.String())
			assert.Empty(t, stderr.String())
		})
	}
}

func TestListAcl(t *testing.T) {
	tests := []struct {
		name          string
		args          []string
		expected      string
		expectedError string
		client        client.Client
	}{
		{
			name: "when acl list method returns empty",
			args: []string{"./rpaasv2", "acl", "get", "-s", "some-service", "-i", "my-instance"},
			client: &fake.FakeClient{
				FakeListAccessControlList: func(instance string) ([]types.AllowedUpstream, error) {
					require.Equal(t, instance, "my-instance")
					return []types.AllowedUpstream{}, nil
				},
			},
		},
		{
			name: "when acl list method returns a list of acls",
			args: []string{"./rpaasv2", "acl", "get", "-s", "some-service", "-i", "my-instance"},
			expected: `+-------+-------+
| Host  | Port  |
+-------+-------+
| host1 |   443 |
| host1 |    80 |
| mongo | 27017 |
+-------+-------+
`,
			client: &fake.FakeClient{
				FakeListAccessControlList: func(instance string) ([]types.AllowedUpstream, error) {
					require.Equal(t, instance, "my-instance")
					return []types.AllowedUpstream{
						{
							Host: "host1",
							Port: 443,
						},
						{
							Host: "host1",
							Port: 80,
						},
						{
							Host: "mongo",
							Port: 27017,
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
				assert.EqualError(t, err, tt.expectedError)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.expected, stdout.String())
			assert.Empty(t, stderr.String())
		})
	}
}
