// Copyright 2025 tsuru authors. All rights reserved.
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
)

func TestPurgeCacheArgs_Validate(t *testing.T) {
	tests := []struct {
		name          string
		args          PurgeCacheArgs
		expectedError error
	}{
		{
			name: "when instance is empty",
			args: PurgeCacheArgs{
				Instance: "",
				Path:     "/path",
			},
			expectedError: ErrMissingInstance,
		},
		{
			name: "when path is empty",
			args: PurgeCacheArgs{
				Instance: "my-instance",
				Path:     "",
			},
			expectedError: ErrMissingPath,
		},
		{
			name: "when all required fields are provided",
			args: PurgeCacheArgs{
				Instance: "my-instance",
				Path:     "/path",
			},
			expectedError: nil,
		},
		{
			name: "when all fields are provided including optional ones",
			args: PurgeCacheArgs{
				Instance:     "my-instance",
				Path:         "/path",
				PreservePath: true,
				ExtraHeaders: map[string][]string{"X-Custom": {"value"}},
			},
			expectedError: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.args.Validate()
			if tt.expectedError == nil {
				require.NoError(t, err)
				return
			}
			assert.EqualError(t, err, tt.expectedError.Error())
		})
	}
}

func TestClientThroughTsuru_PurgeCache(t *testing.T) {
	tests := []struct {
		name          string
		args          PurgeCacheArgs
		expectedError string
		expectedCount int
		handler       http.HandlerFunc
	}{
		{
			name: "when instance is empty",
			args: PurgeCacheArgs{
				Instance: "",
				Path:     "/path",
			},
			expectedError: "rpaasv2: instance cannot be empty",
		},
		{
			name: "when path is empty",
			args: PurgeCacheArgs{
				Instance: "my-instance",
				Path:     "",
			},
			expectedError: "rpaasv2: path cannot be empty",
		},
		{
			name: "when server returns an unexpected status code",
			args: PurgeCacheArgs{
				Instance: "my-instance",
				Path:     "/index.html",
			},
			expectedError: "rpaasv2: unexpected status code: 404 Not Found, detail: instance not found",
			handler: func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusNotFound)
				fmt.Fprintf(w, "instance not found")
			},
		},
		{
			name: "when server returns success with purge count",
			args: PurgeCacheArgs{
				Instance: "my-instance",
				Path:     "/index.html",
			},
			expectedCount: 3,
			handler: func(w http.ResponseWriter, r *http.Request) {
				assert.Equal(t, "POST", r.Method)
				assert.Equal(t, fmt.Sprintf("/1.20/services/%s/resources/%s/purge", FakeTsuruService, "my-instance"), r.URL.RequestURI())
				assert.Equal(t, "Bearer f4k3t0k3n", r.Header.Get("Authorization"))
				assert.Equal(t, "application/x-www-form-urlencoded", r.Header.Get("Content-Type"))
				body := getBody(t, r)
				assert.Contains(t, body, "path=%2Findex.html")
				w.WriteHeader(http.StatusOK)
				fmt.Fprintf(w, "Object purged on 3 servers")
			},
		},
		{
			name: "when server returns success with preserve path option",
			args: PurgeCacheArgs{
				Instance:     "my-instance",
				Path:         "/api/*",
				PreservePath: true,
			},
			expectedCount: 5,
			handler: func(w http.ResponseWriter, r *http.Request) {
				assert.Equal(t, "POST", r.Method)
				assert.Equal(t, fmt.Sprintf("/1.20/services/%s/resources/%s/purge", FakeTsuruService, "my-instance"), r.URL.RequestURI())
				assert.Equal(t, "Bearer f4k3t0k3n", r.Header.Get("Authorization"))
				assert.Equal(t, "application/x-www-form-urlencoded", r.Header.Get("Content-Type"))
				body := getBody(t, r)
				assert.Contains(t, body, "path=%2Fapi%2F%2A")
				assert.Contains(t, body, "preserve_path=true")
				w.WriteHeader(http.StatusOK)
				fmt.Fprintf(w, "Object purged on 5 servers")
			},
		},
		{
			name: "when server returns success with extra headers",
			args: PurgeCacheArgs{
				Instance: "my-instance",
				Path:     "/cache",
				ExtraHeaders: map[string][]string{
					"X-Custom-Header": {"value1", "value2"},
					"X-Another":       {"value3"},
				},
			},
			expectedCount: 2,
			handler: func(w http.ResponseWriter, r *http.Request) {
				assert.Equal(t, "POST", r.Method)
				assert.Equal(t, fmt.Sprintf("/1.20/services/%s/resources/%s/purge", FakeTsuruService, "my-instance"), r.URL.RequestURI())
				assert.Equal(t, "Bearer f4k3t0k3n", r.Header.Get("Authorization"))
				assert.Equal(t, "application/x-www-form-urlencoded", r.Header.Get("Content-Type"))
				body := getBody(t, r)
				assert.Contains(t, body, "path=%2Fcache")
				w.WriteHeader(http.StatusOK)
				fmt.Fprintf(w, "Object purged on 2 servers")
			},
		},
		{
			name: "when server returns response without count",
			args: PurgeCacheArgs{
				Instance: "my-instance",
				Path:     "/something",
			},
			expectedCount: 0,
			handler: func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusOK)
				fmt.Fprintf(w, "Purge completed")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client, server := newClientThroughTsuru(t, tt.handler)
			defer server.Close()
			count, err := client.PurgeCache(context.TODO(), tt.args)
			if tt.expectedError != "" {
				assert.EqualError(t, err, tt.expectedError)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.expectedCount, count)
		})
	}
}

