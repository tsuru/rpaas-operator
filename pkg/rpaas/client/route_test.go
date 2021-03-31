// Copyright 2021 tsuru authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package client

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/tsuru/rpaas-operator/pkg/rpaas/client/types"
)

func TestClientThroughTsuru_DeleteRoute(t *testing.T) {
	tests := []struct {
		name          string
		args          DeleteRouteArgs
		expectedError string
		handler       http.HandlerFunc
	}{
		{
			name:          "when instance is empty",
			expectedError: "rpaasv2: instance cannot be empty",
		},
		{
			name: "when path is empty",
			args: DeleteRouteArgs{
				Instance: "my-instance",
			},
			expectedError: "rpaasv2: path cannot be empty",
		},
		{
			name: "when the server returns an error",
			args: DeleteRouteArgs{
				Instance: "my-instance",
				Path:     "/custom/path",
			},
			expectedError: "rpaasv2: unexpected status code: 404 Not Found, detail: instance not found",
			handler: func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusNotFound)
				fmt.Fprintf(w, "instance not found")
			},
		},
		{
			name: "when the server returns the expected response",
			args: DeleteRouteArgs{
				Instance: "my-instance",
				Path:     "/custom/path",
			},
			handler: func(w http.ResponseWriter, r *http.Request) {
				assert.Equal(t, r.Method, "DELETE")
				assert.Equal(t, fmt.Sprintf("/services/%s/proxy/%s?callback=%s", FakeTsuruService, "my-instance", "/resources/my-instance/route"), r.URL.RequestURI())
				assert.Equal(t, "Bearer f4k3t0k3n", r.Header.Get("Authorization"))
				assert.Equal(t, "application/x-www-form-urlencoded", r.Header.Get("Content-Type"))
				assert.Equal(t, "path=%2Fcustom%2Fpath", getBody(t, r))
				w.WriteHeader(http.StatusOK)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client, server := newClientThroughTsuru(t, tt.handler)
			defer server.Close()
			err := client.DeleteRoute(context.TODO(), tt.args)
			if tt.expectedError != "" {
				assert.EqualError(t, err, tt.expectedError)
				return
			}
			assert.NoError(t, err)
		})
	}
}

func TestClientThroughTsuru_ListRoutes(t *testing.T) {
	tests := []struct {
		name          string
		args          ListRoutesArgs
		expected      []types.Route
		expectedError string
		handler       http.HandlerFunc
	}{
		{
			name:          "when instance is empty",
			expectedError: "rpaasv2: instance cannot be empty",
		},
		{
			name: "when the server returns an error",
			args: ListRoutesArgs{
				Instance: "my-instance",
			},
			expectedError: "rpaasv2: unexpected status code: 404 Not Found, detail: instance not found",
			handler: func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusNotFound)
				fmt.Fprintf(w, "instance not found")
			},
		},
		{
			name: "when the server returns the expected response",
			args: ListRoutesArgs{
				Instance: "my-instance",
			},
			expected: []types.Route{
				{
					Path:        "/static",
					Destination: "static.apps.tsuru.example.com",
				},
				{
					Path:        "/login",
					Destination: "login.apps.tsuru.example.com",
					HTTPSOnly:   true,
				},
				{
					Path:    "/custom/path",
					Content: "# some NGINX configuration",
				},
			},
			handler: func(w http.ResponseWriter, r *http.Request) {
				assert.Equal(t, r.Method, "GET")
				assert.Equal(t, fmt.Sprintf("/services/%s/proxy/%s?callback=%s", FakeTsuruService, "my-instance", "/resources/my-instance/route"), r.URL.RequestURI())
				assert.Equal(t, "Bearer f4k3t0k3n", r.Header.Get("Authorization"))
				fmt.Fprintf(w, `{"paths": [{"path": "/static", "destination": "static.apps.tsuru.example.com"}, {"path": "/login", "destination": "login.apps.tsuru.example.com", "https_only": true}, {"path": "/custom/path", "content": "# some NGINX configuration"}]}`)
				w.WriteHeader(http.StatusOK)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client, server := newClientThroughTsuru(t, tt.handler)
			defer server.Close()
			blocks, err := client.ListRoutes(context.TODO(), tt.args)
			if tt.expectedError != "" {
				assert.EqualError(t, err, tt.expectedError)
				return
			}
			assert.NoError(t, err)
			assert.Equal(t, tt.expected, blocks)
		})
	}
}

func TestClientThroughTsuru_UpdateRoute(t *testing.T) {
	tests := []struct {
		name          string
		args          UpdateRouteArgs
		expectedError string
		handler       http.HandlerFunc
	}{
		{
			name:          "when instance is empty",
			expectedError: "rpaasv2: instance cannot be empty",
		},
		{
			name: "when path is empty",
			args: UpdateRouteArgs{
				Instance: "my-instance",
			},
			expectedError: "rpaasv2: path cannot be empty",
		},
		{
			name: "when the server returns an error",
			args: UpdateRouteArgs{
				Instance:    "my-instance",
				Path:        "/app",
				Destination: "app.tsuru.example.com",
			},
			expectedError: "rpaasv2: unexpected status code: 404 Not Found, detail: instance not found",
			handler: func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusNotFound)
				fmt.Fprintf(w, "instance not found")
			},
		},
		{
			name: "when the server returns the expected response",
			args: UpdateRouteArgs{
				Instance:    "my-instance",
				Path:        "/app",
				Destination: "app.tsuru.example.com",
			},
			handler: func(w http.ResponseWriter, r *http.Request) {
				assert.Equal(t, r.Method, "POST")
				assert.Equal(t, fmt.Sprintf("/services/%s/proxy/%s?callback=%s", FakeTsuruService, "my-instance", "/resources/my-instance/route"), r.URL.RequestURI())
				assert.Equal(t, "Bearer f4k3t0k3n", r.Header.Get("Authorization"))
				expected := url.Values{
					"path":        []string{"/app"},
					"destination": []string{"app.tsuru.example.com"},
				}
				values, err := url.ParseQuery(getBody(t, r))
				assert.NoError(t, err)
				assert.Equal(t, expected, values)
				w.WriteHeader(http.StatusCreated)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client, server := newClientThroughTsuru(t, tt.handler)
			defer server.Close()
			err := client.UpdateRoute(context.TODO(), tt.args)
			if tt.expectedError != "" {
				assert.EqualError(t, err, tt.expectedError)
				return
			}
			assert.NoError(t, err)
		})
	}
}
