// Copyright 2019 tsuru authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package cmd

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	rpaasclient "github.com/tsuru/rpaas-operator/pkg/rpaas/client"
	"github.com/tsuru/rpaas-operator/pkg/rpaas/client/fake"
	"github.com/tsuru/rpaas-operator/pkg/rpaas/client/types"
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
				FakeUpdateCertificate: func(args rpaasclient.UpdateCertificateArgs) error {
					expected := rpaasclient.UpdateCertificateArgs{
						Instance:    "my-instance",
						Name:        "my-instance.example.com",
						Certificate: certPem,
						Key:         keyPem,
					}
					assert.Equal(t, expected, args)
					return fmt.Errorf("some error")
				},
			},
		},
		{
			name: "when UpdateCertificate returns no error",
			args: []string{"./rpaasv2", "certificates", "update", "-i", "my-instance", "--name", "my-instance.example.com", "--cert", certFile.Name(), "--key", keyFile.Name()},
			client: &fake.FakeClient{
				FakeUpdateCertificate: func(args rpaasclient.UpdateCertificateArgs) error {
					expected := rpaasclient.UpdateCertificateArgs{
						Instance:    "my-instance",
						Name:        "my-instance.example.com",
						Certificate: certPem,
						Key:         keyPem,
					}
					assert.Equal(t, expected, args)
					return nil
				},
			},
			expected: "certificate \"my-instance.example.com\" updated in my-instance\n",
		},

		{
			name: "enabling cert-manager integration",
			args: []string{"./rpaasv2", "certificates", "add", "-i", "my-instance", "--cert-manager", "--issuer", "lets-encrypt", "--dns", "my-instance.example.com", "--dns", "foo.example.com", "--ip", "169.196.100.100", "--ip", "2001:db8:dead:beef::"},
			client: &fake.FakeClient{
				FakeUpdateCertManager: func(args rpaasclient.UpdateCertManagerArgs) error {
					assert.Equal(t, rpaasclient.UpdateCertManagerArgs{
						Instance: "my-instance",
						CertManager: types.CertManager{
							Issuer:      "lets-encrypt",
							DNSNames:    []string{"my-instance.example.com", "foo.example.com"},
							IPAddresses: []string{"169.196.100.100", "2001:db8:dead:beef::"},
						},
					}, args)
					return nil
				},
			},
			expected: "cert manager certificate was updated\n",
		},

		{
			name: "passing DNS names without cert manager flag",
			args: []string{"./rpaasv2", "certificates", "add", "-i", "my-instance", "--dns", "my-instance.example.com"},
			client: &fake.FakeClient{
				FakeUpdateCertManager: func(args rpaasclient.UpdateCertManagerArgs) error {
					require.FailNow(t, "should not invoke this method")
					return fmt.Errorf("some error")
				},
			},
			expectedError: "issuer, DNS names and IP addresses require --cert-manager=true",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			stdout := &bytes.Buffer{}
			stderr := &bytes.Buffer{}
			app := NewApp(stdout, stderr, tt.client)
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

func TestDeleteCertificate(t *testing.T) {
	tests := []struct {
		name          string
		args          []string
		expected      string
		expectedError string
		client        rpaasclient.Client
	}{
		{
			name:          "when Delete certificate returns an error",
			args:          []string{"./rpaasv2", "certificates", "delete", "-i", "my-instance", "--name", "my-instance.example.com"},
			expectedError: "some error",
			client: &fake.FakeClient{
				FakeDeleteCertificate: func(args rpaasclient.DeleteCertificateArgs) error {
					expected := rpaasclient.DeleteCertificateArgs{
						Instance: "my-instance",
						Name:     "my-instance.example.com",
					}
					assert.Equal(t, expected, args)
					return fmt.Errorf("some error")
				},
			},
		},
		{
			name: "when DeleteCertificate returns no error",
			args: []string{"./rpaasv2", "certificates", "delete", "-i", "my-instance", "--name", "my-instance.example.com"},
			client: &fake.FakeClient{
				FakeDeleteCertificate: func(args rpaasclient.DeleteCertificateArgs) error {
					expected := rpaasclient.DeleteCertificateArgs{
						Instance: "my-instance",
						Name:     "my-instance.example.com",
					}
					assert.Equal(t, expected, args)
					return nil
				},
			},
			expected: "certificate \"my-instance.example.com\" successfully deleted on my-instance\n",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			stdout := &bytes.Buffer{}
			stderr := &bytes.Buffer{}
			app := NewApp(stdout, stderr, tt.client)
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