func TestPurgeCacheBulkArgs_Validate(t *testing.T) {
	tests := []struct {
		name          string
		args          PurgeCacheBulkArgs
		expectedError string
	}{
		{
			name: "when instance is empty",
			args: PurgeCacheBulkArgs{
				Instance: "",
				Items: []PurgeCacheItem{
					{Path: "/path1"},
				},
			},
			expectedError: "rpaasv2: instance cannot be empty",
		},
		{
			name: "when items list is empty",
			args: PurgeCacheBulkArgs{
				Instance: "my-instance",
				Items:    []PurgeCacheItem{},
			},
			expectedError: "at least one purge item is required",
		},
		{
			name: "when an item has empty path",
			args: PurgeCacheBulkArgs{
				Instance: "my-instance",
				Items: []PurgeCacheItem{
					{Path: "/path1"},
					{Path: ""},
					{Path: "/path3"},
				},
			},
			expectedError: "path is required for item 1",
		},
		{
			name: "when all required fields are provided",
			args: PurgeCacheBulkArgs{
				Instance: "my-instance",
				Items: []PurgeCacheItem{
					{Path: "/path1"},
					{Path: "/path2"},
				},
			},
			expectedError: "",
		},
		{
			name: "when all fields are provided including optional ones",
			args: PurgeCacheBulkArgs{
				Instance: "my-instance",
				Items: []PurgeCacheItem{
					{
						Path:         "/path1",
						PreservePath: true,
						ExtraHeaders: map[string][]string{"X-Custom": {"value"}},
					},
					{
						Path:         "/path2",
						PreservePath: false,
					},
				},
			},
			expectedError: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.args.Validate()
			if tt.expectedError == "" {
				require.NoError(t, err)
				return
			}
			assert.EqualError(t, err, tt.expectedError)
		})
	}
}

