// Copyright 2020 tsuru authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package web

import (
	"bytes"
	"crypto/tls"
	"errors"
	"fmt"
	"mime/multipart"
	"net/http"
	"net/url"
	"strings"
	"testing"

	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/tsuru/rpaas-operator/internal/pkg/rpaas"
	"github.com/tsuru/rpaas-operator/internal/pkg/rpaas/fake"
	clientTypes "github.com/tsuru/rpaas-operator/pkg/rpaas/client/types"
)

func Test_updateCertificate(t *testing.T) {
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

	testCases := map[string]struct {
		name         string
		certificate  string
		key          string
		expectedCode int
		expectedBody string
		manager      rpaas.RpaasManager
	}{
		"when no private key is sent": {
			certificate:  "some certificate",
			expectedCode: http.StatusBadRequest,
			expectedBody: `{"message":"cannot read the key from request"}`,
		},

		"when no certificate is sent": {
			key:          "some private key",
			expectedCode: http.StatusBadRequest,
			expectedBody: `{"message":"cannot read the certificate from request"}`,
		},

		"when successfully adding a default certificate": {
			certificate:  certPem,
			key:          keyPem,
			expectedCode: http.StatusOK,
			manager: &fake.RpaasManager{
				FakeUpdateCertificate: func(instance, name string, c tls.Certificate) error {
					assert.Equal(t, "", name)
					assert.Equal(t, "my-instance", instance)
					assert.Equal(t, c, certificate)
					return nil
				},
			},
		},

		"when successfully adding a certificate with custom name": {
			name:         "mycert",
			certificate:  certPem,
			key:          keyPem,
			expectedCode: http.StatusOK,
			manager: &fake.RpaasManager{
				FakeUpdateCertificate: func(instance, name string, c tls.Certificate) error {
					assert.Equal(t, "mycert", name)
					assert.Equal(t, "my-instance", instance)
					assert.Equal(t, c, certificate)
					return nil
				},
			},
		},

		"when cannot update the certificate due to an error": {
			certificate:  certPem,
			key:          keyPem,
			expectedCode: http.StatusInternalServerError,
			expectedBody: `{"message":"some error"}`,
			manager: &fake.RpaasManager{
				FakeUpdateCertificate: func(instance, name string, c tls.Certificate) error {
					return errors.New("some error")
				},
			},
		},
	}

	for name, tt := range testCases {
		t.Run(name, func(t *testing.T) {
			srv := newTestingServer(t, tt.manager)
			defer srv.Close()
			path := fmt.Sprintf("%s/resources/%s/certificate", srv.URL, "my-instance")

			t.Run("Content-Type: multipart/form-data", func(t *testing.T) {
				body, boundary := makeMultipartFormForCertificate(t, tt.certificate, tt.key, tt.name)
				r, err := http.NewRequest(http.MethodPost, path, strings.NewReader(body))
				require.NoError(t, err)
				r.Header.Set(echo.HeaderContentType, fmt.Sprintf(`%s; boundary=%s`, echo.MIMEMultipartForm, boundary))
				rsp, err := srv.Client().Do(r)
				require.NoError(t, err)
				assert.Equal(t, tt.expectedCode, rsp.StatusCode)
				assert.Equal(t, tt.expectedBody, bodyContent(rsp))
			})

			t.Run("Content-Type: application/x-www-form-urlencoded", func(t *testing.T) {
				body := makeFormBodyForCertificate(tt.certificate, tt.key, tt.name)
				r, err := http.NewRequest(http.MethodPost, path, strings.NewReader(body))
				require.NoError(t, err)
				r.Header.Set(echo.HeaderContentType, echo.MIMEApplicationForm)
				rsp, err := srv.Client().Do(r)
				require.NoError(t, err)
				assert.Equal(t, tt.expectedCode, rsp.StatusCode)
				assert.Equal(t, tt.expectedBody, bodyContent(rsp))
			})
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
			expectedBody: `{"message":""}`,
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
			expectedBody: `{"message":"no certificate bound to instance \"real-instance\""}`,
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

func Test_GetCertManagerRequests(t *testing.T) {
	tests := map[string]struct {
		manager      rpaas.RpaasManager
		expectedCode int
		expectedBody string
	}{
		"instance with two requests": {
			manager: &fake.RpaasManager{
				FakeGetCertManagerRequests: func(instanceName string) ([]clientTypes.CertManager, error) {
					assert.Equal(t, "my-instance", instanceName)
					return []clientTypes.CertManager{
						{
							Issuer:      "my-issuer",
							DNSNames:    []string{"www.my-instance.example.com", "my-instance.example.com"},
							IPAddresses: []string{"169.196.254.100"},
						},
						{
							Issuer:   "lets-encrypt",
							DNSNames: []string{"*.example.com"},
						},
					}, nil
				},
			},
			expectedCode: http.StatusOK,
			expectedBody: `[{"issuer":"my-issuer","dnsNames":["www.my-instance.example.com","my-instance.example.com"],"ipAddresses":["169.196.254.100"]},{"issuer":"lets-encrypt","dnsNames":["*.example.com"]}]`,
		},

		"when some error is returned": {
			manager: &fake.RpaasManager{
				FakeGetCertManagerRequests: func(instanceName string) ([]clientTypes.CertManager, error) {
					return nil, &rpaas.ValidationError{Msg: "some error"}
				},
			},
			expectedCode: http.StatusBadRequest,
			expectedBody: `{"message":"some error"}`,
		},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			srv := newTestingServer(t, tt.manager)
			defer srv.Close()
			path := fmt.Sprintf("%s/resources/my-instance/cert-manager", srv.URL)
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

		"doing a correct request with name": {
			requestBody: `{"name": "cert01", "issuer": "my-issuer", "dnsNames": ["foo.example.com"]}`,
			manager: &fake.RpaasManager{
				FakeUpdateCertManagerRequest: func(instanceName string, in clientTypes.CertManager) error {
					assert.Equal(t, "my-instance", instanceName)
					assert.Equal(t, clientTypes.CertManager{
						Name:     "cert01",
						Issuer:   "my-issuer",
						DNSNames: []string{"foo.example.com"},
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
			expectedBody: `{"message":"some error"}`,
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

func Test_DeleteCertManagerRequestByIssuer(t *testing.T) {
	tests := map[string]struct {
		manager      rpaas.RpaasManager
		instance     string
		issuer       string
		expectedCode int
		expectedBody string
	}{
		"remove Cert Manager request without issuer": {
			instance: "my-instance",
			manager: &fake.RpaasManager{
				FakeDeleteCertManagerRequestByIssuer: func(instanceName, issuer string) error {
					assert.Equal(t, "my-instance", instanceName)
					assert.Empty(t, issuer)
					return nil
				},
			},
			expectedCode: http.StatusOK,
		},

		"remove Cert Manager request from a specific issuer": {
			instance: "my-instance",
			issuer:   "my-cert-issuer",
			manager: &fake.RpaasManager{
				FakeDeleteCertManagerRequestByIssuer: func(instanceName, issuer string) error {
					assert.Equal(t, "my-instance", instanceName)
					assert.Equal(t, "my-cert-issuer", issuer)
					return nil
				},
			},
			expectedCode: http.StatusOK,
		},

		"when some error is returned": {
			manager: &fake.RpaasManager{
				FakeDeleteCertManagerRequestByIssuer: func(instanceName, issuer string) error {
					return &rpaas.ValidationError{Msg: "some error"}
				},
			},
			expectedCode: http.StatusBadRequest,
			expectedBody: `{"message":"some error"}`,
		},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			srv := newTestingServer(t, tt.manager)
			defer srv.Close()
			path := fmt.Sprintf("%s/resources/%s/cert-manager?issuer=%s", srv.URL, tt.instance, tt.issuer)
			request, err := http.NewRequest(http.MethodDelete, path, nil)
			require.NoError(t, err)
			rsp, err := srv.Client().Do(request)
			require.NoError(t, err)
			assert.Equal(t, tt.expectedCode, rsp.StatusCode)
			assert.Equal(t, tt.expectedBody, bodyContent(rsp))
		})
	}
}

func Test_DeleteCertManagerRequestByName(t *testing.T) {
	tests := map[string]struct {
		manager      rpaas.RpaasManager
		instance     string
		name         string
		expectedCode int
		expectedBody string
	}{

		"remove Cert Manager request from a specific issuer": {
			instance: "my-instance",
			name:     "cert01",
			manager: &fake.RpaasManager{
				FakeDeleteCertManagerRequestByName: func(instanceName, name string) error {
					assert.Equal(t, "my-instance", instanceName)
					assert.Equal(t, "cert01", name)
					return nil
				},
			},
			expectedCode: http.StatusOK,
		},

		"when some error is returned": {
			name: "cert02",
			manager: &fake.RpaasManager{
				FakeDeleteCertManagerRequestByName: func(instanceName, issuer string) error {
					return &rpaas.ValidationError{Msg: "some error"}
				},
			},
			expectedCode: http.StatusBadRequest,
			expectedBody: `{"message":"some error"}`,
		},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			srv := newTestingServer(t, tt.manager)
			defer srv.Close()
			path := fmt.Sprintf("%s/resources/%s/cert-manager?name=%s", srv.URL, tt.instance, tt.name)
			request, err := http.NewRequest(http.MethodDelete, path, nil)
			require.NoError(t, err)
			rsp, err := srv.Client().Do(request)
			require.NoError(t, err)
			assert.Equal(t, tt.expectedBody, bodyContent(rsp))
			assert.Equal(t, tt.expectedCode, rsp.StatusCode)
		})
	}
}

func makeMultipartFormForCertificate(t *testing.T, cert, key, name string) (string, string) {
	b := &bytes.Buffer{}
	w := multipart.NewWriter(b)
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
	return b.String(), w.Boundary()
}

func makeFormBodyForCertificate(cert, key, name string) string {
	u := make(url.Values)
	u.Set("cert", cert)
	u.Set("key", key)
	u.Set("name", name)
	return u.Encode()
}
