// Copyright 2023 tsuru authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package client

import (
	"context"
	"net/http"
	"net/url"
	"testing"

	"github.com/gorilla/websocket"
	"github.com/stretchr/testify/assert"
)

func TestClientThroughTsuru_Debug(t *testing.T) {
	var called bool
	tests := []struct {
		name           string
		args           DebugArgs
		expectedError  string
		expectedCalled bool
		handler        http.HandlerFunc
	}{
		{
			name:          "when instance is empty",
			expectedError: "rpaasv2: instance cannot be empty",
		},
		{
			name: "when all options are set",
			args: DebugArgs{
				Instance:       "my-instance",
				Command:        []string{"bash"},
				Pod:            "pod-1",
				Container:      "nginx",
				Interactive:    true,
				TTY:            true,
				Image:          "my-image/test",
				TerminalWidth:  uint16(80),
				TerminalHeight: uint16(24),
			},
			handler: func(w http.ResponseWriter, r *http.Request) {
				called = true
				assert.True(t, websocket.IsWebSocketUpgrade(r))
				assert.Equal(t, "Bearer f4k3t0k3n", r.Header.Get("Authorization"))
				expectedQS := url.Values{}
				expectedQS.Set("callback", "/resources/my-instance/debug")
				expectedQS.Set("ws", "true")
				expectedQS.Set("command", "bash")
				expectedQS.Set("pod", "pod-1")
				expectedQS.Set("container", "nginx")
				expectedQS.Set("width", "80")
				expectedQS.Set("height", "24")
				expectedQS.Set("tty", "true")
				expectedQS.Set("interactive", "true")
				expectedQS.Set("image", "my-image/test")
				assert.Equal(t, expectedQS, r.URL.Query())
				w.WriteHeader(http.StatusBadRequest)
			},
			expectedCalled: true,
			expectedError:  "websocket: bad handshake",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			called = false
			client, server := newClientThroughTsuru(t, tt.handler)
			defer server.Close()
			_, err := client.Debug(context.TODO(), tt.args)
			if tt.expectedError != "" {
				assert.EqualError(t, err, tt.expectedError)
				return
			}
			assert.NoError(t, err)
			assert.Equal(t, tt.expectedCalled, called)
		})
	}
}
