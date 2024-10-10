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
				assert.Equal(t, fmt.Sprintf("/1.20/services/%s/resources/%s/certificate", FakeTsuruService, "my-instance"), r.URL.RequestURI())
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
				assert.Equal(t, fmt.Sprintf("/1.20/services/%s/resources/%s/certificate/", FakeTsuruService, "my-instance"), r.URL.RequestURI())
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
				assert.Equal(t, fmt.Sprintf("/1.20/services/%s/resources/%s/certificate/my-certificate", FakeTsuruService, "my-instance"), r.URL.RequestURI())
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
				assert.Equal(t, fmt.Sprintf("/1.20/services/%s/resources/%s/certificate/my+certificate", FakeTsuruService, "my-instance"), r.URL.RequestURI())
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

func TestClientThroughTsuru_ListCertManagerRequests(t *testing.T) {
	tests := map[string]struct {
		instance      string
		expectedError string
		expected      []types.CertManager
		handler       http.HandlerFunc
	}{
		"when instance is empty": {
			expectedError: "rpaasv2: instance cannot be empty",
		},

		"when server returns several requests": {
			instance: "my-instance",
			handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				assert.Equal(t, r.Method, "GET")
				assert.Equal(t, fmt.Sprintf("/1.20/services/%s/resources/%s/cert-manager", FakeTsuruService, "my-instance"), r.URL.RequestURI())
				assert.Equal(t, "Bearer f4k3t0k3n", r.Header.Get("Authorization"))

				fmt.Fprintf(w, `[{"issuer": "my-issuer", "dnsNames": ["www.example.com", "web.example.com"], "ipAddresses": ["169.196.254.100"]}, {"issuer": "my-issuer-1", "dnsNames": ["*.test"]}]`)
			}),
			expected: []types.CertManager{
				{
					Issuer:      "my-issuer",
					DNSNames:    []string{"www.example.com", "web.example.com"},
					IPAddresses: []string{"169.196.254.100"},
				},
				{
					Issuer:   "my-issuer-1",
					DNSNames: []string{"*.test"},
				},
			},
		},

		"when server returns an error": {
			instance: "my-instance",
			handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				assert.Equal(t, r.Method, "GET")
				assert.Equal(t, fmt.Sprintf("/1.20/services/%s/resources/%s/cert-manager", FakeTsuruService, "my-instance"), r.URL.RequestURI())
				assert.Equal(t, "Bearer f4k3t0k3n", r.Header.Get("Authorization"))

				w.WriteHeader(http.StatusInternalServerError)
				fmt.Fprintf(w, `{"Msg": "some error"}`)
			}),
			expectedError: `rpaasv2: unexpected status code: 500 Internal Server Error, detail: {"Msg": "some error"}`,
		},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			client, server := newClientThroughTsuru(t, tt.handler)
			defer server.Close()

			cmRequests, err := client.ListCertManagerRequests(context.TODO(), tt.instance)
			if tt.expectedError == "" {
				require.NoError(t, err)
				return
			}

			assert.EqualError(t, err, tt.expectedError)
			assert.Equal(t, cmRequests, tt.expected)
		})
	}
}

func TestClientThroughTsuru_UpdateCertManager(t *testing.T) {
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
				assert.Equal(t, fmt.Sprintf("/1.20/services/%s/resources/%s/cert-manager", FakeTsuruService, "my-instance"), r.URL.RequestURI())
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

func TestClientThroughTsuru_DeleteCertManagerByIssuer(t *testing.T) {
	tests := map[string]struct {
		instance      string
		issuer        string
		expectedError string
		handler       http.HandlerFunc
	}{
		"when removing a Cert Manager request with no issuer provided": {
			instance: "my-instance",
			handler: func(w http.ResponseWriter, r *http.Request) {
				assert.Equal(t, fmt.Sprintf("/1.20/services/%s/resources/%s/cert-manager", FakeTsuruService, "my-instance"), r.URL.RequestURI())
				assert.Equal(t, "DELETE", r.Method)
				w.WriteHeader(http.StatusOK)
			},
		},

		"when removing a Cert Manager request from a specific issuer": {
			instance: "my-instance",
			issuer:   "lets-encrypt",
			handler: func(w http.ResponseWriter, r *http.Request) {
				assert.Equal(t, fmt.Sprintf("/1.20/services/%s/resources/%s/cert-manager?issuer=lets-encrypt", FakeTsuruService, "my-instance"), r.URL.RequestURI())
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

			err := client.DeleteCertManagerByIssuer(context.TODO(), tt.instance, tt.issuer)
			if tt.expectedError == "" {
				require.NoError(t, err)
				return
			}

			assert.EqualError(t, err, tt.expectedError)
		})
	}
}

func TestClientThroughTsuru_DeleteCertManagerByName(t *testing.T) {
	tests := map[string]struct {
		instance      string
		name          string
		expectedError string
		handler       http.HandlerFunc
	}{
		"when removing a Cert Manager request with no issuer provided": {
			instance: "my-instance",
			handler: func(w http.ResponseWriter, r *http.Request) {
				assert.Equal(t, fmt.Sprintf("/1.20/services/%s/resources/%s/cert-manager", FakeTsuruService, "my-instance"), r.URL.RequestURI())
				assert.Equal(t, "DELETE", r.Method)
				w.WriteHeader(http.StatusOK)
			},
		},

		"when removing a Cert Manager request from a specific issuer": {
			instance: "my-instance",
			name:     "my-name",
			handler: func(w http.ResponseWriter, r *http.Request) {
				assert.Equal(t, fmt.Sprintf("/1.20/services/%s/resources/%s/cert-manager?name=my-name", FakeTsuruService, "my-instance"), r.URL.RequestURI())
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

			err := client.DeleteCertManagerByName(context.TODO(), tt.instance, tt.name)
			if tt.expectedError == "" {
				require.NoError(t, err)
				return
			}

			assert.EqualError(t, err, tt.expectedError)
		})
	}
}
