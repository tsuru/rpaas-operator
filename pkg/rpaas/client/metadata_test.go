// Copyright 2024 tsuru authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package client

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/tsuru/rpaas-operator/pkg/rpaas/client/types"
)

func TestClientThroughTsuru_GetMetadata(t *testing.T) {
	tests := []struct {
		name          string
		instance      string
		expectedError string
		handler       http.HandlerFunc
	}{
		{
			name:          "missing instance",
			instance:      "",
			expectedError: "rpaasv2: instance cannot be empty",
			handler:       func(w http.ResponseWriter, r *http.Request) {},
		},
		{
			name:          "unexpected status code",
			instance:      "my-instance",
			expectedError: "rpaasv2: unexpected status code: 404 Not Found, detail: instance not found",
			handler: func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusNotFound)
				w.Write([]byte("instance not found"))
			},
		},
		{
			name:     "success",
			instance: "my-instance",
			handler: func(w http.ResponseWriter, r *http.Request) {
				assert.Equal(t, r.Method, "GET")
				assert.Equal(t, fmt.Sprintf("/1.20/services/%s/resources/%s/metadata", FakeTsuruService, "my-instance"), r.URL.RequestURI())
				assert.Equal(t, "Bearer f4k3t0k3n", r.Header.Get("Authorization"))

				metadata := types.Metadata{
					Labels:      []types.MetadataItem{},
					Annotations: []types.MetadataItem{},
				}

				m, _ := json.Marshal(metadata)
				w.WriteHeader(http.StatusOK)
				w.Write(m)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client, server := newClientThroughTsuru(t, tt.handler)
			defer server.Close()

			metadata, err := client.GetMetadata(context.TODO(), tt.instance)
			if tt.expectedError != "" {
				assert.EqualError(t, err, tt.expectedError)
				return
			}

			assert.NoError(t, err)
			assert.NotNil(t, metadata)
		})
	}
}

func TestClientThroughTsuru_SetMetadata(t *testing.T) {
	tests := []struct {
		name          string
		instance      string
		expectedError string
		handler       http.HandlerFunc
	}{
		{
			name:          "missing instance",
			instance:      "",
			expectedError: "rpaasv2: instance cannot be empty",
			handler:       func(w http.ResponseWriter, r *http.Request) {},
		},
		{
			name:          "unexpected status code",
			instance:      "my-instance",
			expectedError: "rpaasv2: unexpected status code: 404 Not Found, detail: instance not found",
			handler: func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusNotFound)
				w.Write([]byte("instance not found"))
			},
		},
		{
			name:     "success",
			instance: "my-instance",
			handler: func(w http.ResponseWriter, r *http.Request) {
				assert.Equal(t, r.Method, "POST")
				assert.Equal(t, fmt.Sprintf("/1.20/services/%s/resources/%s/metadata", FakeTsuruService, "my-instance"), r.URL.RequestURI())
				assert.Equal(t, "Bearer f4k3t0k3n", r.Header.Get("Authorization"))
				assert.Equal(t, "application/json", r.Header.Get("Content-Type"))
				assert.NotNil(t, r.Body)
				w.WriteHeader(http.StatusOK)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client, server := newClientThroughTsuru(t, tt.handler)
			defer server.Close()

			err := client.SetMetadata(context.TODO(), tt.instance, &types.Metadata{})
			if tt.expectedError != "" {
				assert.EqualError(t, err, tt.expectedError)
				return
			}

			assert.NoError(t, err)
		})
	}
}

func TestClientThroughTsuru_UnsetMetadata(t *testing.T) {
	tests := []struct {
		name          string
		instance      string
		expectedError string
		handler       http.HandlerFunc
	}{
		{
			name:          "missing instance",
			instance:      "",
			expectedError: "rpaasv2: instance cannot be empty",
			handler:       func(w http.ResponseWriter, r *http.Request) {},
		},
		{
			name:          "unexpected status code",
			instance:      "my-instance",
			expectedError: "rpaasv2: unexpected status code: 404 Not Found, detail: instance not found",
			handler: func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusNotFound)
				w.Write([]byte("instance not found"))
			},
		},
		{
			name:     "success",
			instance: "my-instance",
			handler: func(w http.ResponseWriter, r *http.Request) {
				assert.Equal(t, r.Method, "DELETE")
				assert.Equal(t, fmt.Sprintf("/1.20/services/%s/resources/%s/metadata", FakeTsuruService, "my-instance"), r.URL.RequestURI())
				assert.Equal(t, "Bearer f4k3t0k3n", r.Header.Get("Authorization"))
				assert.Equal(t, "application/json", r.Header.Get("Content-Type"))
				assert.NotNil(t, r.Body)
				w.WriteHeader(http.StatusOK)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client, server := newClientThroughTsuru(t, tt.handler)
			defer server.Close()

			err := client.UnsetMetadata(context.TODO(), tt.instance, &types.Metadata{})
			if tt.expectedError != "" {
				assert.EqualError(t, err, tt.expectedError)
				return
			}

			assert.NoError(t, err)
		})
	}
}
