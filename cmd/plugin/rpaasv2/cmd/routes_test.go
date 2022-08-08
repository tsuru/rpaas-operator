// Copyright 2019 tsuru authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package cmd

import (
	"bytes"
	"fmt"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	rpaasclient "github.com/tsuru/rpaas-operator/pkg/rpaas/client"
	"github.com/tsuru/rpaas-operator/pkg/rpaas/client/fake"
	clientTypes "github.com/tsuru/rpaas-operator/pkg/rpaas/client/types"
)

func TestDeleteRoute(t *testing.T) {
	tests := []struct {
		name          string
		args          []string
		expected      string
		expectedError string
		client        rpaasclient.Client
	}{
		{
			name:          "when DeleteRoute returns an error",
			args:          []string{"./rpaasv2", "routes", "delete", "-i", "my-instance", "-p", "/my/custom/path"},
			expectedError: "some error",
			client: &fake.FakeClient{
				FakeDeleteRoute: func(args rpaasclient.DeleteRouteArgs) error {
					expected := rpaasclient.DeleteRouteArgs{
						Instance: "my-instance",
						Path:     "/my/custom/path",
					}
					assert.Equal(t, expected, args)
					return fmt.Errorf("some error")
				},
			},
		},
		{
			name:     "when DeleteRoute returns no error",
			args:     []string{"./rpaasv2", "routes", "delete", "-i", "my-instance", "-p", "/my/custom/path"},
			expected: "Route \"/my/custom/path\" deleted.\n",
			client: &fake.FakeClient{
				FakeDeleteRoute: func(args rpaasclient.DeleteRouteArgs) error {
					expected := rpaasclient.DeleteRouteArgs{
						Instance: "my-instance",
						Path:     "/my/custom/path",
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

func TestListRoutes(t *testing.T) {
	tests := []struct {
		name          string
		args          []string
		expected      string
		expectedError string
		client        rpaasclient.Client
	}{
		{
			name:          "when ListRoutes returns an error",
			args:          []string{"./rpaasv2", "routes", "list", "-i", "my-instance"},
			expectedError: "some error",
			client: &fake.FakeClient{
				FakeListRoutes: func(args rpaasclient.ListRoutesArgs) ([]clientTypes.Route, error) {
					expected := rpaasclient.ListRoutesArgs{Instance: "my-instance"}
					assert.Equal(t, expected, args)
					return nil, fmt.Errorf("some error")
				},
			},
		},
		{
			name: "when listing routes on table format",
			args: []string{"./rpaasv2", "routes", "list", "-i", "my-instance"},
			expected: `+--------------+-------------------------------+--------------+-------------------+
| Path         | Destination                   | Force HTTPS? | Configuration     |
+--------------+-------------------------------+--------------+-------------------+
| /static      | static.apps.tsuru.example.com |              |                   |
| /login       | login.apps.tsuru.example.com  |      ✓       |                   |
| /custom/path |                               |              | # My NGINX config |
+--------------+-------------------------------+--------------+-------------------+
`,
			client: &fake.FakeClient{
				FakeListRoutes: func(args rpaasclient.ListRoutesArgs) ([]clientTypes.Route, error) {
					expected := rpaasclient.ListRoutesArgs{Instance: "my-instance"}
					assert.Equal(t, expected, args)
					return []clientTypes.Route{
						{
							Path:        "/static",
							Destination: "static.apps.tsuru.example.com",
						},
						{
							Path:        "/login",
							Destination: "login.apps.tsuru.example.com",
							HTTPSOnly:   true,
						},
						{
							Path:    "/custom/path",
							Content: "# My NGINX config",
						},
					}, nil
				},
			},
		},
		{
			name: "when listing blocks on raw format",
			args: []string{"./rpaasv2", "routes", "list", "-i", "my-instance", "--raw-output"},
			expected: `[
	{
		"path": "/static",
		"destination": "static.apps.tsuru.example.com"
	},
	{
		"path": "/login",
		"destination": "login.apps.tsuru.example.com",
		"https_only": true
	},
	{
		"path": "/custom/path",
		"content": "# My NGINX config"
	}
]
`,
			client: &fake.FakeClient{
				FakeListRoutes: func(args rpaasclient.ListRoutesArgs) ([]clientTypes.Route, error) {
					expected := rpaasclient.ListRoutesArgs{Instance: "my-instance"}
					assert.Equal(t, expected, args)
					return []clientTypes.Route{
						{
							Path:        "/static",
							Destination: "static.apps.tsuru.example.com",
						},
						{
							Path:        "/login",
							Destination: "login.apps.tsuru.example.com",
							HTTPSOnly:   true,
						},
						{
							Path:    "/custom/path",
							Content: "# My NGINX config",
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

func TestUpdateRoute(t *testing.T) {
	nginxConfig := `# My custom NGINX configuration`
	configFile, err := os.CreateTemp("", "nginx.*.cfg")
	require.NoError(t, err)
	_, err = configFile.Write([]byte(nginxConfig))
	require.NoError(t, err)
	require.NoError(t, configFile.Close())
	defer os.Remove(configFile.Name())

	tests := []struct {
		name          string
		args          []string
		expected      string
		expectedError string
		client        rpaasclient.Client
	}{
		{
			name:          "when UpdateRoute returns an error",
			args:          []string{"./rpaasv2", "routes", "update", "-i", "my-instance", "-p", "/app", "-d", "app.tsuru.example.com"},
			expectedError: "some error",
			client: &fake.FakeClient{
				FakeUpdateRoute: func(args rpaasclient.UpdateRouteArgs) error {
					expected := rpaasclient.UpdateRouteArgs{
						Instance:    "my-instance",
						Path:        "/app",
						Destination: "app.tsuru.example.com",
					}
					assert.Equal(t, expected, args)
					return fmt.Errorf("some error")
				},
			},
		},
		{
			name:     "when proxying to a destination",
			args:     []string{"./rpaasv2", "routes", "update", "-i", "my-instance", "-p", "/app", "-d", "app.tsuru.example.com", "--https-only"},
			expected: "Route \"/app\" updated.\n",
			client: &fake.FakeClient{
				FakeUpdateRoute: func(args rpaasclient.UpdateRouteArgs) error {
					expected := rpaasclient.UpdateRouteArgs{
						Instance:    "my-instance",
						Path:        "/app",
						Destination: "app.tsuru.example.com",
						HTTPSOnly:   true,
					}
					assert.Equal(t, expected, args)
					return nil
				},
			},
		},
		{
			name:     "when using a custom NGINX config",
			args:     []string{"./rpaasv2", "routes", "update", "-i", "my-instance", "-p", "/custom/path", "-c", configFile.Name()},
			expected: "Route \"/custom/path\" updated.\n",
			client: &fake.FakeClient{
				FakeUpdateRoute: func(args rpaasclient.UpdateRouteArgs) error {
					expected := rpaasclient.UpdateRouteArgs{
						Instance: "my-instance",
						Path:     "/custom/path",
						Content:  nginxConfig,
					}
					assert.Equal(t, expected, args)
					return nil
				},
			},
		},
		{
			name:     "when using a custom NGINX config with @ prefixed file path",
			args:     []string{"./rpaasv2", "routes", "update", "-i", "my-instance", "-p", "/custom/path", "-c", "@" + configFile.Name()},
			expected: "Route \"/custom/path\" updated.\n",
			client: &fake.FakeClient{
				FakeUpdateRoute: func(args rpaasclient.UpdateRouteArgs) error {
					expected := rpaasclient.UpdateRouteArgs{
						Instance: "my-instance",
						Path:     "/custom/path",
						Content:  nginxConfig,
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
