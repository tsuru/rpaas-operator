package api

import (
	"bytes"
	"crypto/tls"
	"errors"
	"fmt"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/labstack/echo"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tsuru/rpaas-operator/config"
	"github.com/tsuru/rpaas-operator/rpaas/fake"
)

func Test_updateCertificate(t *testing.T) {
	instanceName := "my-instance-name"
	path := fmt.Sprintf("/resources/%s/certificate", instanceName)
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
		requestBody   string
		expectedCode  int
		expectedBody  string
		expectedError error
		setup         func(*testing.T, echo.Context)
	}{
		{
			makeBodyRequest("some certificate", "", ""),
			400,
			"key file is either not provided or not valid",
			nil,
			nil,
		},
		{
			makeBodyRequest("", "some private key", ""),
			400,
			"cert file is either not provided or not valid",
			nil,
			nil,
		},
		{
			makeBodyRequest(certPem, keyPem, ""),
			200,
			"",
			nil,
			func(t *testing.T, c echo.Context) {
				manager := &fake.RpaasManager{
					FakeUpdateCertificate: func(instance, name string, c tls.Certificate) error {
						assert.Equal(t, "", name)
						assert.Equal(t, instance, instanceName)
						assert.Equal(t, c, certificate)
						return nil
					},
				}
				setManager(c, manager)
			},
		},
		{
			makeBodyRequest(certPem, keyPem, "mycert"),
			200,
			"",
			nil,
			func(t *testing.T, c echo.Context) {
				manager := &fake.RpaasManager{
					FakeUpdateCertificate: func(instance, name string, c tls.Certificate) error {
						assert.Equal(t, "mycert", name)
						assert.Equal(t, instance, instanceName)
						assert.Equal(t, c, certificate)
						return nil
					},
				}
				setManager(c, manager)
			},
		},
		{
			makeBodyRequest(certPem, keyPem, ""),
			500,
			`{"message":"Internal Server Error"}
`,
			errors.New("invalid manager state: <nil>"),
			func(t *testing.T, c echo.Context) {
				c.Set("manager", nil)
			},
		},
		{
			makeBodyRequest(certPem, keyPem, ""),
			500,
			`{"message":"Internal Server Error"}
`,
			errors.New("some error"),
			func(t *testing.T, c echo.Context) {
				manager := &fake.RpaasManager{
					FakeUpdateCertificate: func(instance, name string, c tls.Certificate) error {
						assert.Equal(t, "", name)
						assert.Equal(t, instance, instanceName)
						assert.Equal(t, c, certificate)
						return errors.New("some error")
					},
				}
				setManager(c, manager)
			},
		},
	}

	for _, testCase := range testCases {
		t.Run("", func(t *testing.T) {
			request := httptest.NewRequest(http.MethodPost, path, strings.NewReader(testCase.requestBody))
			request.Header.Set(echo.HeaderContentType, fmt.Sprintf(`%s; boundary=%s`, echo.MIMEMultipartForm, boundary))
			recorder := httptest.NewRecorder()
			e := echo.New()
			context := e.NewContext(request, recorder)
			if testCase.setup != nil {
				testCase.setup(t, context)
			}
			context.SetParamNames("instance")
			context.SetParamValues(instanceName)
			err := updateCertificate(context)
			assert.Equal(t, testCase.expectedError, err)
			e.HTTPErrorHandler(err, context)
			assert.Equal(t, testCase.expectedCode, recorder.Code)
			assert.Equal(t, testCase.expectedBody, recorder.Body.String())
		})
	}
}

func Test_healthcheck(t *testing.T) {
	testCases := []struct {
		name  string
		setup func(*testing.T)
	}{
		{
			name: "without auth",
		},
		{
			name: "with auth",
			setup: func(t *testing.T) {
				config.Set("API_USERNAME", "u1")
				config.Set("API_PASSWORD", "p1")
			},
		},
	}

	for _, tt := range testCases {
		t.Run(tt.name, func(t *testing.T) {
			if tt.setup != nil {
				tt.setup(t)
			}
			defer config.Unset("API_USERNAME")
			defer config.Unset("API_PASSWORD")
			webApi, err := New(nil)
			require.NoError(t, err)
			srv := httptest.NewServer(webApi.Handler())
			defer srv.Close()
			path := fmt.Sprintf("%s/healthcheck", srv.URL)
			request, err := http.NewRequest(http.MethodGet, path, nil)
			require.NoError(t, err)
			rsp, err := srv.Client().Do(request)
			require.NoError(t, err)
			assert.Equal(t, 200, rsp.StatusCode)
			assert.Regexp(t, "OK", bodyContent(rsp))
		})
	}
}

func Test_MiddlewareBasicAuth(t *testing.T) {
	testCases := []struct {
		name         string
		setup        func(*testing.T, *http.Request)
		expectedCode int
	}{
		{
			name:         "without auth",
			expectedCode: 404,
		},
		{
			name: "with auth enabled",
			setup: func(t *testing.T, r *http.Request) {
				config.Set("API_USERNAME", "u1")
				config.Set("API_PASSWORD", "p1")
			},
			expectedCode: 401,
		},
		{
			name: "with auth enabled and credentials",
			setup: func(t *testing.T, r *http.Request) {
				config.Set("API_USERNAME", "u1")
				config.Set("API_PASSWORD", "p1")
				r.SetBasicAuth("u1", "p1")
			},
			expectedCode: 404,
		},
		{
			name: "with auth enabled and invalid username",
			setup: func(t *testing.T, r *http.Request) {
				config.Set("API_USERNAME", "u1")
				config.Set("API_PASSWORD", "p1")
				r.SetBasicAuth("u9", "p1")
			},
			expectedCode: 401,
		},
		{
			name: "with auth enabled and invalid password",
			setup: func(t *testing.T, r *http.Request) {
				config.Set("API_USERNAME", "u1")
				config.Set("API_PASSWORD", "p1")
				r.SetBasicAuth("u1", "p9")
			},
			expectedCode: 401,
		},
	}

	for _, tt := range testCases {
		t.Run(tt.name, func(t *testing.T) {
			defer config.Unset("API_USERNAME")
			defer config.Unset("API_PASSWORD")
			webApi, err := New(nil)
			require.NoError(t, err)
			srv := httptest.NewServer(webApi.Handler())
			defer srv.Close()
			path := fmt.Sprintf("%s/", srv.URL)
			request, err := http.NewRequest(http.MethodGet, path, nil)
			require.NoError(t, err)
			if tt.setup != nil {
				tt.setup(t, request)
			}
			rsp, err := srv.Client().Do(request)
			require.NoError(t, err)
			assert.Equal(t, tt.expectedCode, rsp.StatusCode)
		})
	}
}
