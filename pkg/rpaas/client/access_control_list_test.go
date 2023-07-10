// Copyright 2021 tsuru authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package client

import (
	"context"
	"fmt"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/tsuru/rpaas-operator/pkg/rpaas/client/types"
)

func TestClientThroughTsuru_AddAccessControlList(t *testing.T) {
	tests := []struct {
		name          string
		instance      string
		host          string
		port          int
		expectedError string
		handler       http.HandlerFunc
	}{
		{
			name:          "when instance is empty",
			host:          "some-host.com",
			port:          443,
			expectedError: "rpaasv2: instance cannot be empty",
		},
		{
			name:     "when AddAccessControlList is successful",
			instance: "my-instance",
			host:     "some-host.com",
			port:     80,
			handler: func(w http.ResponseWriter, r *http.Request) {
				assert.Equal(t, r.Method, "POST")
				assert.Equal(t, fmt.Sprintf("/services/%s/proxy/%s?callback=%s", FakeTsuruService, "my-instance", "/resources/my-instance/acl"), r.URL.RequestURI())
				assert.Equal(t, "Bearer f4k3t0k3n", r.Header.Get("Authorization"))
				w.WriteHeader(http.StatusCreated)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client, server := newClientThroughTsuru(t, tt.handler)
			defer server.Close()
			err := client.AddAccessControlList(context.TODO(), tt.instance, tt.host, tt.port)
			if tt.expectedError == "" {
				require.NoError(t, err)
				return
			}
			assert.EqualError(t, err, tt.expectedError)
		})
	}
}

func TestClientThroughTsuru_RemoveAccessControlList(t *testing.T) {
	tests := []struct {
		name          string
		instance      string
		host          string
		port          int
		expectedError string
		handler       http.HandlerFunc
	}{
		{
			name:          "when instance is empty",
			host:          "some-host.com",
			port:          443,
			expectedError: "rpaasv2: instance cannot be empty",
		},
		{
			name:     "when RemoveAccessControlList is successful",
			instance: "my-instance",
			host:     "some-host.com",
			port:     80,
			handler: func(w http.ResponseWriter, r *http.Request) {
				assert.Equal(t, r.Method, "DELETE")
				assert.Equal(t, fmt.Sprintf("/services/%s/proxy/%s?callback=%s", FakeTsuruService, "my-instance", "/resources/my-instance/acl"), r.URL.RequestURI())
				assert.Equal(t, "Bearer f4k3t0k3n", r.Header.Get("Authorization"))
				w.WriteHeader(http.StatusNoContent)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client, server := newClientThroughTsuru(t, tt.handler)
			defer server.Close()
			err := client.RemoveAccessControlList(context.TODO(), tt.instance, tt.host, tt.port)
			if tt.expectedError == "" {
				require.NoError(t, err)
				return
			}
			assert.EqualError(t, err, tt.expectedError)
		})
	}
}

func TestClientThroughTsuru_ListAccessControlList(t *testing.T) {
	tests := []struct {
		name          string
		instance      string
		expectedError string
		expectedAcls  []types.AllowedUpstream
		handler       http.HandlerFunc
	}{
		{
			name:          "when instance is empty",
			expectedError: "rpaasv2: instance cannot be empty",
		},
		{
			name:     "when ListAccessControlList is successful",
			instance: "my-instance",
			expectedAcls: []types.AllowedUpstream{
				{
					Host: "some-host.com",
					Port: 443,
				},
				{
					Host: "some-host2.com",
					Port: 80,
				},
			},
			handler: func(w http.ResponseWriter, r *http.Request) {
				assert.Equal(t, r.Method, "GET")
				assert.Equal(t, fmt.Sprintf("/services/%s/proxy/%s?callback=%s", FakeTsuruService, "my-instance", "/resources/my-instance/acl"), r.URL.RequestURI())
				assert.Equal(t, "Bearer f4k3t0k3n", r.Header.Get("Authorization"))
				fmt.Fprintf(w, `[{"host": "some-host.com", "port": 443}, {"host": "some-host2.com", "port": 80}]}`)
				w.WriteHeader(http.StatusOK)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client, server := newClientThroughTsuru(t, tt.handler)
			defer server.Close()
			acls, err := client.ListAccessControlList(context.TODO(), tt.instance)
			if tt.expectedError == "" {
				require.NoError(t, err)
				return
			}
			assert.EqualError(t, err, tt.expectedError)
			assert.Equal(t, tt.expectedAcls, acls)
		})
	}
}
