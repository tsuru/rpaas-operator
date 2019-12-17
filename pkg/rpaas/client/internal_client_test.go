// Copyright 2019 tsuru authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package client

import (
	"context"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

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
			expectedError: "cannot create a client over tsuru without either target, token or service",
		},
		{
			name:          "missing Tsuru service",
			target:        "https://tsuru.example.com",
			token:         "some-token",
			expectedError: "cannot create a client over tsuru without either target, token or service",
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
				assert.Equal(t, tt.expected, rpaasClient.(*client))
			}
			if tt.teardown != nil {
				tt.teardown(t)
			}
		})
	}
}

func TestClientThroughTsuru_Scale(t *testing.T) {
	tests := []struct {
		name          string
		args          ScaleArgs
		expectedError string
		handler       http.HandlerFunc
	}{
		{
			name: "when replicas number is negative",
			args: ScaleArgs{
				Instance: "my-instance",
				Replicas: int32(-1),
			},
			expectedError: "rpaasv2: replicas must be greater or equal than zero",
		},
		{
			name: "when server returns an unexpected status code",
			args: ScaleArgs{
				Instance: "my-instance",
				Replicas: int32(777),
			},
			expectedError: ErrUnexpectedStatusCode.Error(),
			handler: func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusNotFound)
				fmt.Fprintf(w, "instance not found")
			},
		},
		{
			name: "when server returns the expected response",
			args: ScaleArgs{
				Instance: "my-instance",
				Replicas: int32(777),
			},
			handler: func(w http.ResponseWriter, r *http.Request) {
				assert.Equal(t, r.Method, "POST")
				assert.Equal(t, fmt.Sprintf("/services/%s/proxy/%s?callback=%s", FakeTsuruService, "my-instance", "/resources/my-instance/scale"), r.URL.RequestURI())
				assert.Equal(t, "Bearer f4k3t0k3n", r.Header.Get("Authorization"))
				assert.Equal(t, getBody(t, r), "quantity=777")
				w.WriteHeader(http.StatusOK)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client, server := newClientThroughTsuru(t, tt.handler)
			defer server.Close()
			_, err := client.Scale(context.TODO(), tt.args)
			if tt.expectedError == "" {
				require.NoError(t, err)
				return
			}
			assert.EqualError(t, err, tt.expectedError)
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
