// Copyright 2019 tsuru authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package cmd

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	rpaasclient "github.com/tsuru/rpaas-operator/pkg/rpaas/client"
	"github.com/tsuru/rpaas-operator/pkg/rpaas/client/fake"
)

func TestUpdateBlock(t *testing.T) {
	nginxConfig := `# My custom NGINX configuration`

	blockFile, err := ioutil.TempFile("", "nginx.*.cfg")
	require.NoError(t, err)
	_, err = blockFile.Write([]byte(nginxConfig))
	require.NoError(t, err)
	require.NoError(t, blockFile.Close())
	defer os.Remove(blockFile.Name())

	tests := []struct {
		name          string
		args          []string
		expected      string
		expectedError string
		client        rpaasclient.Client
	}{
		{
			name:          "when UpdateBlock returns an error",
			args:          []string{"./rpaasv2", "blocks", "update", "-i", "my-instance", "--name", "http", "--content", blockFile.Name()},
			expectedError: "some error",
			client: &fake.FakeClient{
				FakeUpdateBlock: func(args rpaasclient.UpdateBlockArgs) (*http.Response, error) {
					expected := rpaasclient.UpdateBlockArgs{
						Instance: "my-instance",
						Name:     "http",
						Content:  nginxConfig,
					}
					assert.Equal(t, expected, args)
					return nil, fmt.Errorf("some error")
				},
			},
		},
		{
			name:     "when UpdateBlock returns no error",
			args:     []string{"./rpaasv2", "blocks", "update", "-i", "my-instance", "--name", "server", "--content", blockFile.Name()},
			expected: "NGINX configuration fragment inserted at \"server\" context\n",
			client: &fake.FakeClient{
				FakeUpdateBlock: func(args rpaasclient.UpdateBlockArgs) (*http.Response, error) {
					expected := rpaasclient.UpdateBlockArgs{
						Instance: "my-instance",
						Name:     "server",
						Content:  nginxConfig,
					}
					assert.Equal(t, expected, args)
					return nil, nil
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

func TestDeleteBlock(t *testing.T) {
	tests := []struct {
		name          string
		args          []string
		expected      string
		expectedError string
		client        rpaasclient.Client
	}{
		{
			name:          "when DeleteBlock returns an error",
			args:          []string{"./rpaasv2", "blocks", "delete", "-i", "my-instance", "--name", "http"},
			expectedError: "some error",
			client: &fake.FakeClient{
				FakeDeleteBlock: func(args rpaasclient.DeleteBlockArgs) (*http.Response, error) {
					expected := rpaasclient.DeleteBlockArgs{
						Instance: "my-instance",
						Name:     "http",
					}
					assert.Equal(t, expected, args)
					return nil, fmt.Errorf("some error")
				},
			},
		},
		{
			name:     "when DeleteBlock returns no error",
			args:     []string{"./rpaasv2", "blocks", "delete", "-i", "my-instance", "--name", "http"},
			expected: "NGINX configuration at \"http\" context removed\n",
			client: &fake.FakeClient{
				FakeDeleteBlock: func(args rpaasclient.DeleteBlockArgs) (*http.Response, error) {
					expected := rpaasclient.DeleteBlockArgs{
						Instance: "my-instance",
						Name:     "http",
					}
					assert.Equal(t, expected, args)
					return nil, nil
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

func TestListBlocks(t *testing.T) {
	tests := []struct {
		name          string
		args          []string
		expected      string
		expectedError string
		client        rpaasclient.Client
	}{
		{
			name:          "when ListBlocks returns an error",
			args:          []string{"./rpaasv2", "blocks", "list", "-i", "my-instance"},
			expectedError: "some error",
			client: &fake.FakeClient{
				FakeListBlocks: func(args rpaasclient.ListBlocksArgs) ([]rpaasclient.Block, *http.Response, error) {
					expected := rpaasclient.ListBlocksArgs{
						Instance: "my-instance",
					}
					assert.Equal(t, expected, args)
					return nil, nil, fmt.Errorf("some error")
				},
			},
		},
		{
			name: "when listing blocks on table format",
			args: []string{"./rpaasv2", "blocks", "list", "-i", "my-instance"},
			expected: `+---------+-----------------------------+
| CONTEXT |        CONFIGURATION        |
+---------+-----------------------------+
| http    | # some HTTP configuration   |
| server  | # some server configuration |
+---------+-----------------------------+
`,
			client: &fake.FakeClient{
				FakeListBlocks: func(args rpaasclient.ListBlocksArgs) ([]rpaasclient.Block, *http.Response, error) {
					expected := rpaasclient.ListBlocksArgs{
						Instance: "my-instance",
					}
					assert.Equal(t, expected, args)
					return []rpaasclient.Block{
						{Name: "http", Content: "# some HTTP configuration"},
						{Name: "server", Content: "# some server configuration"},
					}, nil, nil
				},
			},
		},
		{
			name: "when listing blocks on raw format",
			args: []string{"./rpaasv2", "blocks", "list", "-i", "my-instance", "--raw-output"},
			expected: `[
	{
		"block_name": "http",
		"content": "# some HTTP configuration"
	},
	{
		"block_name": "server",
		"content": "# some server configuration"
	}
]
`,
			client: &fake.FakeClient{
				FakeListBlocks: func(args rpaasclient.ListBlocksArgs) ([]rpaasclient.Block, *http.Response, error) {
					expected := rpaasclient.ListBlocksArgs{
						Instance: "my-instance",
					}
					assert.Equal(t, expected, args)
					return []rpaasclient.Block{
						{Name: "http", Content: "# some HTTP configuration"},
						{Name: "server", Content: "# some server configuration"},
					}, nil, nil
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
