// Copyright 2021 tsuru authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package certificates_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	nginxv1alpha1 "github.com/tsuru/nginx-operator/api/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	k8sruntime "k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	"github.com/tsuru/rpaas-operator/api/v1alpha1"
	"github.com/tsuru/rpaas-operator/internal/controllers/certificates"
	"github.com/tsuru/rpaas-operator/pkg/runtime"
	"github.com/tsuru/rpaas-operator/pkg/util"
)

func k8sResources() []k8sruntime.Object {
	return []k8sruntime.Object{
		&v1alpha1.RpaasInstance{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "my-instance-1",
				Namespace: "rpaasv2",
			},
		},

		&v1alpha1.RpaasInstance{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "my-instance-2",
				Namespace: "rpaasv2",
			},
			Spec: v1alpha1.RpaasInstanceSpec{
				PodTemplate: nginxv1alpha1.NginxPodTemplateSpec{
					Annotations: map[string]string{
						"rpaas.extensions.tsuru.io/www.example.com-certificate-sha256": "a08e4b2caf275287f20bc92d163362f2390b58ee0713cb611da3d0983bc10db4",
						"rpaas.extensions.tsuru.io/www.example.com-key-sha256":         "c8d681dd3ea46337426b74ee3da264c9de1d417ef2c8daef102e9ca257476bea",
					},
				},

				TLS: []nginxv1alpha1.NginxTLS{{
					SecretName: "my-instance-2-certs-abc123",
					Hosts:      []string{"www.example.com"},
				}},
			},
		},

		&corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "my-instance-2-certs-abc123",
				Namespace: "rpaasv2",
				Labels: map[string]string{
					"rpaas.extensions.tsuru.io/certificate-name": "www.example.com",
					"rpaas.extensions.tsuru.io/instance-name":    "my-instance-2",
				},
			},
			Type: corev1.SecretTypeTLS,
			StringData: map[string]string{
				// Generated with:
				//   go run $(go env GOROOT)/src/crypto/tls/generate_cert.go -duration 8760h -host www.example.com -ecdsa-curve P224
				"tls.crt": `-----BEGIN CERTIFICATE-----
MIIBXzCCAQ6gAwIBAgIQCcNGyPGPZIG5Ot7eTcSpxTAKBggqhkjOPQQDAjASMRAw
DgYDVQQKEwdBY21lIENvMB4XDTIxMDgzMTE1NDEwNVoXDTIyMDgzMTE1NDEwNVow
EjEQMA4GA1UEChMHQWNtZSBDbzBOMBAGByqGSM49AgEGBSuBBAAhAzoABP+p5p81
Pl9QRjU0uL5fKzsiGw+GUkSDSR8P99rcJpAgvENF8dR2N8vHPXXhIhy/Disv2H+K
28mPo1EwTzAOBgNVHQ8BAf8EBAMCB4AwEwYDVR0lBAwwCgYIKwYBBQUHAwEwDAYD
VR0TAQH/BAIwADAaBgNVHREEEzARgg93d3cuZXhhbXBsZS5jb20wCgYIKoZIzj0E
AwIDPwAwPAIcZ1OoRZgyUn+M4lMTP2p9jgYhp8aMd/BZLxiKAQIcF7AdfXbn95Xp
DKfkqhV+7ygzNlcJyb5yYUfkgQ==
-----END CERTIFICATE-----`,
				"tls.key": `-----BEGIN PRIVATE KEY-----
MHgCAQAwEAYHKoZIzj0CAQYFK4EEACEEYTBfAgEBBBwiSUVHPwwhedjHltdoJi/M
BgepE84duC/2eeqqoTwDOgAE/6nmnzU+X1BGNTS4vl8rOyIbD4ZSRINJHw/32twm
kCC8Q0Xx1HY3y8c9deEiHL8OKy/Yf4rbyY8=
-----END PRIVATE KEY-----`,
			},
		},

		&v1alpha1.RpaasInstance{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "my-instance-3",
				Namespace: "rpaasv2",
			},
			Spec: v1alpha1.RpaasInstanceSpec{},
		},

		&corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "my-instance-3-cert-manager",
				Namespace: "rpaasv2",
			},
			Type: corev1.SecretTypeTLS,
			Data: map[string][]byte{
				// Generated with:
				//   go run $(go env GOROOT)/src/crypto/tls/generate_cert.go -duration 8760h -host *.example.com,*.example.org -ecdsa-curve P224
				"tls.crt": []byte(`-----BEGIN CERTIFICATE-----
MIIBbjCCARugAwIBAgIQWRF65AU1tWeZczAq3aqDjzAKBggqhkjOPQQDAjASMRAw
DgYDVQQKEwdBY21lIENvMB4XDTIxMDkwMzIxMTcyNFoXDTIyMDkwMzIxMTcyNFow
EjEQMA4GA1UEChMHQWNtZSBDbzBOMBAGByqGSM49AgEGBSuBBAAhAzoABN836A1F
Hku1le2ZgtgNZFsRr8ylOpzhQT5k8XQk9vlOSRqv40O0ku/AZoY9bsR9dmH6jDXR
CasIo14wXDAOBgNVHQ8BAf8EBAMCB4AwEwYDVR0lBAwwCgYIKwYBBQUHAwEwDAYD
VR0TAQH/BAIwADAnBgNVHREEIDAegg0qLmV4YW1wbGUuY29tgg0qLmV4YW1wbGUu
b3JnMAoGCCqGSM49BAMCA0EAMD4CHQDoVyjfDdtJ523kkNZA8ryt+H2ztUI8Vr8N
nneDAh0AqwHZyubwD3GE2eOTyC7DbmG9PbQA9g1Zeq8BVw==
-----END CERTIFICATE-----`),
				"tls.key": []byte(`-----BEGIN PRIVATE KEY-----
MHgCAQAwEAYHKoZIzj0CAQYFK4EEACEEYTBfAgEBBBwjAF6qPoiGd7BCj28Cg//t
cHPInBo/AIYDW+WuoTwDOgAE3zfoDUUeS7WV7ZmC2A1kWxGvzKU6nOFBPmTxdCT2
+U5JGq/jQ7SS78Bmhj1uxH12YfqMNdEJqwg=
-----END PRIVATE KEY-----`),
			},
		},
	}
}

