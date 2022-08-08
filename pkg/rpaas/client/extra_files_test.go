// Copyright 2022 tsuru authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package client

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tsuru/rpaas-operator/pkg/rpaas/client/types"
)

func TestClientThroughTsuru_DeleteExtraFiles(t *testing.T) {
	tests := []struct {
		name          string
		args          DeleteExtraFilesArgs
		expectedError string
		handler       http.HandlerFunc
	}{
		{
			name:          "when instance is empty",
			expectedError: "rpaasv2: instance cannot be empty",
		},
		{
			name: "when files is nil",
			args: DeleteExtraFilesArgs{
				Instance: "my-instance",
			},
			expectedError: "rpaasv2: file list must not be empty",
		},
		{
			name: "when the server returns an error",
			args: DeleteExtraFilesArgs{
				Instance: "my-instance",
				Files:    []string{"f1", "f2"},
			},
			expectedError: "rpaasv2: unexpected status code: 404 Not Found, detail: instance not found",
			handler: func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusNotFound)
				fmt.Fprintf(w, "instance not found")
			},
		},
		{
			name: "when the server returns the expected response",
			args: DeleteExtraFilesArgs{
				Instance: "my-instance",
				Files:    []string{"f1", "f2"},
			},
			handler: func(w http.ResponseWriter, r *http.Request) {
				assert.Equal(t, r.Method, "DELETE")
				assert.Equal(t, fmt.Sprintf("/services/%s/proxy/%s?callback=%s", FakeTsuruService, "my-instance", "/resources/my-instance/files"), r.URL.RequestURI())
				assert.Equal(t, "Bearer f4k3t0k3n", r.Header.Get("Authorization"))
				assert.Equal(t, "application/json", r.Header.Get("Content-Type"))
				assert.Equal(t, `["f1","f2"]`, getBody(t, r))
				w.WriteHeader(http.StatusOK)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client, server := newClientThroughTsuru(t, tt.handler)
			defer server.Close()
			err := client.DeleteExtraFiles(context.TODO(), tt.args)
			if tt.expectedError != "" {
				assert.EqualError(t, err, tt.expectedError)
				return
			}
			assert.NoError(t, err)
		})
	}
}

func TestClientThroughTsuru_AddExtraFiles(t *testing.T) {
	tests := []struct {
		name          string
		args          ExtraFilesArgs
		expectedError string
		handler       http.HandlerFunc
	}{
		{
			name:          "when instance is empty",
			expectedError: "rpaasv2: instance cannot be empty",
		},
		{
			name: "when files is nil",
			args: ExtraFilesArgs{
				Instance: "my-instance",
			},
			expectedError: "rpaasv2: file list must not be empty",
		},
		{
			name: "when the server returns an error",
			args: ExtraFilesArgs{
				Instance: "my-instance",
				Files: []types.RpaasFile{
					{
						Name:    "f1",
						Content: []byte("content 1"),
					},
					{
						Name:    "f2",
						Content: []byte("content 2"),
					},
				},
			},
			expectedError: "rpaasv2: unexpected status code: 404 Not Found, detail: instance not found",
			handler: func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusNotFound)
				fmt.Fprintf(w, "instance not found")
			},
		},
		{
			name: "when the server returns the expected response",
			args: ExtraFilesArgs{
				Instance: "my-instance",
				Files: []types.RpaasFile{
					{
						Name:    "f1",
						Content: []byte("content 1"),
					},
					{
						Name:    "f2",
						Content: []byte("content 2"),
					},
				},
			},
			handler: func(w http.ResponseWriter, r *http.Request) {
				assert.Equal(t, r.Method, "POST")
				assert.Equal(t, fmt.Sprintf("/services/%s/proxy/%s?callback=%s", FakeTsuruService, "my-instance", "/resources/my-instance/files"), r.URL.RequestURI())
				assert.Equal(t, "Bearer f4k3t0k3n", r.Header.Get("Authorization"))
				assert.Contains(t, r.Header.Get("Content-Type"), "multipart/form-data")

				body, err := io.ReadAll(r.Body)
				require.NoError(t, err)
				defer r.Body.Close()
				bodyString := string(body)
				assert.Contains(t, bodyString, "Content-Disposition: form-data; name=\"files\"; filename=\"f1\"\r\nContent-Type: application/octet-stream\r\n\r\ncontent 1\r\n")
				assert.Contains(t, bodyString, "Content-Disposition: form-data; name=\"files\"; filename=\"f2\"\r\nContent-Type: application/octet-stream\r\n\r\ncontent 2\r\n")
				w.WriteHeader(http.StatusCreated)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client, server := newClientThroughTsuru(t, tt.handler)
			defer server.Close()
			err := client.AddExtraFiles(context.TODO(), tt.args)
			if tt.expectedError != "" {
				assert.EqualError(t, err, tt.expectedError)
				return
			}
			assert.NoError(t, err)
		})
	}
}

