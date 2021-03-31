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

func TestClientThroughTsuru_UpdateCertificate(t *testing.T) {
	tests := []struct {
		name          string
		args          UpdateCertificateArgs
		expectedError string
		handler       http.HandlerFunc
	}{
		{
			name: "when instance is empty",
			args: UpdateCertificateArgs{
				Certificate: "some cert",
				Key:         "some key",
			},
			expectedError: "rpaasv2: instance cannot be empty",
		},
		{
			name: "when certificate is empty",
			args: UpdateCertificateArgs{
				Instance: "my-instance",
				Key:      "some key",
			},
			expectedError: "rpaasv2: certificate cannot be empty",
		},
		{
			name: "when key is empty",
			args: UpdateCertificateArgs{
				Instance:    "my-instance",
				Certificate: "some cert",
			},
			expectedError: "rpaasv2: key cannot be empty",
		},
		{
			name: "when the server returns the expected response",
			args: UpdateCertificateArgs{
				Instance:    "my-instance",
				Certificate: `my certificate`,
				Key:         `my key`,
				boundary:    "custom-boundary",
			},
			handler: func(w http.ResponseWriter, r *http.Request) {
				assert.Equal(t, r.Method, "POST")
				assert.Equal(t, fmt.Sprintf("/services/%s/proxy/%s?callback=%s", FakeTsuruService, "my-instance", "/resources/my-instance/certificate"), r.URL.RequestURI())
				assert.Equal(t, "Bearer f4k3t0k3n", r.Header.Get("Authorization"))
				assert.Equal(t, "multipart/form-data; boundary=\"custom-boundary\"", r.Header.Get("Content-Type"))
				assert.Equal(t, "--custom-boundary\r\nContent-Disposition: form-data; name=\"cert\"; filename=\"cert.pem\"\r\nContent-Type: application/octet-stream\r\n\r\nmy certificate\r\n--custom-boundary\r\nContent-Disposition: form-data; name=\"key\"; filename=\"key.pem\"\r\nContent-Type: application/octet-stream\r\n\r\nmy key\r\n--custom-boundary\r\nContent-Disposition: form-data; name=\"name\"\r\n\r\n\r\n--custom-boundary--\r\n", getBody(t, r))
				w.WriteHeader(http.StatusOK)
			},
		},
		{
			name: "when the server returns an error",
			args: UpdateCertificateArgs{
				Instance:    "my-instance",
				Certificate: `my certificate`,
				Key:         `my key`,
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
			err := client.UpdateCertificate(context.TODO(), tt.args)
			if tt.expectedError == "" {
				require.NoError(t, err)
				return
			}
			assert.EqualError(t, err, tt.expectedError)
		})
	}
}
