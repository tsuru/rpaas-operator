// Copyright 2021 tsuru authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package client

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tsuru/rpaas-operator/pkg/rpaas/client/types"
)

func TestClientThroughTsuru_UpdateAutoscale(t *testing.T) {
	handlerCount := 0
	tests := []struct {
		name          string
		args          UpdateAutoscaleArgs
		expectedError string
		handler       http.HandlerFunc
		handlerCount  int
	}{
		{
			name:          "when instance is empty",
			expectedError: "rpaasv2: instance cannot be empty",
		},
		{
			name: "when the server returns an error",
			args: UpdateAutoscaleArgs{
				Instance:    "my-instance",
				MinReplicas: pointerToInt(5),
				MaxReplicas: pointerToInt(10),
				CPU:         pointerToInt(27),
				Memory:      pointerToInt(33),
			},
			expectedError: "rpaasv2: unexpected status code: 404 Not Found, detail: instance not found",
			handler: func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusNotFound)
				fmt.Fprintf(w, "instance not found")
			},
		},
		{
			name: "when an autoscale spec is found",
			args: UpdateAutoscaleArgs{
				Instance:    "my-instance",
				MinReplicas: pointerToInt(5),
				MaxReplicas: pointerToInt(10),
				CPU:         pointerToInt(27),
				Memory:      pointerToInt(33),
			},
			handler: func(w http.ResponseWriter, r *http.Request) {
				switch handlerCount {
				case 0:
					assert.Equal(t, "GET", r.Method)
					assert.Equal(t, "Bearer f4k3t0k3n", r.Header.Get("Authorization"))
					assert.Equal(t, fmt.Sprintf("/services/%s/proxy/%s?callback=%s", FakeTsuruService, "my-instance", "/resources/my-instance/autoscale"), r.URL.RequestURI())
					w.WriteHeader(http.StatusOK)
					payload, err := json.Marshal(&types.Autoscale{})
					require.NoError(t, err)
					w.Write(payload)
					handlerCount++

				case 1:
					assert.Equal(t, "PATCH", r.Method)
					assert.Equal(t, "Bearer f4k3t0k3n", r.Header.Get("Authorization"))
					assert.Equal(t, fmt.Sprintf("/services/%s/proxy/%s?callback=%s", FakeTsuruService, "my-instance", "/resources/my-instance/autoscale"), r.URL.RequestURI())
					expected := url.Values{
						"max":    []string{"10"},
						"min":    []string{"5"},
						"cpu":    []string{"27"},
						"memory": []string{"33"},
					}
					values, err := url.ParseQuery(getBody(t, r))
					assert.NoError(t, err)
					assert.Equal(t, expected, values)
					w.WriteHeader(http.StatusCreated)
					handlerCount = 0
				}
			},
		},
		{
			name: "when an autoscale spec is not found",
			args: UpdateAutoscaleArgs{
				Instance:    "my-instance",
				MinReplicas: pointerToInt(5),
				MaxReplicas: pointerToInt(10),
				CPU:         pointerToInt(27),
				Memory:      pointerToInt(33),
			},
			handler: func(w http.ResponseWriter, r *http.Request) {
				switch handlerCount {
				case 0:
					assert.Equal(t, "GET", r.Method)
					assert.Equal(t, "Bearer f4k3t0k3n", r.Header.Get("Authorization"))
					assert.Equal(t, fmt.Sprintf("/services/%s/proxy/%s?callback=%s", FakeTsuruService, "my-instance", "/resources/my-instance/autoscale"), r.URL.RequestURI())
					w.WriteHeader(http.StatusNotFound)
					handlerCount++

				case 1:
					assert.Equal(t, "POST", r.Method)
					assert.Equal(t, "Bearer f4k3t0k3n", r.Header.Get("Authorization"))
					assert.Equal(t, fmt.Sprintf("/services/%s/proxy/%s?callback=%s", FakeTsuruService, "my-instance", "/resources/my-instance/autoscale"), r.URL.RequestURI())
					expected := url.Values{
						"max":    []string{"10"},
						"min":    []string{"5"},
						"cpu":    []string{"27"},
						"memory": []string{"33"},
					}
					values, err := url.ParseQuery(getBody(t, r))
					assert.NoError(t, err)
					assert.Equal(t, expected, values)
					w.WriteHeader(http.StatusOK)
					handlerCount = 0
				}
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client, server := newClientThroughTsuru(t, tt.handler)
			defer server.Close()
			err := client.UpdateAutoscale(context.TODO(), tt.args)
			if tt.expectedError != "" {
				assert.EqualError(t, err, tt.expectedError)
				return
			}
			assert.NoError(t, err)
		})
	}
}

func TestClientThroughTsuru_GetAutoscale(t *testing.T) {
	tests := []struct {
		name          string
		args          GetAutoscaleArgs
		expectedError string
		handler       http.HandlerFunc
	}{
		{
			name:          "when instance is empty",
			expectedError: "rpaasv2: instance cannot be empty",
		},
		{
			name: "when the server returns an error",
			args: GetAutoscaleArgs{
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
			args: GetAutoscaleArgs{
				Instance: "my-instance",
			},
			handler: func(w http.ResponseWriter, r *http.Request) {
				assert.Equal(t, r.Method, "GET")
				assert.Equal(t, fmt.Sprintf("/services/%s/proxy/%s?callback=%s", FakeTsuruService, "my-instance", "/resources/my-instance/autoscale"), r.URL.RequestURI())
				assert.Equal(t, "Bearer f4k3t0k3n", r.Header.Get("Authorization"))
				w.WriteHeader(http.StatusOK)
				fmt.Fprintf(w, "{\n\t\"minReplicas\": 2,\n\t\"maxReplicas\": 5,\n\t\"cpu\": 50,\n\t\"memory\": 55\n}\n")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client, server := newClientThroughTsuru(t, tt.handler)
			defer server.Close()
			_, err := client.GetAutoscale(context.TODO(), tt.args)
			if tt.expectedError != "" {
				assert.EqualError(t, err, tt.expectedError)
				return
			}
			assert.NoError(t, err)
		})
	}
}

func TestClientThroughTsuru_RemoveAutoscale(t *testing.T) {
	tests := []struct {
		name          string
		args          RemoveAutoscaleArgs
		expectedError string
		handler       http.HandlerFunc
	}{
		{
			name:          "when instance is empty",
			expectedError: "rpaasv2: instance cannot be empty",
		},
		{
			name: "when the server returns an error",
			args: RemoveAutoscaleArgs{
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
			args: RemoveAutoscaleArgs{
				Instance: "my-instance",
			},
			handler: func(w http.ResponseWriter, r *http.Request) {
				assert.Equal(t, r.Method, "DELETE")
				assert.Equal(t, fmt.Sprintf("/services/%s/proxy/%s?callback=%s", FakeTsuruService, "my-instance", "/resources/my-instance/autoscale"), r.URL.RequestURI())
				assert.Equal(t, "Bearer f4k3t0k3n", r.Header.Get("Authorization"))
				w.WriteHeader(http.StatusOK)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client, server := newClientThroughTsuru(t, tt.handler)
			defer server.Close()
			err := client.RemoveAutoscale(context.TODO(), tt.args)
			if tt.expectedError != "" {
				assert.EqualError(t, err, tt.expectedError)
				return
			}
			assert.NoError(t, err)
		})
	}
}

func pointerToInt(x int32) *int32 {
	return &x
}
