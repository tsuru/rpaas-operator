// Copyright 2020 tsuru authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package cmd

import (
	"bytes"
	"context"
	"fmt"
	"net/http"
	"os"
	"testing"

	"github.com/gorilla/websocket"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tsuru/rpaas-operator/pkg/rpaas/client"
	"github.com/tsuru/rpaas-operator/pkg/rpaas/client/fake"
)

func TestExec(t *testing.T) {
	var called bool

	tests := []struct {
		name           string
		args           []string
		expected       string
		expectedError  string
		expectedCalled bool
		client         client.Client
	}{
		{
			name: "with command and arguments",
			args: []string{"rpaasv2", "exec", "-s", "rpaasv2", "-i", "my-instance", "--", "my-command", "-arg1", "--arg2"},
			client: &fake.FakeClient{
				FakeExec: func(ctx context.Context, args client.ExecArgs) (*websocket.Conn, *http.Response, error) {
					called = true
					expected := client.ExecArgs{
						Command:  []string{"my-command", "-arg1", "--arg2"},
						Instance: "my-instance",
					}
					assert.Equal(t, expected, args)
					return nil, nil, fmt.Errorf("some error")
				},
			},
			expectedCalled: true,
			expectedError:  "some error",
		},
		{
			name: "with all options activated",
			args: []string{"rpaasv2", "exec", "-s", "rpaasv2", "-i", "my-instance", "--tty", "--interactive", "-p", "pod-1", "-c", "container-1", "--", "my-shell"},
			client: &fake.FakeClient{
				FakeExec: func(ctx context.Context, args client.ExecArgs) (*websocket.Conn, *http.Response, error) {
					called = true
					expected := client.ExecArgs{
						In:          os.Stdin,
						Command:     []string{"my-shell"},
						Instance:    "my-instance",
						Pod:         "pod-1",
						Container:   "container-1",
						TTY:         true,
						Interactive: true,
					}
					assert.Equal(t, expected, args)
					return nil, nil, fmt.Errorf("another error")
				},
			},
			expectedCalled: true,
			expectedError:  "another error",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			called = false
			stdout := &bytes.Buffer{}
			stderr := &bytes.Buffer{}
			app := NewApp(stdout, stderr, tt.client)
			err := app.Run(tt.args)
			if tt.expectedError != "" {
				assert.EqualError(t, err, tt.expectedError)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.expectedCalled, called)
			assert.Equal(t, tt.expected, stdout.String())
			assert.Empty(t, stderr.String())
		})
	}
}
