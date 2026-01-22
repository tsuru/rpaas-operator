// Copyright 2021 tsuru authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package cmd

import (
	"bytes"
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	rpaasclient "github.com/tsuru/rpaas-operator/pkg/rpaas/client"
	"github.com/tsuru/rpaas-operator/pkg/rpaas/client/fake"
)

func TestLog(t *testing.T) {
	tests := []struct {
		name          string
		args          []string
		expected      string
		expectedError string
		client        rpaasclient.Client
	}{
		{
			name:          "when Log returns an error",
			args:          []string{"./rpaasv2", "log", "-i", "my-instance"},
			expectedError: "some error",
			client: &fake.FakeClient{
				FakeLog: func(args rpaasclient.LogArgs) error {
					expected := rpaasclient.LogArgs{
						Out:      &bytes.Buffer{},
						Instance: "my-instance",
						Color:    true,
					}
					assert.Equal(t, expected, args)
					return fmt.Errorf("some error")
				},
			},
		},
		{
			name: "when Log returns no error",
			args: []string{"./rpaasv2", "logs", "-i", "my-instance", "--since", "2s", "--follow", "--pod", "some-pod", "--container", "some-container", "--lines", "15"},
			client: &fake.FakeClient{
				FakeLog: func(args rpaasclient.LogArgs) error {
					expected := rpaasclient.LogArgs{
						Out:       &bytes.Buffer{},
						Instance:  "my-instance",
						Since:     time.Second * 2,
						Follow:    true,
						Pod:       "some-pod",
						Container: "some-container",
						Lines:     15,
						Color:     true,
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
			err := app.Run(context.Background(), tt.args)
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
