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
	"github.com/tsuru/rpaas-operator/rpaas"
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

	makeBodyRequest := func(cert, key string) string {
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
		w.Close()
		return b.String()
	}

	testCases := []struct {
		requestBody   string
		expectedCode  int
		expectedBody  string
		expectedError error
		setup         func(*testing.T)
	}{
		{
			makeBodyRequest("some certificate", ""),
			400,
			"key file is either not provided or not valid",
			nil,
			nil,
		},
		{
			makeBodyRequest("", "some private key"),
			400,
			"cert file is either not provided or not valid",
			nil,
			nil,
		},
		{
			makeBodyRequest(certPem, keyPem),
			200,
			"",
			nil,
			func(t *testing.T) {
				manager := &fake.RpaasManager{
					FakeUpdateCertificate: func(name string, c *tls.Certificate) error {
						assert.Equal(t, name, instanceName)
						assert.Equal(t, c, &certificate)
						return nil
					},
				}
				rpaas.SetRpaasManager(manager)
			},
		},
		{
			makeBodyRequest(certPem, keyPem),
			500,
			`{"message":"Internal Server Error"}
`,
			errors.New("invalid manager state"),
			func(t *testing.T) {
				rpaas.SetRpaasManager(nil)
			},
		},
		{
			makeBodyRequest(certPem, keyPem),
			500,
			`{"message":"Internal Server Error"}
`,
			errors.New("some error"),
			func(t *testing.T) {
				manager := &fake.RpaasManager{
					FakeUpdateCertificate: func(name string, c *tls.Certificate) error {
						assert.Equal(t, name, instanceName)
						assert.Equal(t, c, &certificate)
						return errors.New("some error")
					},
				}
				rpaas.SetRpaasManager(manager)
			},
		},
	}

	for _, testCase := range testCases {
		t.Run("", func(t *testing.T) {
			if testCase.setup != nil {
				testCase.setup(t)
			}
			request := httptest.NewRequest(http.MethodPost, path, strings.NewReader(testCase.requestBody))
			request.Header.Set(echo.HeaderContentType, fmt.Sprintf(`%s; boundary=%s`, echo.MIMEMultipartForm, boundary))
			recorder := httptest.NewRecorder()
			e := echo.New()
			context := e.NewContext(request, recorder)
			context.SetParamNames("instance")
			context.SetParamValues(instanceName)
			err := updateCertificate(context)
			assert.Equal(t, testCase.expectedError, err)
			e.HTTPErrorHandler(err, context)
			assert.Equal(t, testCase.expectedCode, recorder.Code)
			assert.Equal(t, testCase.expectedBody, recorder.Body.String())
		})
	}

	rpaas.SetRpaasManager(nil)
}
