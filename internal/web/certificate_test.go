// Copyright 2020 tsuru authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package web

import (
	"bytes"
	"crypto/tls"
	"fmt"
	"mime/multipart"
	"net/http"
	"net/url"
	"strings"
	"testing"

	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/tsuru/rpaas-operator/internal/config"
	"github.com/tsuru/rpaas-operator/internal/pkg/rpaas"
	"github.com/tsuru/rpaas-operator/internal/pkg/rpaas/fake"
	clientTypes "github.com/tsuru/rpaas-operator/pkg/rpaas/client/types"
)

func Test_updateCertificate(t *testing.T) {
	instanceName := "my-instance-name"
	boundary := "XXXXXXXXXXXXXXX"

	certPem := `-----BEGIN CERTIFICATE-----
MIIBhTCCASugAwIBAgIQIRi6zePL6mKjOipn+dNuaTAKBggqhkjOPQQDAjASMRAw
DgYDVQQKEwdBY21lIENvMB4XDTE3MTAyMDE5NDMwNloXDTE4MTAyMDE5NDMwNlow
EjEQMA4GA1UEChMHQWNtZSBDbzBZMBMGByqGSM49AgEGCCqGSM49AwEHA0IABD0d
7VNhbWvZLWPuj/RtHFjvtJBEwOkhbN/BnnE8rnZR8+sbwnc/KhCk3FhnpHZnQz7B
5aETbbIgmuvewdjvSBSjYzBhMA4GA1UdDwEB/wQEAwICpDATBgNVHSUEDDAKBggr
BgEFBQcDATAPBgNVHRMBAf8EBTADAQH/MCkGA1UdEQQiMCCCDmxvY2FsaG9zdDo1
NDUzgg4xMjcuMC4wLjE6NTQ1MzAKBggqhkjOPQQDAgNIADBFAiEA2zpJEPQyz6/l
Wf86aX6PepsntZv2GYlA5UpabfT2EZICICpJ5h/iI+i341gBmLiAFQOyTDT+/wQc
6MF9+Yw1Yy0t
-----END CERTIFICATE-----`

	keyPem := `-----BEGIN EC PRIVATE KEY-----
MHcCAQEEIIrYSSNQFaA2Hwf1duRSxKtLYX5CB04fSeQ6tF1aY/PuoAoGCCqGSM49
AwEHoUQDQgAEPR3tU2Fta9ktY+6P9G0cWO+0kETA6SFs38GecTyudlHz6xvCdz8q
EKTcWGekdmdDPsHloRNtsiCa697B2O9IFA==
-----END EC PRIVATE KEY-----`

	certificate, err := tls.X509KeyPair([]byte(certPem), []byte(keyPem))
	require.NoError(t, err)

	makeBodyRequest := func(cert, key, name string) string {
		b := &bytes.Buffer{}
		w := multipart.NewWriter(b)
		w.SetBoundary(boundary)
		if cert != "" {
			writer, err := w.CreateFormFile("cert", "cert.pem")
			require.NoError(t, err)
			writer.Write([]byte(cert))
		}
		if key != "" {
			writer, err := w.CreateFormFile("key", "key.pem")
			require.NoError(t, err)
			writer.Write([]byte(key))
		}
		if name != "" {
			err := w.WriteField("name", name)
			require.NoError(t, err)
		}
		w.Close()
		return b.String()
	}

	testCases := []struct {
		name         string
		requestBody  string
		expectedCode int
		expectedBody string
		manager      rpaas.RpaasManager
	}{
		{
			name:         "when no private key is sent",
			requestBody:  makeBodyRequest("some certificate", "", ""),
			expectedCode: 400,
			expectedBody: "key file is either not provided or not valid",
			manager:      &fake.RpaasManager{},
		},
		{
			name:         "when no certificate is sent",
			requestBody:  makeBodyRequest("", "some private key", ""),
			expectedCode: 400,
			expectedBody: "cert file is either not provided or not valid",
			manager:      &fake.RpaasManager{},
		},
		{
			name:         "when successfully adding a default certificate",
			requestBody:  makeBodyRequest(certPem, keyPem, ""),
			expectedCode: 200,
			expectedBody: "",
			manager: &fake.RpaasManager{
				FakeUpdateCertificate: func(instance, name string, c tls.Certificate) error {
					assert.Equal(t, "", name)
					assert.Equal(t, instance, instanceName)
					assert.Equal(t, c, certificate)
					return nil
				},
			},
		},
		{
			name:         "when successfully adding a named certificate",
			requestBody:  makeBodyRequest(certPem, keyPem, "mycert"),
			expectedCode: 200,
			expectedBody: "",
			manager: &fake.RpaasManager{
				FakeUpdateCertificate: func(instance, name string, c tls.Certificate) error {
					assert.Equal(t, "mycert", name)
					assert.Equal(t, instance, instanceName)
					assert.Equal(t, c, certificate)
					return nil
				},
			},
		},
		{
			name:         "when UpdateCertificate method returns ",
			requestBody:  makeBodyRequest(certPem, keyPem, ""),
			expectedCode: 400,
			expectedBody: "{\"Msg\":\"some error\"}",
			manager: &fake.RpaasManager{
				FakeUpdateCertificate: func(instance, name string, c tls.Certificate) error {
					return &rpaas.ValidationError{Msg: "some error"}
				},
			},
		},
	}

	for _, tt := range testCases {
		t.Run(tt.name, func(t *testing.T) {
			srv := newTestingServer(t, tt.manager)
			defer srv.Close()
			path := fmt.Sprintf("%s/resources/%s/certificate", srv.URL, instanceName)
			request, err := http.NewRequest(http.MethodPost, path, strings.NewReader(tt.requestBody))
			require.NoError(t, err)
			request.Header.Set(echo.HeaderContentType, fmt.Sprintf(`%s; boundary=%s`, echo.MIMEMultipartForm, boundary))
			rsp, err := srv.Client().Do(request)
			require.NoError(t, err)
			assert.Equal(t, tt.expectedCode, rsp.StatusCode)
			assert.Equal(t, tt.expectedBody, bodyContent(rsp))
		})
	}
}

