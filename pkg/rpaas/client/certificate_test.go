// Copyright 2021 tsuru authors. All rights reserved.
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
	"github.com/stretchr/testify/require"
	"github.com/tsuru/rpaas-operator/pkg/rpaas/client/types"
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

func TestClientThroughTsuru_DeleteCertificate(t *testing.T) {
	tests := []struct {
		name          string
		args          DeleteCertificateArgs
		expectedError string
		handler       http.HandlerFunc
	}{
		{
			name:          "when instance is empty",
			args:          DeleteCertificateArgs{},
			expectedError: "rpaasv2: instance cannot be empty",
		},
		{
			name: "when certificate name is empty, should not return error since empty = default",
			args: DeleteCertificateArgs{
				Instance: "my-instance",
			},
			handler: func(w http.ResponseWriter, r *http.Request) {
				assert.Equal(t, r.Method, "DELETE")
				assert.Equal(t, fmt.Sprintf("/services/%s/proxy/%s?callback=%s", FakeTsuruService, "my-instance", "/resources/my-instance/certificate/"), r.URL.RequestURI())
				assert.Equal(t, "Bearer f4k3t0k3n", r.Header.Get("Authorization"))
				w.WriteHeader(http.StatusOK)
			},
		},
		{
			name: "when the server returns the expected response",
			args: DeleteCertificateArgs{
				Instance: "my-instance",
				Name:     "my-certificate",
			},
			handler: func(w http.ResponseWriter, r *http.Request) {
				assert.Equal(t, r.Method, "DELETE")
				assert.Equal(t, fmt.Sprintf("/services/%s/proxy/%s?callback=%s", FakeTsuruService, "my-instance", "/resources/my-instance/certificate/my-certificate"), r.URL.RequestURI())
				assert.Equal(t, "Bearer f4k3t0k3n", r.Header.Get("Authorization"))
				assert.Equal(t, "", getBody(t, r))
				w.WriteHeader(http.StatusOK)
			},
		},
		{
			name: "when the server returns an error",
			args: DeleteCertificateArgs{
				Instance: "my-instance",
				Name:     "my-certificate",
			},
			expectedError: "rpaasv2: unexpected status code: 404 Not Found, detail: instance not found",
			handler: func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusNotFound)
				fmt.Fprintf(w, "instance not found")
			},
		},
		{
			name: "when the certificate name has spaces should should query escape cert name",
			args: DeleteCertificateArgs{
				Instance: "my-instance",
				Name:     "my certificate",
			},
			handler: func(w http.ResponseWriter, r *http.Request) {
				assert.Equal(t, fmt.Sprintf("/services/%s/proxy/%s?callback=%s", FakeTsuruService, "my-instance", "/resources/my-instance/certificate/my+certificate"), r.URL.RequestURI())
				w.WriteHeader(http.StatusOK)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client, server := newClientThroughTsuru(t, tt.handler)
			defer server.Close()
			err := client.DeleteCertificate(context.TODO(), tt.args)
			if tt.expectedError == "" {
				require.NoError(t, err)
				return
			}
			assert.EqualError(t, err, tt.expectedError)
		})
	}
}

func TestClientThroughTsuru_DeleteUpdateCertManager(t *testing.T) {
	tests := map[string]struct {
		args          UpdateCertManagerArgs
		expectedError string
		handler       http.HandlerFunc
	}{
		"when instance is empty": {
			expectedError: "rpaasv2: instance cannot be empty",
		},

		"when cert-manager is successfully updated": {
			args: UpdateCertManagerArgs{
				Instance: "my-instance",
				CertManager: types.CertManager{
					Issuer:      "my-issuer",
					DNSNames:    []string{"my-instance.example.com"},
					IPAddresses: []string{"169.196.100.1"},
				},
			},
			handler: func(w http.ResponseWriter, r *http.Request) {
				assert.Equal(t, fmt.Sprintf("/services/%s/proxy/%s?callback=%s", FakeTsuruService, "my-instance", "/resources/my-instance/cert-manager"), r.URL.RequestURI())
				assert.Equal(t, "POST", r.Method)
				assert.Equal(t, "application/json", r.Header.Get("Content-Type"))
				var cm types.CertManager
				body := getBody(t, r)
				require.NoError(t, json.Unmarshal([]byte(body), &cm))
				assert.Equal(t, types.CertManager{
					Issuer:      "my-issuer",
					DNSNames:    []string{"my-instance.example.com"},
					IPAddresses: []string{"169.196.100.1"},
				}, cm)
				w.WriteHeader(http.StatusOK)
			},
		},

		"when server returns an error": {
			args: UpdateCertManagerArgs{
				Instance: "my-instance",
				CertManager: types.CertManager{
					Issuer:      "my-issuer",
					DNSNames:    []string{"my-instance.example.com"},
					IPAddresses: []string{"169.196.100.1"},
				},
			},
			handler: func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusBadRequest)
				fmt.Fprintf(w, `{Msg: "some error"}`)
			},
			expectedError: `rpaasv2: unexpected status code: 400 Bad Request, detail: {Msg: "some error"}`,
		},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			client, server := newClientThroughTsuru(t, tt.handler)
			defer server.Close()

			err := client.UpdateCertManager(context.TODO(), tt.args)
			if tt.expectedError == "" {
				require.NoError(t, err)
				return
			}

			assert.EqualError(t, err, tt.expectedError)
		})
	}
}

func TestClientThroughTsuru_DeleteCertManager(t *testing.T) {
	tests := map[string]struct {
		instance      string
		expectedError string
		handler       http.HandlerFunc
	}{
		"disabling cert-manager integration": {
			instance: "my-instance",
			handler: func(w http.ResponseWriter, r *http.Request) {
				assert.Equal(t, fmt.Sprintf("/services/%s/proxy/%s?callback=%s", FakeTsuruService, "my-instance", "/resources/my-instance/cert-manager"), r.URL.RequestURI())
				assert.Equal(t, "DELETE", r.Method)
				w.WriteHeader(http.StatusOK)
			},
		},

		"when server returns an error": {
			instance: "my-instance",
			handler: func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusNotFound)
				fmt.Fprintf(w, `{"Msg": "some error"}`)
			},
			expectedError: `rpaasv2: unexpected status code: 404 Not Found, detail: {"Msg": "some error"}`,
		},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			client, server := newClientThroughTsuru(t, tt.handler)
			defer server.Close()

			err := client.DeleteCertManager(context.TODO(), tt.instance)
			if tt.expectedError == "" {
				require.NoError(t, err)
				return
			}

			assert.EqualError(t, err, tt.expectedError)
		})
	}
}
