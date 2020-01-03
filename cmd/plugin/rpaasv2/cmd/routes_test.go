// Copyright 2019 tsuru authors. All rights reserved.
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
	rpaasclient "github.com/tsuru/rpaas-operator/pkg/rpaas/client"
	"github.com/tsuru/rpaas-operator/pkg/rpaas/client/fake"
)

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
				FakeListRoutes: func(args rpaasclient.ListRoutesArgs) ([]rpaasclient.Route, *http.Response, error) {
					expected := rpaasclient.ListRoutesArgs{Instance: "my-instance"}
					assert.Equal(t, expected, args)
					return nil, nil, fmt.Errorf("some error")
				},
			},
		},
		{
			name: "when listing routes on table format",
			args: []string{"./rpaasv2", "routes", "list", "-i", "my-instance"},
			expected: `+--------------+-------------------------------+--------------+-------------------+
|     PATH     |          DESTINATION          | FORCE HTTPS? |   CONFIGURATION   |
+--------------+-------------------------------+--------------+-------------------+
| /static      | static.apps.tsuru.example.com |              |                   |
+--------------+-------------------------------+--------------+-------------------+
| /login       | login.apps.tsuru.example.com  |      ✓       |                   |
+--------------+-------------------------------+--------------+-------------------+
| /custom/path |                               |              | # My NGINX config |
+--------------+-------------------------------+--------------+-------------------+
`,
			client: &fake.FakeClient{
				FakeListRoutes: func(args rpaasclient.ListRoutesArgs) ([]rpaasclient.Route, *http.Response, error) {
					expected := rpaasclient.ListRoutesArgs{Instance: "my-instance"}
					assert.Equal(t, expected, args)
					return []rpaasclient.Route{
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
					}, nil, nil
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
				FakeListRoutes: func(args rpaasclient.ListRoutesArgs) ([]rpaasclient.Route, *http.Response, error) {
					expected := rpaasclient.ListRoutesArgs{Instance: "my-instance"}
					assert.Equal(t, expected, args)
					return []rpaasclient.Route{
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
					}, nil, nil
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			stdout := &bytes.Buffer{}
			stderr := &bytes.Buffer{}
			app := newTestApp(stdout, stderr, tt.client)
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