func Test_deleteCertificate(t *testing.T) {
	tests := []struct {
		name         string
		manager      rpaas.RpaasManager
		instance     string
		certName     string
		expectedCode int
		expectedBody string
	}{
		{
			name: "when the instance does not exist",
			manager: &fake.RpaasManager{
				FakeDeleteCertificate: func(instance, name string) error {
					return &rpaas.NotFoundError{}
				},
			},
			instance:     "my-instance",
			expectedCode: http.StatusNotFound,
			expectedBody: "{\"Msg\":\"\"}",
		},
		{
			name: "when the certificate exists",
			manager: &fake.RpaasManager{
				FakeDeleteCertificate: func(instance, name string) error {
					return nil
				},
			},
			instance:     "real-instance",
			expectedCode: http.StatusOK,
			expectedBody: "",
		},
		{
			name:     "when the certificate does not exist",
			instance: "real-instance",
			manager: &fake.RpaasManager{
				FakeDeleteCertificate: func(instance, name string) error {
					return &rpaas.NotFoundError{Msg: fmt.Sprintf("no certificate bound to instance %q", instance)}
				},
			},
			expectedCode: http.StatusNotFound,
			expectedBody: "{\"Msg\":\"no certificate bound to instance \\\"real-instance\\\"\"}",
		},
		{
			name:     "passing a certificate name and asserting it",
			instance: "my-instance",
			certName: "junda",
			manager: &fake.RpaasManager{
				FakeDeleteCertificate: func(instance, name string) error {
					assert.Equal(t, "my-instance", instance)
					assert.Equal(t, "junda", name)
					return nil
				},
			},
			expectedCode: http.StatusOK,
		},
		{
			name:     "passing a certificate name and asserting it",
			instance: "my-instance",
			certName: url.QueryEscape("cert name with spaces"),
			manager: &fake.RpaasManager{
				FakeDeleteCertificate: func(instance, name string) error {
					assert.Equal(t, "my-instance", instance)
					assert.Equal(t, "cert name with spaces", name)
					return nil
				},
			},
			expectedCode: http.StatusOK,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			srv := newTestingServer(t, tt.manager)
			defer srv.Close()
			var path string
			if tt.certName != "" {
				path = fmt.Sprintf("%s/resources/%s/certificate/%s", srv.URL, tt.instance, tt.certName)
			} else {
				path = fmt.Sprintf("%s/resources/%s/certificate", srv.URL, tt.instance)
			}
			request, err := http.NewRequest(http.MethodDelete, path, nil)
			require.NoError(t, err)
			rsp, err := srv.Client().Do(request)
			require.NoError(t, err)
			assert.Equal(t, tt.expectedCode, rsp.StatusCode)
			assert.Equal(t, tt.expectedBody, bodyContent(rsp))
		})

	}
}