func TestClientThroughTsuru_UpdateExtraFiles(t *testing.T) {
	tests := []struct {
		name          string
		args          ExtraFilesArgs
		expectedError string
		handler       http.HandlerFunc
	}{
		{
			name:          "when instance is empty",
			expectedError: "rpaasv2: instance cannot be empty",
		},
		{
			name: "when files is nil",
			args: ExtraFilesArgs{
				Instance: "my-instance",
			},
			expectedError: "rpaasv2: file list must not be empty",
		},
		{
			name: "when the server returns an error",
			args: ExtraFilesArgs{
				Instance: "my-instance",
				Files: []types.RpaasFile{
					{
						Name:    "f1",
						Content: []byte("content 1"),
					},
					{
						Name:    "f2",
						Content: []byte("content 2"),
					},
				},
			},
			expectedError: "rpaasv2: unexpected status code: 404 Not Found, detail: instance not found",
			handler: func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusNotFound)
				fmt.Fprintf(w, "instance not found")
			},
		},
		{
			name: "when the server returns the expected response",
			args: ExtraFilesArgs{
				Instance: "my-instance",
				Files: []types.RpaasFile{
					{
						Name:    "f1",
						Content: []byte("content 1"),
					},
					{
						Name:    "f2",
						Content: []byte("content 2"),
					},
				},
			},
			handler: func(w http.ResponseWriter, r *http.Request) {
				assert.Equal(t, r.Method, "PUT")
				assert.Equal(t, fmt.Sprintf("/services/%s/proxy/%s?callback=%s", FakeTsuruService, "my-instance", "/resources/my-instance/files"), r.URL.RequestURI())
				assert.Equal(t, "Bearer f4k3t0k3n", r.Header.Get("Authorization"))
				assert.Contains(t, r.Header.Get("Content-Type"), "multipart/form-data")

				body, err := io.ReadAll(r.Body)
				require.NoError(t, err)
				defer r.Body.Close()
				bodyString := string(body)
				assert.Contains(t, bodyString, "Content-Disposition: form-data; name=\"files\"; filename=\"f1\"\r\nContent-Type: application/octet-stream\r\n\r\ncontent 1\r\n")
				assert.Contains(t, bodyString, "Content-Disposition: form-data; name=\"files\"; filename=\"f2\"\r\nContent-Type: application/octet-stream\r\n\r\ncontent 2\r\n")
				w.WriteHeader(http.StatusOK)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client, server := newClientThroughTsuru(t, tt.handler)
			defer server.Close()
			err := client.UpdateExtraFiles(context.TODO(), tt.args)
			if tt.expectedError != "" {
				assert.EqualError(t, err, tt.expectedError)
				return
			}
			assert.NoError(t, err)
		})
	}
}

