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

func TestClientThroughTsuru_UpdateBlock(t *testing.T) {
	tests := []struct {
		name          string
		args          UpdateBlockArgs
		expectedError string
		handler       http.HandlerFunc
	}{
		{
			name:          "when instance is empty",
			expectedError: "rpaasv2: instance cannot be empty",
		},
		{
			name: "when block name is empty",
			args: UpdateBlockArgs{
				Instance: "some-instance",
				Content:  "some content",
			},
			expectedError: "rpaasv2: block name cannot be empty",
		},
		{
			name: "when content is empty",
			args: UpdateBlockArgs{
				Instance: "my-instance",
				Name:     "server",
			},
			expectedError: "rpaasv2: content cannot be empty",
		},
		{
			name: "when the server returns the expected response",
			args: UpdateBlockArgs{
				Instance: "my-instance",
				Name:     "http",
				Content:  "# NGINX configuration block",
			},
			handler: func(w http.ResponseWriter, r *http.Request) {
				assert.Equal(t, r.Method, "POST")
				assert.Equal(t, fmt.Sprintf("/1.20/services/%s/resources/%s/block", FakeTsuruService, "my-instance"), r.URL.RequestURI())
				assert.Equal(t, "Bearer f4k3t0k3n", r.Header.Get("Authorization"))
				assert.Equal(t, "application/x-www-form-urlencoded", r.Header.Get("Content-Type"))
				assert.Equal(t, "block_name=http&content=%23+NGINX+configuration+block", getBody(t, r))
				w.WriteHeader(http.StatusOK)
			},
		},
		{
			name: "when the server returns the expected response with server name",
			args: UpdateBlockArgs{
				Instance:   "my-instance",
				Name:       "http",
				Content:    "# NGINX configuration block",
				ServerName: "example.org",
				Extend:     true,
			},
			handler: func(w http.ResponseWriter, r *http.Request) {
				assert.Equal(t, r.Method, "POST")
				assert.Equal(t, fmt.Sprintf("/1.20/services/%s/resources/%s/block", FakeTsuruService, "my-instance"), r.URL.RequestURI())
				assert.Equal(t, "Bearer f4k3t0k3n", r.Header.Get("Authorization"))
				assert.Equal(t, "application/x-www-form-urlencoded", r.Header.Get("Content-Type"))
				assert.Equal(t, "block_name=http&content=%23+NGINX+configuration+block&extend=true&server_name=example.org", getBody(t, r))
				w.WriteHeader(http.StatusOK)
			},
		},
		{
			name: "when the server returns an error",
			args: UpdateBlockArgs{
				Instance: "my-instance",
				Name:     "server",
				Content:  "Some NGINX snippet",
			},
			expectedError: "rpaasv2: unexpected status code: 404 Not Found, detail: instance not found",
			handler: func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusNotFound)
				fmt.Fprintf(w, "instance not found")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client, server := newClientThroughTsuru(t, tt.handler)
			defer server.Close()
			err := client.UpdateBlock(context.TODO(), tt.args)
			if tt.expectedError == "" {
				require.NoError(t, err)
				return
			}
			assert.EqualError(t, err, tt.expectedError)
		})
	}
}

func TestClientThroughTsuru_DeleteBlock(t *testing.T) {
	tests := []struct {
		name          string
		args          DeleteBlockArgs
		expectedError string
		handler       http.HandlerFunc
	}{
		{
			name:          "when instance is empty",
			expectedError: "rpaasv2: instance cannot be empty",
		},
		{
			name: "when block name is empty",
			args: DeleteBlockArgs{
				Instance: "some-instance",
			},
			expectedError: "rpaasv2: block name cannot be empty",
		},
		{
			name: "when the server returns the expected response",
			args: DeleteBlockArgs{
				Instance: "my-instance",
				Name:     "http",
			},
			handler: func(w http.ResponseWriter, r *http.Request) {
				assert.Equal(t, r.Method, "DELETE")
				assert.Equal(t, fmt.Sprintf("/1.20/services/%s/resources/%s/block/http", FakeTsuruService, "my-instance"), r.URL.RequestURI())
				assert.Equal(t, "Bearer f4k3t0k3n", r.Header.Get("Authorization"))
				w.WriteHeader(http.StatusOK)
			},
		},
		{
			name: "when the server returns the expected response with server_name defined",
			args: DeleteBlockArgs{
				Instance:   "my-instance",
				Name:       "http",
				ServerName: "example.org",
			},
			handler: func(w http.ResponseWriter, r *http.Request) {
				assert.Equal(t, r.Method, "DELETE")
				assert.Equal(t, fmt.Sprintf("/1.20/services/%s/resources/%s/block/http?server_name=example.org", FakeTsuruService, "my-instance"), r.URL.RequestURI())
				assert.Equal(t, "Bearer f4k3t0k3n", r.Header.Get("Authorization"))
				w.WriteHeader(http.StatusOK)
			},
		},
		{
			name: "when the server returns an error",
			args: DeleteBlockArgs{
				Instance: "my-instance",
				Name:     "server",
			},
			expectedError: "rpaasv2: unexpected status code: 404 Not Found, detail: instance not found",
			handler: func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusNotFound)
				fmt.Fprintf(w, "instance not found")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client, server := newClientThroughTsuru(t, tt.handler)
			defer server.Close()
			err := client.DeleteBlock(context.TODO(), tt.args)
			if tt.expectedError == "" {
				require.NoError(t, err)
				return
			}
			assert.EqualError(t, err, tt.expectedError)
		})
	}
}

func TestClientThroughTsuru_ListBlocks(t *testing.T) {
	tests := []struct {
		name          string
		args          ListBlocksArgs
		expected      []types.Block
		expectedError string
		handler       http.HandlerFunc
	}{
		{
			name:          "when instance is empty",
			expectedError: "rpaasv2: instance cannot be empty",
		},
		{
			name: "when the server returns an error",
			args: ListBlocksArgs{
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
			args: ListBlocksArgs{
				Instance: "my-instance",
			},
			expected: []types.Block{
				{
					Name:    "http",
					Content: "Some HTTP conf",
				},
				{
					Name:    "server",
					Content: "Some server conf",
				},
			},
			handler: func(w http.ResponseWriter, r *http.Request) {
				assert.Equal(t, r.Method, "GET")
				assert.Equal(t, fmt.Sprintf("/1.20/services/%s/resources/%s/block", FakeTsuruService, "my-instance"), r.URL.RequestURI())
				assert.Equal(t, "Bearer f4k3t0k3n", r.Header.Get("Authorization"))
				fmt.Fprintf(w, `{"blocks": [{"block_name": "http", "content": "Some HTTP conf"}, {"block_name": "server", "content": "Some server conf"}]}`)
				w.WriteHeader(http.StatusOK)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client, server := newClientThroughTsuru(t, tt.handler)
			defer server.Close()
			blocks, err := client.ListBlocks(context.TODO(), tt.args)
			if tt.expectedError != "" {
				assert.EqualError(t, err, tt.expectedError)
				return
			}
			assert.NoError(t, err)
			assert.Equal(t, tt.expected, blocks)
		})
	}
}