func TestClientThroughTsuru_PurgeCacheBulk(t *testing.T) {
	tests := []struct {
		name            string
		args            PurgeCacheBulkArgs
		expectedError   string
		expectedResults []PurgeBulkResult
		handler         http.HandlerFunc
	}{
		{
			name: "when instance is empty",
			args: PurgeCacheBulkArgs{
				Instance: "",
				Items: []PurgeCacheItem{
					{Path: "/path1"},
				},
			},
			expectedError: "rpaasv2: instance cannot be empty",
		},
		{
			name: "when items list is empty",
			args: PurgeCacheBulkArgs{
				Instance: "my-instance",
				Items:    []PurgeCacheItem{},
			},
			expectedError: "at least one purge item is required",
		},
		{
			name: "when server returns an unexpected status code",
			args: PurgeCacheBulkArgs{
				Instance: "my-instance",
				Items: []PurgeCacheItem{
					{Path: "/index.html"},
				},
			},
			expectedError: "rpaasv2: unexpected status code: 404 Not Found, detail: instance not found",
			handler: func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusNotFound)
				fmt.Fprintf(w, "instance not found")
			},
		},
		{
			name: "when server returns success with all items purged",
			args: PurgeCacheBulkArgs{
				Instance: "my-instance",
				Items: []PurgeCacheItem{
					{Path: "/index.html"},
					{Path: "/api/users"},
				},
			},
			expectedResults: []PurgeBulkResult{
				{Path: "/index.html", InstancesPurged: 3},
				{Path: "/api/users", InstancesPurged: 3},
			},
			handler: func(w http.ResponseWriter, r *http.Request) {
				assert.Equal(t, "POST", r.Method)
				assert.Equal(t, fmt.Sprintf("/1.20/services/%s/resources/%s/purge/bulk", FakeTsuruService, "my-instance"), r.URL.RequestURI())
				assert.Equal(t, "Bearer f4k3t0k3n", r.Header.Get("Authorization"))
				assert.Equal(t, "application/json", r.Header.Get("Content-Type"))
				body := getBody(t, r)
				assert.Contains(t, body, `"path":"/index.html"`)
				assert.Contains(t, body, `"path":"/api/users"`)
				w.WriteHeader(http.StatusOK)
				fmt.Fprintf(w, `[{"path":"/index.html","instances_purged":3},{"path":"/api/users","instances_purged":3}]`)
			},
		},
		{
			name: "when server returns partial success with errors",
			args: PurgeCacheBulkArgs{
				Instance: "my-instance",
				Items: []PurgeCacheItem{
					{Path: "/index.html"},
					{Path: "/invalid"},
					{Path: "/api/users"},
				},
			},
			expectedResults: []PurgeBulkResult{
				{Path: "/index.html", InstancesPurged: 3},
				{Path: "/invalid", Error: "path not found"},
				{Path: "/api/users", InstancesPurged: 3},
			},
			handler: func(w http.ResponseWriter, r *http.Request) {
				assert.Equal(t, "POST", r.Method)
				assert.Equal(t, fmt.Sprintf("/1.20/services/%s/resources/%s/purge/bulk", FakeTsuruService, "my-instance"), r.URL.RequestURI())
				assert.Equal(t, "Bearer f4k3t0k3n", r.Header.Get("Authorization"))
				assert.Equal(t, "application/json", r.Header.Get("Content-Type"))
				w.WriteHeader(http.StatusInternalServerError)
				fmt.Fprintf(w, `[{"path":"/index.html","instances_purged":3},{"path":"/invalid","error":"path not found"},{"path":"/api/users","instances_purged":3}]`)
			},
		},
		{
			name: "when server returns success with preserve path and extra headers",
			args: PurgeCacheBulkArgs{
				Instance: "my-instance",
				Items: []PurgeCacheItem{
					{
						Path:         "/cache/*",
						PreservePath: true,
						ExtraHeaders: map[string][]string{
							"X-Custom": {"value1"},
						},
					},
				},
			},
			expectedResults: []PurgeBulkResult{
				{Path: "/cache/*", InstancesPurged: 5},
			},
			handler: func(w http.ResponseWriter, r *http.Request) {
				assert.Equal(t, "POST", r.Method)
				assert.Equal(t, fmt.Sprintf("/1.20/services/%s/resources/%s/purge/bulk", FakeTsuruService, "my-instance"), r.URL.RequestURI())
				assert.Equal(t, "Bearer f4k3t0k3n", r.Header.Get("Authorization"))
				assert.Equal(t, "application/json", r.Header.Get("Content-Type"))
				body := getBody(t, r)
				assert.Contains(t, body, `"path":"/cache/*"`)
				assert.Contains(t, body, `"preserve_path":true`)
				assert.Contains(t, body, `"extra_headers"`)
				w.WriteHeader(http.StatusOK)
				fmt.Fprintf(w, `[{"path":"/cache/*","instances_purged":5}]`)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client, server := newClientThroughTsuru(t, tt.handler)
			defer server.Close()
			results, err := client.PurgeCacheBulk(context.TODO(), tt.args)
			if tt.expectedError != "" {
				assert.EqualError(t, err, tt.expectedError)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.expectedResults, results)
		})
	}
}
