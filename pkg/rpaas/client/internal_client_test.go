// Copyright 2019 tsuru authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package client

import (
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var (
	FakeTsuruToken   = "f4k3t0k3n"
	FakeTsuruService = "rpaasv2"
)

func TestNewClientThroughTsuruWithOptions(t *testing.T) {
	tests := []struct {
		name          string
		target        string
		token         string
		service       string
		opts          ClientOptions
		expected      *client
		expectedError string
		setUp         func(t *testing.T)
		teardown      func(t *testing.T)
	}{
		{
			name:          "missing all mandatory arguments",
			expectedError: ErrMissingTsuruTarget.Error(),
		},
		{
			name:          "missing Tsuru service",
			target:        "https://tsuru.example.com",
			token:         "some-token",
			expectedError: ErrMissingTsuruService.Error(),
		},
		{
			name:    "creating a client successfully",
			target:  "https://tsuru.example.com",
			token:   "some-token",
			service: "rpaasv2",
			expected: &client{
				tsuruTarget:  "https://tsuru.example.com",
				tsuruToken:   "some-token",
				tsuruService: "rpaasv2",
				throughTsuru: true,
				client:       &http.Client{},
				ws:           websocket.DefaultDialer,
			},
		},
		{
			name:    "getting Tsuru target and token from env vars",
			service: "rpaasv2",
			opts: ClientOptions{
				Timeout: 5 * time.Second,
			},
			expected: &client{
				tsuruTarget:  "https://tsuru.example.com",
				tsuruToken:   "tsuru-token",
				tsuruService: "rpaasv2",
				throughTsuru: true,
				client: &http.Client{
					Timeout: 5 * time.Second,
				},
				ws: websocket.DefaultDialer,
			},
			setUp: func(t *testing.T) {
				require.NoError(t, os.Setenv("TSURU_TARGET", "https://tsuru.example.com"))
				require.NoError(t, os.Setenv("TSURU_TOKEN", "tsuru-token"))
			},
			teardown: func(t *testing.T) {
				require.NoError(t, os.Unsetenv("TSURU_TARGET"))
				require.NoError(t, os.Unsetenv("TSURU_TOKEN"))
			},
		},
		{
			name:    "when tsuru target and token both are set on args and env vars, should prefer the args ones",
			target:  "https://tsuru.example.com",
			token:   "tok3n",
			service: "rpaasv2",
			expected: &client{
				tsuruTarget:  "https://tsuru.example.com",
				tsuruToken:   "tok3n",
				tsuruService: "rpaasv2",
				throughTsuru: true,
				client:       &http.Client{},
				ws:           websocket.DefaultDialer,
			},
			setUp: func(t *testing.T) {
				require.NoError(t, os.Setenv("TSURU_TARGET", "https://other.tsuru.example.com"))
				require.NoError(t, os.Setenv("TSURU_TOKEN", "tsuru-token"))
			},
			teardown: func(t *testing.T) {
				require.NoError(t, os.Unsetenv("TSURU_TARGET"))
				require.NoError(t, os.Unsetenv("TSURU_TOKEN"))
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.setUp != nil {
				tt.setUp(t)
			}
			rpaasClient, err := NewClientThroughTsuruWithOptions(tt.target, tt.token, tt.service, tt.opts)
			if tt.expectedError != "" {
				assert.EqualError(t, err, tt.expectedError)
			} else {
				require.NoError(t, err)
				rpaasClient.(*client).client.Transport = nil // void compare
				assert.Equal(t, tt.expected, rpaasClient.(*client))
			}
			if tt.teardown != nil {
				tt.teardown(t)
			}
		})
	}
}

func newClientThroughTsuru(t *testing.T, h http.Handler) (Client, *httptest.Server) {
	server := httptest.NewServer(h)
	client, err := NewClientThroughTsuru(server.URL, FakeTsuruToken, FakeTsuruService)
	require.NoError(t, err)
	return client, server
}

func getBody(t *testing.T, r *http.Request) string {
	body, err := ioutil.ReadAll(r.Body)
	require.NoError(t, err)
	defer r.Body.Close()
	return string(body)
}