func Test_GetCertificates(t *testing.T) {
	tests := []struct {
		name         string
		manager      rpaas.RpaasManager
		instance     string
		expectedCode int
		expectedBody string
		config       *config.RpaasConfig
	}{
		{
			name:         "when the instance does not exist",
			manager:      &fake.RpaasManager{},
			instance:     "my-instance",
			expectedCode: http.StatusOK,
			expectedBody: "[]",
		},
		{
			name: "when the instance and certificate exists",
			manager: &fake.RpaasManager{
				FakeGetCertificates: func(instanceName string) ([]rpaas.CertificateData, error) {
					return []rpaas.CertificateData{
						{
							Name:        "cert-name",
							Certificate: `my-certificate`,
							Key:         `my-key`,
						},
					}, nil
				},
			},
			instance:     "real-instance",
			expectedCode: http.StatusOK,
			expectedBody: "[{\"name\":\"cert-name\",\"certificate\":\"my-certificate\",\"key\":\"my-key\"}]",
		},
		{
			name: "when the instance exists but the certificate has a missing key",
			manager: &fake.RpaasManager{
				FakeGetCertificates: func(instanceName string) ([]rpaas.CertificateData, error) {
					return nil, fmt.Errorf("key data not found")
				},
			},
			instance:     "real-instance",
			expectedCode: http.StatusInternalServerError,
			expectedBody: "{\"message\":\"key data not found\"}",
		},
		{
			name:     "when suppressing private key is enabled",
			instance: "my-instance",
			config: &config.RpaasConfig{
				SuppressPrivateKeyOnCertificatesList: true,
			},
			manager: &fake.RpaasManager{
				FakeGetCertificates: func(instance string) ([]rpaas.CertificateData, error) {
					return []rpaas.CertificateData{
						{
							Name:        "my-example.com",
							Certificate: "X509 certificate",
							Key:         "PEM ENCODED KEY",
						},
					}, nil
				},
			},
			expectedCode: http.StatusOK,
			expectedBody: "[{\"name\":\"my-example.com\",\"certificate\":\"X509 certificate\",\"key\":\"*** private ***\"}]",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.config != nil {
				config.Set(*tt.config)
			}
			srv := newTestingServer(t, tt.manager)
			defer srv.Close()
			path := fmt.Sprintf("%s/resources/%s/certificate", srv.URL, tt.instance)
			request, err := http.NewRequest(http.MethodGet, path, nil)
			require.NoError(t, err)
			rsp, err := srv.Client().Do(request)
			require.NoError(t, err)
			assert.Equal(t, tt.expectedCode, rsp.StatusCode)
			assert.Equal(t, tt.expectedBody, bodyContent(rsp))
		})
	}
}

func Test_UpdateCertManagerRequest(t *testing.T) {
	tests := map[string]struct {
		manager      rpaas.RpaasManager
		requestBody  string
		expectedCode int
		expectedBody string
	}{
		"doing a correct request": {
			requestBody: `{"issuer": "my-issuer", "dnsNames": ["foo.example.com", "bar.example.com"], "ipAddresses": ["169.196.100.1"]}`,
			manager: &fake.RpaasManager{
				FakeUpdateCertManagerRequest: func(instanceName string, in clientTypes.CertManager) error {
					assert.Equal(t, "my-instance", instanceName)
					assert.Equal(t, clientTypes.CertManager{
						Issuer:      "my-issuer",
						DNSNames:    []string{"foo.example.com", "bar.example.com"},
						IPAddresses: []string{"169.196.100.1"},
					}, in)
					return nil
				},
			},
			expectedCode: http.StatusOK,
		},

		"when some error is returned": {
			manager: &fake.RpaasManager{
				FakeUpdateCertManagerRequest: func(instanceName string, in clientTypes.CertManager) error {
					return &rpaas.ValidationError{Msg: "some error"}
				},
			},
			expectedCode: http.StatusBadRequest,
			expectedBody: `{"Msg":"some error"}`,
		},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			srv := newTestingServer(t, tt.manager)
			defer srv.Close()
			path := fmt.Sprintf("%s/resources/my-instance/cert-manager", srv.URL)
			request, err := http.NewRequest(http.MethodPost, path, strings.NewReader(tt.requestBody))
			require.NoError(t, err)
			request.Header.Set("Content-Type", "application/json")
			rsp, err := srv.Client().Do(request)
			require.NoError(t, err)
			assert.Equal(t, tt.expectedCode, rsp.StatusCode)
			assert.Equal(t, tt.expectedBody, bodyContent(rsp))
		})
	}
}

func Test_DeleteCertManagerRequest(t *testing.T) {
	tests := map[string]struct {
		manager      rpaas.RpaasManager
		expectedCode int
		expectedBody string
	}{
		"doing a correct request": {
			manager: &fake.RpaasManager{
				FakeDeleteCertManagerRequest: func(instanceName string) error {
					assert.Equal(t, "my-instance", instanceName)
					return nil
				},
			},
			expectedCode: http.StatusOK,
		},

		"when some error is returned": {
			manager: &fake.RpaasManager{
				FakeDeleteCertManagerRequest: func(instanceName string) error {
					return &rpaas.ValidationError{Msg: "some error"}
				},
			},
			expectedCode: http.StatusBadRequest,
			expectedBody: `{"Msg":"some error"}`,
		},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			srv := newTestingServer(t, tt.manager)
			defer srv.Close()
			path := fmt.Sprintf("%s/resources/my-instance/cert-manager", srv.URL)
			request, err := http.NewRequest(http.MethodDelete, path, nil)
			require.NoError(t, err)
			rsp, err := srv.Client().Do(request)
			require.NoError(t, err)
			assert.Equal(t, tt.expectedCode, rsp.StatusCode)
			assert.Equal(t, tt.expectedBody, bodyContent(rsp))
		})
	}
}
