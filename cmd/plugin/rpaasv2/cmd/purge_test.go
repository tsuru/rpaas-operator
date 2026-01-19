// Copyright 2025 tsuru authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package cmd

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	rpaasclient "github.com/tsuru/rpaas-operator/pkg/rpaas/client"
	"github.com/tsuru/rpaas-operator/pkg/rpaas/client/fake"
)

func TestPurge(t *testing.T) {
	tests := []struct {
		name          string
		args          []string
		expected      string
		expectedError string
		client        rpaasclient.Client
	}{
		{
			name:          "when PurgeCache returns an error",
			args:          []string{"./rpaasv2", "purge", "-i", "my-instance", "-p", "/path/to/purge"},
			expectedError: "some error",
			client: &fake.FakeClient{
				FakePurgeCache: func(args rpaasclient.PurgeCacheArgs) (int, error) {
					expected := rpaasclient.PurgeCacheArgs{
						Instance:     "my-instance",
						Path:         "/path/to/purge",
						PreservePath: false,
						ExtraHeaders: nil,
					}
					assert.Equal(t, expected, args)
					return 0, fmt.Errorf("some error")
				},
			},
		},
		{
			name:     "when PurgeCache returns success",
			args:     []string{"./rpaasv2", "purge", "-i", "my-instance", "-p", "/path/to/purge"},
			expected: "Object purged on 3 servers\n",
			client: &fake.FakeClient{
				FakePurgeCache: func(args rpaasclient.PurgeCacheArgs) (int, error) {
					expected := rpaasclient.PurgeCacheArgs{
						Instance:     "my-instance",
						Path:         "/path/to/purge",
						PreservePath: false,
						ExtraHeaders: nil,
					}
					assert.Equal(t, expected, args)
					return 3, nil
				},
			},
		},
		{
			name:     "when PurgeCache with preserve-path flag",
			args:     []string{"./rpaasv2", "purge", "-i", "my-instance", "-p", "/path/to/purge", "--preserve-path"},
			expected: "Object purged on 2 servers\n",
			client: &fake.FakeClient{
				FakePurgeCache: func(args rpaasclient.PurgeCacheArgs) (int, error) {
					expected := rpaasclient.PurgeCacheArgs{
						Instance:     "my-instance",
						Path:         "/path/to/purge",
						PreservePath: true,
						ExtraHeaders: nil,
					}
					assert.Equal(t, expected, args)
					return 2, nil
				},
			},
		},
		{
			name:     "when PurgeCache with extra headers",
			args:     []string{"./rpaasv2", "purge", "-i", "my-instance", "-p", "/path/to/purge", "-H", "X-Custom: value1", "-H", "X-Another: value2"},
			expected: "Object purged on 5 servers\n",
			client: &fake.FakeClient{
				FakePurgeCache: func(args rpaasclient.PurgeCacheArgs) (int, error) {
					expected := rpaasclient.PurgeCacheArgs{
						Instance:     "my-instance",
						Path:         "/path/to/purge",
						PreservePath: false,
						ExtraHeaders: map[string][]string{
							"X-Custom":  {"value1"},
							"X-Another": {"value2"},
						},
					}
					assert.Equal(t, expected, args)
					return 5, nil
				},
			},
		},
		{
			name:          "when neither path nor file is provided",
			args:          []string{"./rpaasv2", "purge", "-i", "my-instance"},
			expectedError: "either --path or --file must be provided",
			client:        &fake.FakeClient{},
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

func TestPurgeWithFile(t *testing.T) {
	tests := []struct {
		name          string
		args          []string
		fileContent   string
		expected      string
		expectedError string
		client        rpaasclient.Client
	}{
		{
			name: "when PurgeCacheBulk returns success",
			args: []string{"./rpaasv2", "purge", "-i", "my-instance", "-f"},
			fileContent: `[
  {
    "path": "/path1",
    "preserve_path": false
  },
  {
    "path": "/path2",
    "preserve_path": true
  }
]`,
			expected: "Path \"/path1\": purged on 3 servers\nPath \"/path2\": purged on 2 servers\n",
			client: &fake.FakeClient{
				FakePurgeCacheBulk: func(args rpaasclient.PurgeCacheBulkArgs) ([]rpaasclient.PurgeBulkResult, error) {
					assert.Equal(t, "my-instance", args.Instance)
					assert.Len(t, args.Items, 2)
					assert.Equal(t, "/path1", args.Items[0].Path)
					assert.False(t, args.Items[0].PreservePath)
					assert.Equal(t, "/path2", args.Items[1].Path)
					assert.True(t, args.Items[1].PreservePath)
					return []rpaasclient.PurgeBulkResult{
						{Path: "/path1", InstancesPurged: 3},
						{Path: "/path2", InstancesPurged: 2},
					}, nil
				},
			},
		},
		{
			name: "when PurgeCacheBulk returns partial errors",
			args: []string{"./rpaasv2", "purge", "-i", "my-instance", "-f"},
			fileContent: `[
  {
    "path": "/path1"
  },
  {
    "path": "/path2"
  }
]`,
			expected:      "Path \"/path1\": purged on 1 servers\nPath \"/path2\": ERROR - connection timeout\n",
			expectedError: "some purge operations failed",
			client: &fake.FakeClient{
				FakePurgeCacheBulk: func(args rpaasclient.PurgeCacheBulkArgs) ([]rpaasclient.PurgeBulkResult, error) {
					return []rpaasclient.PurgeBulkResult{
						{Path: "/path1", InstancesPurged: 1},
						{Path: "/path2", Error: "connection timeout"},
					}, nil
				},
			},
		},
		{
			name: "when PurgeCacheBulk returns an error",
			args: []string{"./rpaasv2", "purge", "-i", "my-instance", "-f"},
			fileContent: `[
  {
    "path": "/path1"
  }
]`,
			expectedError: "bulk purge error",
			client: &fake.FakeClient{
				FakePurgeCacheBulk: func(args rpaasclient.PurgeCacheBulkArgs) ([]rpaasclient.PurgeBulkResult, error) {
					return nil, fmt.Errorf("bulk purge error")
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpDir := t.TempDir()
			tmpFile := filepath.Join(tmpDir, "purge.json")
			err := os.WriteFile(tmpFile, []byte(tt.fileContent), 0644)
			require.NoError(t, err)

			args := append(tt.args, tmpFile)

			stdout := &bytes.Buffer{}
			stderr := &bytes.Buffer{}
			app := NewApp(stdout, stderr, tt.client)
			err = app.Run(args)
			if tt.expectedError != "" {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.expectedError)
				if tt.expected != "" {
					assert.Equal(t, tt.expected, stdout.String())
				}
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.expected, stdout.String())
			assert.Empty(t, stderr.String())
		})
	}
}
