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
)

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
			expectedError: "rpaasv2: unexpected status code: 404 Not Found, detail: instance not found",
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
				assert.Equal(t, "application/x-www-form-urlencoded", r.Header.Get("Content-Type"))
				assert.Equal(t, "quantity=777", getBody(t, r))
				w.WriteHeader(http.StatusOK)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client, server := newClientThroughTsuru(t, tt.handler)
			defer server.Close()
			err := client.Scale(context.TODO(), tt.args)
			if tt.expectedError == "" {
				require.NoError(t, err)
				return
			}
			assert.EqualError(t, err, tt.expectedError)
		})
	}
}
