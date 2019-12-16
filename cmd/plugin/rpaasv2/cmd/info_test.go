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
	"github.com/tsuru/rpaas-operator/pkg/rpaas/client"
	"github.com/tsuru/rpaas-operator/pkg/rpaas/client/fake"
	"github.com/tsuru/rpaas-operator/pkg/rpaas/client/types"
	"github.com/urfave/cli"
)

func TestInfo(t *testing.T) {
	tests := []struct {
		name          string
		args          []string
		expected      string
		expectedError string
		client        client.Client
	}{
		{
			name:          "when GetPlans returns an error",
			args:          []string{"./rpaasv2", "info", "-s", "some-service", "-i", "fake-instance"},
			expectedError: "some error",
			client: &fake.FakeClient{
				FakeGetPlans: func(instance string) ([]types.Plan, *http.Response, error) {
					assert.Equal(t, "fake-instance", instance)
					return nil, nil, fmt.Errorf("some error")
				},
				FakeGetFlavors: func(instance string) ([]types.Flavor, *http.Response, error) {
					require.FailNow(t, "cannot call the GetFlavors method")
					return nil, nil, nil
				},
			},
		},
		{
			name:          "when GetFlavors returns an error",
			args:          []string{"./rpaasv2", "info", "-s", "some-service", "-i", "fake-instance"},
			expectedError: "other error",
			client: &fake.FakeClient{
				FakeGetPlans: func(instance string) ([]types.Plan, *http.Response, error) {
					assert.Equal(t, "fake-instance", instance)
					return []types.Plan{}, nil, nil
				},
				FakeGetFlavors: func(instance string) ([]types.Flavor, *http.Response, error) {
					assert.Equal(t, "fake-instance", instance)
					return nil, nil, fmt.Errorf("other error")
				},
			},
		},
		{
			name: "testing info route with valid arguments",
			args: []string{"./rpaasv2", "info", "-s", "some-service", "-i", "fake-instance"},
			expected: `
+-----------+------------------+---------+
|   PLANS   |   DESCRIPTION    | DEFAULT |
+-----------+------------------+---------+
| plan name | plan description | true    |
+-----------+------------------+---------+

+-------------+--------------------+
|   FLAVORS   |    DESCRIPTION     |
+-------------+--------------------+
| flavor name | flavor description |
+-------------+--------------------+
`,
			client: &fake.FakeClient{
				FakeGetPlans: func(instance string) ([]types.Plan, *http.Response, error) {
					return []types.Plan{
						{
							Name:        "plan name",
							Description: "plan description",
							Default:     true,
						},
					}, nil, nil
				},
				FakeGetFlavors: func(instance string) ([]types.Flavor, *http.Response, error) {
					return []types.Flavor{
						{
							Name:        "flavor name",
							Description: "flavor description",
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
				assert.EqualError(t, err, tt.expectedError)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.expected, stdout.String())
			assert.Empty(t, stderr.String())
		})
	}
}

func newTestApp(stdout, stderr *bytes.Buffer, rpaasClient client.Client) *cli.App {
	setRpaasClient(rpaasClient)
	app := NewApp()
	app.Writer = stdout
	app.ErrWriter = stderr
	return app
}