func TestUpdateCertificate(t *testing.T) {
	tests := map[string]struct {
		instance        string
		certificateName string
		certificate     string
		certificateKey  string
		expectedError   string
		assert          func(t *testing.T, c client.Client)
	}{
		"first certificate in the instance, certificate with three SANs (www.example.com, www.example.org, www.example.test)": {
			instance:        "my-instance-1",
			certificateName: "www.example.com",
			// Generated with:
			//  go run $(go env GOROOT)/src/crypto/tls/generate_cert.go -duration 8760h -host www.example.com,www.example.org,www.example.test -rsa-bits 512
			certificate: `-----BEGIN CERTIFICATE-----
MIIBmDCCAUKgAwIBAgIQRr737j1vwFND83io4IliEzANBgkqhkiG9w0BAQsFADAS
MRAwDgYDVQQKEwdBY21lIENvMB4XDTIxMDgzMDE4MTgxNVoXDTIyMDgzMDE4MTgx
NVowEjEQMA4GA1UEChMHQWNtZSBDbzBcMA0GCSqGSIb3DQEBAQUAA0sAMEgCQQC+
0Zlmis2JigXdmCRKF+sZqBuVSPbpBsy4cP7eUBkcyxRir3jwPNoahd6Qv57Tr1vO
ZAj+hb5Rf75T7NgRzrQVAgMBAAGjdDByMA4GA1UdDwEB/wQEAwIFoDATBgNVHSUE
DDAKBggrBgEFBQcDATAMBgNVHRMBAf8EAjAAMD0GA1UdEQQ2MDSCD3d3dy5leGFt
cGxlLmNvbYIPd3d3LmV4YW1wbGUub3JnghB3d3cuZXhhbXBsZS50ZXN0MA0GCSqG
SIb3DQEBCwUAA0EAc/GgmuRfov3QD+RAXcHYQKvmG9WxBRvOK8ALB+l4ibak0rS2
RBUhFyKxlZEjXu5Fhv9PgYBzEA2AcWtiM7j8lA==
-----END CERTIFICATE-----`,
			certificateKey: `-----BEGIN PRIVATE KEY-----
MIIBVAIBADANBgkqhkiG9w0BAQEFAASCAT4wggE6AgEAAkEAvtGZZorNiYoF3Zgk
ShfrGagblUj26QbMuHD+3lAZHMsUYq948DzaGoXekL+e069bzmQI/oW+UX++U+zY
Ec60FQIDAQABAkB1W83f/lBpXgU7g54WH93NetH0H9sT+MWiToTCUDsRtFkOFpJf
ayKQpriEtcJjW1s/BIW5ldYYi4uJo9rHm+MFAiEA1xwbuJUm+JrlkfrDPsV9fb3p
02hr3cOuC9rVFYPfBzsCIQDjF2pEt0vNmFZLE/EwpiGQ+HB5d8UxCn8cfKqEB52c
7wIgYzTacBGRzJwbfmzJORz52FELEu4YuUky7tK47VhJNtsCIQCRRhhoby3iD1Mc
4lwIOC7+87/YJOOUFNfuHF5k6g5NJwIgYYt7B4pbCW5092Z5M2lDPvujEAr7quDI
wg4cGbIbBPs=
-----END PRIVATE KEY-----`,
			assert: func(t *testing.T, c client.Client) {
				var sl corev1.SecretList
				err := c.List(context.TODO(), &sl, &client.ListOptions{
					LabelSelector: labels.Set{
						certificates.CertificateNameLabel:         "www.example.com",
						"rpaas.extensions.tsuru.io/instance-name": "my-instance-1",
					}.AsSelector(),
					Namespace: "rpaasv2",
				})
				require.NoError(t, err)
				require.Len(t, sl.Items, 1)

				s := sl.Items[0]
				assert.Equal(t, corev1.SecretTypeTLS, s.Type)
				assert.Equal(t, "my-instance-1-certs-", s.GenerateName)
				assert.Equal(t, "rpaasv2", s.Namespace)
				assert.Equal(t, map[string]string{
					"rpaas.extensions.tsuru.io/certificate-name": "www.example.com",
					"rpaas.extensions.tsuru.io/instance-name":    "my-instance-1",
				}, s.Labels)
				assert.Equal(t, "a0610da4d1958cfa7c375870e2c1bac796e84f509bbd989fa5a7c0e040965f28", util.SHA256(s.Data["tls.crt"]))
				assert.Equal(t, "e644183deec75208c5fc53b4afb98e471ee290c7e7e10c5b95caff6851346132", util.SHA256(s.Data["tls.key"]))

				var i v1alpha1.RpaasInstance
				err = c.Get(context.TODO(), types.NamespacedName{Name: "my-instance-1", Namespace: "rpaasv2"}, &i)
				require.NoError(t, err)

				assert.Equal(t, []nginxv1alpha1.NginxTLS{{
					SecretName: s.Name,
					Hosts:      []string{"www.example.com", "www.example.org", "www.example.test"},
				}}, i.Spec.TLS)
			},
		},

		"updating the \"www.example.com\" certificate": {
			instance:        "my-instance-2",
			certificateName: "www.example.com",
			// Generated with:
			//  go run $(go env GOROOT)/src/crypto/tls/generate_cert.go -duration 8760h -host www.example.com,www.example.org,www.example.test -rsa-bits 512
			certificate: `-----BEGIN CERTIFICATE-----
MIIBmDCCAUKgAwIBAgIQRr737j1vwFND83io4IliEzANBgkqhkiG9w0BAQsFADAS
MRAwDgYDVQQKEwdBY21lIENvMB4XDTIxMDgzMDE4MTgxNVoXDTIyMDgzMDE4MTgx
NVowEjEQMA4GA1UEChMHQWNtZSBDbzBcMA0GCSqGSIb3DQEBAQUAA0sAMEgCQQC+
0Zlmis2JigXdmCRKF+sZqBuVSPbpBsy4cP7eUBkcyxRir3jwPNoahd6Qv57Tr1vO
ZAj+hb5Rf75T7NgRzrQVAgMBAAGjdDByMA4GA1UdDwEB/wQEAwIFoDATBgNVHSUE
DDAKBggrBgEFBQcDATAMBgNVHRMBAf8EAjAAMD0GA1UdEQQ2MDSCD3d3dy5leGFt
cGxlLmNvbYIPd3d3LmV4YW1wbGUub3JnghB3d3cuZXhhbXBsZS50ZXN0MA0GCSqG
SIb3DQEBCwUAA0EAc/GgmuRfov3QD+RAXcHYQKvmG9WxBRvOK8ALB+l4ibak0rS2
RBUhFyKxlZEjXu5Fhv9PgYBzEA2AcWtiM7j8lA==
-----END CERTIFICATE-----`,
			certificateKey: `-----BEGIN PRIVATE KEY-----
MIIBVAIBADANBgkqhkiG9w0BAQEFAASCAT4wggE6AgEAAkEAvtGZZorNiYoF3Zgk
ShfrGagblUj26QbMuHD+3lAZHMsUYq948DzaGoXekL+e069bzmQI/oW+UX++U+zY
Ec60FQIDAQABAkB1W83f/lBpXgU7g54WH93NetH0H9sT+MWiToTCUDsRtFkOFpJf
ayKQpriEtcJjW1s/BIW5ldYYi4uJo9rHm+MFAiEA1xwbuJUm+JrlkfrDPsV9fb3p
02hr3cOuC9rVFYPfBzsCIQDjF2pEt0vNmFZLE/EwpiGQ+HB5d8UxCn8cfKqEB52c
7wIgYzTacBGRzJwbfmzJORz52FELEu4YuUky7tK47VhJNtsCIQCRRhhoby3iD1Mc
4lwIOC7+87/YJOOUFNfuHF5k6g5NJwIgYYt7B4pbCW5092Z5M2lDPvujEAr7quDI
wg4cGbIbBPs=
-----END PRIVATE KEY-----`,
			assert: func(t *testing.T, c client.Client) {
				var s corev1.Secret
				err := c.Get(context.TODO(), types.NamespacedName{Name: "my-instance-2-certs-abc123", Namespace: "rpaasv2"}, &s)
				require.NoError(t, err)

				assert.Equal(t, corev1.SecretTypeTLS, s.Type)
				assert.Equal(t, map[string]string{
					"rpaas.extensions.tsuru.io/certificate-name": "www.example.com",
					"rpaas.extensions.tsuru.io/instance-name":    "my-instance-2",
				}, s.Labels)
				assert.Equal(t, "a0610da4d1958cfa7c375870e2c1bac796e84f509bbd989fa5a7c0e040965f28", util.SHA256(s.Data["tls.crt"]))
				assert.Equal(t, "e644183deec75208c5fc53b4afb98e471ee290c7e7e10c5b95caff6851346132", util.SHA256(s.Data["tls.key"]))

				var i v1alpha1.RpaasInstance
				err = c.Get(context.TODO(), types.NamespacedName{Name: "my-instance-2", Namespace: "rpaasv2"}, &i)
				require.NoError(t, err)

				assert.Equal(t, []nginxv1alpha1.NginxTLS{{
					SecretName: s.Name,
					Hosts:      []string{"www.example.com", "www.example.org", "www.example.test"},
				}}, i.Spec.TLS)
			},
		},

		"adding a certificate in an instance with one certificate": {
			instance:        "my-instance-2",
			certificateName: "awesome-certificate",
			// Generated with:
			//  go run $(go env GOROOT)/src/crypto/tls/generate_cert.go -duration 8760h -host blog.example.com -ecdsa-curve P224
			certificate: `-----BEGIN CERTIFICATE-----
MIIBYDCCAQ+gAwIBAgIQOaQWls1CqA8ciV6cYr9swjAKBggqhkjOPQQDAjASMRAw
DgYDVQQKEwdBY21lIENvMB4XDTIxMDgzMTE4MDMxOVoXDTIyMDgzMTE4MDMxOVow
EjEQMA4GA1UEChMHQWNtZSBDbzBOMBAGByqGSM49AgEGBSuBBAAhAzoABHFZ1zJu
0x2/U1sYX15TL0MfSWzmXKKUlXwU9bt0DlwMbdpTSgHXxmGV+6tPVqW28aC5ONxS
Xxz4o1IwUDAOBgNVHQ8BAf8EBAMCB4AwEwYDVR0lBAwwCgYIKwYBBQUHAwEwDAYD
VR0TAQH/BAIwADAbBgNVHREEFDASghBibG9nLmV4YW1wbGUuY29tMAoGCCqGSM49
BAMCAz8AMDwCHGqk7iRuI9eW9zfMYIMHjGV91jLLGWYU3xdsG0wCHEX+4hlRsM2g
1F7296rcT8cY9dQgmGTuzAsAOMs=
-----END CERTIFICATE-----`,
			certificateKey: `-----BEGIN PRIVATE KEY-----
MHgCAQAwEAYHKoZIzj0CAQYFK4EEACEEYTBfAgEBBByQ+TRztjzo6ThtOha8AH+p
XHLzN4TUmsCHIywioTwDOgAEcVnXMm7THb9TWxhfXlMvQx9JbOZcopSVfBT1u3QO
XAxt2lNKAdfGYZX7q09WpbbxoLk43FJfHPg=
-----END PRIVATE KEY-----`,
			assert: func(t *testing.T, c client.Client) {
				var sl corev1.SecretList
				err := c.List(context.TODO(), &sl, &client.ListOptions{
					LabelSelector: labels.Set{
						certificates.CertificateNameLabel:         "awesome-certificate",
						"rpaas.extensions.tsuru.io/instance-name": "my-instance-2",
					}.AsSelector(),
					Namespace: "rpaasv2",
				})
				require.NoError(t, err)
				require.Len(t, sl.Items, 1)

				s := sl.Items[0]

				var i v1alpha1.RpaasInstance
				err = c.Get(context.TODO(), types.NamespacedName{Name: "my-instance-2", Namespace: "rpaasv2"}, &i)
				require.NoError(t, err)

				assert.Equal(t, []nginxv1alpha1.NginxTLS{
					{SecretName: "my-instance-2-certs-abc123", Hosts: []string{"www.example.com"}},
					{SecretName: s.Name, Hosts: []string{"blog.example.com"}},
				}, i.Spec.TLS)
			},
		},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			resources := k8sResources()

			client := fake.NewClientBuilder().
				WithScheme(runtime.NewScheme()).
				WithRuntimeObjects(resources...).
				Build()

			var instance *v1alpha1.RpaasInstance
			for _, object := range resources {
				if i, found := object.(*v1alpha1.RpaasInstance); found && i.Name == tt.instance {
					instance = i
					break
				}
			}

			require.NotNil(t, instance, "you should select a RpaasInstance from resources")

			err := certificates.UpdateCertificate(context.TODO(), client, instance, tt.certificateName, []byte(tt.certificate), []byte(tt.certificateKey))
			if tt.expectedError != "" {
				assert.EqualError(t, err, tt.expectedError)
				return
			}

			require.NoError(t, err)

			require.NotNil(t, tt.assert, "you must provide an assert function")
			tt.assert(t, client)
		})
	}
}