func TestClientThroughTsuru_GetExtraFiles(t *testing.T) {
	tests := []struct {
		name          string
		args          GetExtraFileArgs
		expectedError string
		expectedFile  types.RpaasFile
		handler       http.HandlerFunc
	}{
		{
			name:          "when instance is empty",
			expectedError: "rpaasv2: instance cannot be empty",
		},
		{
			name: "when files is nil",
			args: GetExtraFileArgs{
				Instance: "my-instance",
			},
			expectedError: "rpaasv2: file must have a name",
		},
		{
			name: "when the server returns an error",
			args: GetExtraFileArgs{
				Instance: "my-instance",
				FileName: "some-file",
			},
			expectedError: "rpaasv2: unexpected status code: 404 Not Found, detail: instance not found",
			handler: func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusNotFound)
				fmt.Fprintf(w, "instance not found")
			},
		},
		{
			name: "when the server returns the expected response",
			args: GetExtraFileArgs{
				Instance: "my-instance",
				FileName: "some-file",
			},
			expectedFile: types.RpaasFile{
				Name:    "some-file",
				Content: []byte("some content"),
			},
			handler: func(w http.ResponseWriter, r *http.Request) {
				assert.Equal(t, r.Method, "GET")
				assert.Equal(t, fmt.Sprintf("/services/%s/proxy/%s?callback=%s", FakeTsuruService, "my-instance", "/resources/my-instance/files/some-file"), r.URL.RequestURI())
				assert.Equal(t, "Bearer f4k3t0k3n", r.Header.Get("Authorization"))
				w.Header().Set("Content-type", "application/json")
				file := types.RpaasFile{
					Name:    "some-file",
					Content: []byte("some content"),
				}
				fileBytes, err := json.Marshal(file)
				assert.NoError(t, err)
				fmt.Fprint(w, string(fileBytes))
				w.WriteHeader(http.StatusOK)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client, server := newClientThroughTsuru(t, tt.handler)
			defer server.Close()
			file, err := client.GetExtraFile(context.TODO(), tt.args)
			if tt.expectedError != "" {
				assert.EqualError(t, err, tt.expectedError)
				return
			}
			if tt.expectedFile.Name != "" {
				assert.EqualValues(t, file, tt.expectedFile)
			}
			assert.NoError(t, err)
		})
	}
}

func TestClientThroughTsuru_ListExtraFiles(t *testing.T) {
	tests := []struct {
		name          string
		args          ListExtraFilesArgs
		expectedError string
		expectedFiles []types.RpaasFile
		handler       http.HandlerFunc
	}{
		{
			name:          "when instance is empty",
			expectedError: "rpaasv2: instance cannot be empty",
		},
		{
			name: "when the server returns an error",
			args: ListExtraFilesArgs{
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
			args: ListExtraFilesArgs{
				Instance: "my-instance",
			},
			expectedFiles: []types.RpaasFile{{Name: "f1"}, {Name: "f2"}},
			handler: func(w http.ResponseWriter, r *http.Request) {
				assert.Equal(t, r.Method, "GET")
				assert.Equal(t, fmt.Sprintf("/services/%s/proxy/%s?callback=%s", FakeTsuruService, "my-instance", "/resources/my-instance/files?show-content=false"), r.URL.RequestURI())
				assert.Equal(t, "Bearer f4k3t0k3n", r.Header.Get("Authorization"))
				w.Header().Set("Content-type", "application/json")
				files := []types.RpaasFile{
					{
						Name: "f1",
					},
					{
						Name: "f2",
					},
				}
				filesBytes, err := json.Marshal(files)
				assert.NoError(t, err)
				fmt.Fprint(w, string(filesBytes))
				w.WriteHeader(http.StatusOK)
			},
		},
		{
			name: "when the server returns the expected response with show-content query param",
			args: ListExtraFilesArgs{
				Instance:    "my-instance",
				ShowContent: true,
			},
			expectedFiles: []types.RpaasFile{
				{
					Name:    "f1",
					Content: []byte("c1"),
				},
				{
					Name:    "f2",
					Content: []byte("c2"),
				},
			},
			handler: func(w http.ResponseWriter, r *http.Request) {
				assert.Equal(t, r.Method, "GET")
				assert.Equal(t, fmt.Sprintf("/services/%s/proxy/%s?callback=%s", FakeTsuruService, "my-instance", "/resources/my-instance/files?show-content=true"), r.URL.RequestURI())
				assert.Equal(t, "Bearer f4k3t0k3n", r.Header.Get("Authorization"))
				w.Header().Set("Content-type", "application/json")
				files := []types.RpaasFile{
					{
						Name:    "f1",
						Content: []byte("c1"),
					},
					{
						Name:    "f2",
						Content: []byte("c2"),
					},
				}
				filesBytes, err := json.Marshal(files)
				assert.NoError(t, err)
				fmt.Fprint(w, string(filesBytes))
				w.WriteHeader(http.StatusOK)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client, server := newClientThroughTsuru(t, tt.handler)
			defer server.Close()
			files, err := client.ListExtraFiles(context.TODO(), tt.args)
			if tt.expectedError != "" {
				assert.EqualError(t, err, tt.expectedError)
				return
			}
			if tt.expectedFiles != nil {
				assert.EqualValues(t, tt.expectedFiles, files)
			}
			assert.NoError(t, err)
		})
	}
}
