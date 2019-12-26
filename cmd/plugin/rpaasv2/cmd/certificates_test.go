// Copyright 2019 tsuru authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package cmd

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	rpaasclient "github.com/tsuru/rpaas-operator/pkg/rpaas/client"
	"github.com/tsuru/rpaas-operator/pkg/rpaas/client/fake"
)

func TestUpdateCertificate(t *testing.T) {
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

	certFile, err := ioutil.TempFile("", "cert.*.pem")
	require.NoError(t, err)
	_, err = certFile.Write([]byte(certPem))
	require.NoError(t, err)
	require.NoError(t, certFile.Close())
	defer os.Remove(certFile.Name())

	keyFile, err := ioutil.TempFile("", "key.*.pem")
	require.NoError(t, err)
	_, err = keyFile.Write([]byte(keyPem))
	require.NoError(t, err)
	require.NoError(t, keyFile.Close())
	defer os.Remove(keyFile.Name())

	tests := []struct {
		name          string
		args          []string
		expected      string
		expectedError string
		client        rpaasclient.Client
	}{
		{
			name:          "when UpdateCertificate returns an error",
			args:          []string{"./rpaasv2", "certificates", "update", "-i", "my-instance", "--name", "my-instance.example.com", "--cert", certFile.Name(), "--key", keyFile.Name()},
			expectedError: "some error",
			client: &fake.FakeClient{
				FakeUpdateCertificate: func(args rpaasclient.UpdateCertificateArgs) (*http.Response, error) {
					expected := rpaasclient.UpdateCertificateArgs{
						Instance:    "my-instance",
						Name:        "my-instance.example.com",
						Certificate: certPem,
						Key:         keyPem,
					}
					assert.Equal(t, expected, args)
					return nil, fmt.Errorf("some error")
				},
			},
		},
		{
			name: "when UpdateCertificate returns no error",
			args: []string{"./rpaasv2", "certificates", "update", "-i", "my-instance", "--name", "my-instance.example.com", "--cert", certFile.Name(), "--key", keyFile.Name()},
			client: &fake.FakeClient{
				FakeUpdateCertificate: func(args rpaasclient.UpdateCertificateArgs) (*http.Response, error) {
					expected := rpaasclient.UpdateCertificateArgs{
						Instance:    "my-instance",
						Name:        "my-instance.example.com",
						Certificate: certPem,
						Key:         keyPem,
					}
					assert.Equal(t, expected, args)
					return nil, nil
				},
			},
			expected: "certificate \"my-instance.example.com\" updated in my-instance\n",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			stdout := &bytes.Buffer{}
			stderr := &bytes.Buffer{}
			app := newTestApp(stdout, stderr, tt.client)
			err := app.Run(tt.args)
			if tt.expectedError != "" {
				assert.Error(t, err)
				assert.EqualError(t, err, tt.expectedError)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.expected, stdout.String())
			assert.Empty(t, stderr.String())
		})
	}
}
