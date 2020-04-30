// Copyright 2020 tsuru authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package cmd

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tsuru/rpaas-operator/pkg/rpaas/client"
	"github.com/tsuru/rpaas-operator/pkg/rpaas/client/fake"
)

func TestCachePurge(t *testing.T) {
	tests := []struct {
		name          string
		args          []string
		expected      string
		expectedError string
		client        client.Client
	}{
		{
			name:          "when CachePurge method returns an error",
			args:          []string{"./rpaasv2", "cache", "purge", "-s", "some-service", "-i", "my-instance", "-path", "/some/path"},
			expectedError: "some error",
			client: &fake.FakeClient{
				FakeCachePurge: func(args client.CachePurgeArgs) (*http.Response, error) {
					require.Equal(t, args.Instance, "my-instance")
					require.Equal(t, args.Path, "/some/path")
					return nil, fmt.Errorf("some error")
				},
			},
		},
		{
			name:     "when CachePurge is successful",
			args:     []string{"./rpaasv2", "cache", "purge", "-s", "some-service", "-i", "my-instance", "-path", "/some/path"},
			expected: "Object purged on 2 servers",
			client: &fake.FakeClient{
				FakeCachePurge: func(args client.CachePurgeArgs) (*http.Response, error) {
					require.Equal(t, args.Instance, "my-instance")
					require.Equal(t, args.Path, "/some/path")
					return &http.Response{
						Status:     "200 OK",
						StatusCode: 200,
						Body:       ioutil.NopCloser(bytes.NewBufferString("Object purged on 2 servers")),
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