func Test_DeleteCertificate(t *testing.T) {
	tests := map[string]struct {
		instance        string
		certificateName string
		expectedError   string
		assert          func(t *testing.T, c client.Client)
	}{
		"when certificate name is not provided": {
			instance:      "my-instance-1",
			expectedError: "certificate name cannot be empty",
		},

		"when certificate does not exist": {
			instance:        "my-instance-1",
			certificateName: "not-found-certificates",
			expectedError:   "certificate \"not-found-certificates\" does not exist",
		},

		"removing a certificate successfully ": {
			instance:        "my-instance-2",
			certificateName: "www.example.com",
			assert: func(t *testing.T, c client.Client) {
				var sl corev1.SecretList
				err := c.List(context.TODO(), &sl, &client.ListOptions{
					LabelSelector: labels.Set{
						certificates.CertificateNameLabel:         "www.example.com",
						"rpaas.extensions.tsuru.io/instance-name": "my-instance-2",
					}.AsSelector(),
					Namespace: "rpaasv2",
				})
				require.NoError(t, err)
				assert.Len(t, sl.Items, 0)

				var i v1alpha1.RpaasInstance
				err = c.Get(context.TODO(), types.NamespacedName{Name: "my-instance-2", Namespace: "rpaasv2"}, &i)
				require.NoError(t, err)

				assert.Len(t, i.Spec.TLS, 0)
			},
		},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			resources := k8sResources()

			client := fake.NewClientBuilder().
				WithScheme(runtime.NewScheme()).
				WithRuntimeObjects(resources...).
				Build()

			var instance *v1alpha1.RpaasInstance
			for _, object := range resources {
				if i, found := object.(*v1alpha1.RpaasInstance); found && i.Name == tt.instance {
					instance = i
					break
				}
			}

			require.NotNil(t, instance, "you should select a RpaasInstance from resources")

			err := certificates.DeleteCertificate(context.TODO(), client, instance, tt.certificateName)
			if tt.expectedError != "" {
				assert.EqualError(t, err, tt.expectedError)
				return
			}

			require.NoError(t, err)

			require.NotNil(t, tt.assert, "you must provide an assert function")
			tt.assert(t, client)
		})
	}
}
