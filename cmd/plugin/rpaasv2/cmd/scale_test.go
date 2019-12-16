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
)

func TestScale(t *testing.T) {
	tests := []struct {
		name          string
		args          []string
		expected      string
		expectedError string
		client        client.Client
	}{
		{
			name:          "when Scale method returns an error",
			args:          []string{"./rpaasv2", "scale", "-s", "some-service", "-i", "my-instance", "-q", "2"},
			expectedError: "some error",
			client: &fake.FakeClient{
				FakeScale: func(args client.ScaleArgs) (*http.Response, error) {
					require.Equal(t, args.Instance, "my-instance")
					require.Equal(t, args.Replicas, int32(2))
					return nil, fmt.Errorf("some error")
				},
			},
		},
		{
			name:     "scaling up some instance",
			args:     []string{"./rpaasv2", "scale", "-s", "some-service", "-i", "my-instance", "-q", "777"},
			expected: "Instance successfully scaled to 777 replica(s)\n",
			client: &fake.FakeClient{
				FakeScale: func(args client.ScaleArgs) (*http.Response, error) {
					require.Equal(t, args.Instance, "my-instance")
					require.Equal(t, args.Replicas, int32(777))
					return nil, nil
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
