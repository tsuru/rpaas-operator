// Copyright 2021 tsuru authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package certificates

import (
	"context"
	"fmt"
	"testing"

	cmv1 "github.com/cert-manager/cert-manager/pkg/apis/certmanager/v1"
	cmmeta "github.com/cert-manager/cert-manager/pkg/apis/meta/v1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	nginxv1alpha1 "github.com/tsuru/nginx-operator/api/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	k8sruntime "k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	"github.com/tsuru/rpaas-operator/api/v1alpha1"
	"github.com/tsuru/rpaas-operator/pkg/runtime"
	"github.com/tsuru/rpaas-operator/pkg/util"
)

func Test_ReconcileCertManager(t *testing.T) {
	resources := []k8sruntime.Object{
		&cmv1.Issuer{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "issuer-1",
				Namespace: "rpaasv2",
			},
			Spec: cmv1.IssuerSpec{
				IssuerConfig: cmv1.IssuerConfig{
					SelfSigned: &cmv1.SelfSignedIssuer{},
				},
			},
		},

		&cmv1.ClusterIssuer{
			ObjectMeta: metav1.ObjectMeta{
				Name: "cluster-issuer-1",
			},
			Spec: cmv1.IssuerSpec{
				IssuerConfig: cmv1.IssuerConfig{
					SelfSigned: &cmv1.SelfSignedIssuer{},
				},
			},
		},

		&corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "my-instance-2-abc123",
				Namespace: "rpaasv2",
				Labels: map[string]string{
					"rpaas.extensions.tsuru.io/certificate-name": "cert-manager",
					"rpaas.extensions.tsuru.io/instance-name":    "my-instance-2",
				},
			},
			Data: map[string][]byte{
				"tls.crt": []byte(`--- some cert here ---`),
				"tls.key": []byte(`--- some key here ---`),
			},
		},

		&cmv1.Certificate{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "my-instance-2",
				Namespace: "rpaasv2",
				OwnerReferences: []metav1.OwnerReference{
					{
						APIVersion:         "extensions.tsuru.io/v1alpha1",
						Kind:               "RpaasInstance",
						Name:               "my-instance-2",
						Controller:         func(b bool) *bool { return &b }(true),
						BlockOwnerDeletion: func(b bool) *bool { return &b }(true),
					},
				},
			},
			Spec: cmv1.CertificateSpec{
				IssuerRef: cmmeta.ObjectReference{
					Name:  "issuer-1",
					Kind:  "Issuer",
					Group: "cert-manager.io",
				},
				SecretName: "my-instance-2-cert-manager",
				DNSNames:   []string{"my-instance-2.example.com"},
			},
		},

		&cmv1.Certificate{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "my-instance-3-cert-manager-issuer-1",
				Namespace: "rpaasv2",
			},
			Spec: cmv1.CertificateSpec{
				IssuerRef: cmmeta.ObjectReference{
					Name:  "issuer-1",
					Kind:  "Issuer",
					Group: "cert-manager.io",
				},
				SecretName: "my-instance-3-cert-manager",
				DNSNames:   []string{"my-instance-3.example.com"},
			},
			Status: cmv1.CertificateStatus{
				Conditions: []cmv1.CertificateCondition{
					{
						Type:   cmv1.CertificateConditionReady,
						Status: cmmeta.ConditionTrue,
					},
				},
			},
		},

		&corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "my-instance-3-cert-manager-issuer-1",
				Namespace: "rpaasv2",
				Labels: map[string]string{
					"rpaas.extensions.tsuru.io/certificate-name": "cert-manager-issuer-1",
					"rpaas.extensions.tsuru.io/instance-name":    "my-instance-3",
				},
			},
			Data: map[string][]byte{
				// Generated with:
				//  go run $(go env GOROOT)/src/crypto/tls/generate_cert.go -duration 8760h -host www.example.com,www.example.org,www.example.test -rsa-bits 512
				"tls.crt": []byte(`-----BEGIN CERTIFICATE-----
MIIBmDCCAUKgAwIBAgIQRr737j1vwFND83io4IliEzANBgkqhkiG9w0BAQsFADAS
MRAwDgYDVQQKEwdBY21lIENvMB4XDTIxMDgzMDE4MTgxNVoXDTIyMDgzMDE4MTgx
NVowEjEQMA4GA1UEChMHQWNtZSBDbzBcMA0GCSqGSIb3DQEBAQUAA0sAMEgCQQC+
0Zlmis2JigXdmCRKF+sZqBuVSPbpBsy4cP7eUBkcyxRir3jwPNoahd6Qv57Tr1vO
ZAj+hb5Rf75T7NgRzrQVAgMBAAGjdDByMA4GA1UdDwEB/wQEAwIFoDATBgNVHSUE
DDAKBggrBgEFBQcDATAMBgNVHRMBAf8EAjAAMD0GA1UdEQQ2MDSCD3d3dy5leGFt
cGxlLmNvbYIPd3d3LmV4YW1wbGUub3JnghB3d3cuZXhhbXBsZS50ZXN0MA0GCSqG
SIb3DQEBCwUAA0EAc/GgmuRfov3QD+RAXcHYQKvmG9WxBRvOK8ALB+l4ibak0rS2
RBUhFyKxlZEjXu5Fhv9PgYBzEA2AcWtiM7j8lA==
-----END CERTIFICATE-----`),
				"tls.key": []byte(`-----BEGIN PRIVATE KEY-----
MIIBVAIBADANBgkqhkiG9w0BAQEFAASCAT4wggE6AgEAAkEAvtGZZorNiYoF3Zgk
ShfrGagblUj26QbMuHD+3lAZHMsUYq948DzaGoXekL+e069bzmQI/oW+UX++U+zY
Ec60FQIDAQABAkB1W83f/lBpXgU7g54WH93NetH0H9sT+MWiToTCUDsRtFkOFpJf
ayKQpriEtcJjW1s/BIW5ldYYi4uJo9rHm+MFAiEA1xwbuJUm+JrlkfrDPsV9fb3p
02hr3cOuC9rVFYPfBzsCIQDjF2pEt0vNmFZLE/EwpiGQ+HB5d8UxCn8cfKqEB52c
7wIgYzTacBGRzJwbfmzJORz52FELEu4YuUky7tK47VhJNtsCIQCRRhhoby3iD1Mc
4lwIOC7+87/YJOOUFNfuHF5k6g5NJwIgYYt7B4pbCW5092Z5M2lDPvujEAr7quDI
wg4cGbIbBPs=
-----END PRIVATE KEY-----`),
			},
		},
	}

	tests := map[string]struct {
		instance      *v1alpha1.RpaasInstance
		assert        func(*testing.T, client.Client, *v1alpha1.RpaasInstance)
		expectedError string
	}{
		"when cert manager fields are set, should create certificate": {
			instance: &v1alpha1.RpaasInstance{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "my-instance",
					Namespace: "rpaasv2",
				},
				Spec: v1alpha1.RpaasInstanceSpec{
					DynamicCertificates: &v1alpha1.DynamicCertificates{
						CertManager: &v1alpha1.CertManager{
							Issuer:      "issuer-1",
							DNSNames:    []string{"my-instance.example.com"},
							IPAddresses: []string{"169.196.1.100"},
						},
					},
				},
			},
			assert: func(t *testing.T, cli client.Client, instance *v1alpha1.RpaasInstance) {
				var cert cmv1.Certificate
				err := cli.Get(context.TODO(), types.NamespacedName{
					Name:      fmt.Sprintf("%s-%s-%s", instance.Name, CertManagerCertificateName, "issuer-1"),
					Namespace: instance.Namespace,
				}, &cert)
				require.NoError(t, err)

				assert.Equal(t, []metav1.OwnerReference{
					{
						APIVersion:         "extensions.tsuru.io/v1alpha1",
						Kind:               "RpaasInstance",
						Name:               "my-instance",
						Controller:         func(b bool) *bool { return &b }(true),
						BlockOwnerDeletion: func(b bool) *bool { return &b }(true),
					},
				}, cert.OwnerReferences)

				assert.Equal(t, map[string]string{
					"rpaas.extensions.tsuru.io/certificate-name": "cert-manager-issuer-1",
					"rpaas.extensions.tsuru.io/instance-name":    "my-instance",
				}, cert.Labels)

				assert.Equal(t, cmv1.CertificateSpec{
					IssuerRef: cmmeta.ObjectReference{
						Name:  "issuer-1",
						Group: "cert-manager.io",
						Kind:  "Issuer",
					},
					SecretName:  cert.Name,
					DNSNames:    []string{"my-instance.example.com"},
					IPAddresses: []string{"169.196.1.100"},
				}, cert.Spec)
			},
		},

		"when cert manager set to use DNS zone, should create certificate": {
			instance: &v1alpha1.RpaasInstance{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "my-instance",
					Namespace: "rpaasv2",
				},
				Spec: v1alpha1.RpaasInstanceSpec{
					DNS: &v1alpha1.DNSConfig{
						Zone: "rpaasv2.example.org",
					},
					DynamicCertificates: &v1alpha1.DynamicCertificates{
						CertManager: &v1alpha1.CertManager{
							Issuer:          "issuer-1",
							DNSNamesDefault: true,
						},
					},
				},
			},
			assert: func(t *testing.T, cli client.Client, instance *v1alpha1.RpaasInstance) {
				var cert cmv1.Certificate
				err := cli.Get(context.TODO(), types.NamespacedName{
					Name:      fmt.Sprintf("%s-%s-%s", instance.Name, CertManagerCertificateName, "issuer-1"),
					Namespace: instance.Namespace,
				}, &cert)
				require.NoError(t, err)

				assert.Equal(t, []metav1.OwnerReference{
					{
						APIVersion:         "extensions.tsuru.io/v1alpha1",
						Kind:               "RpaasInstance",
						Name:               "my-instance",
						Controller:         func(b bool) *bool { return &b }(true),
						BlockOwnerDeletion: func(b bool) *bool { return &b }(true),
					},
				}, cert.OwnerReferences)

				assert.Equal(t, map[string]string{
					"rpaas.extensions.tsuru.io/certificate-name": "cert-manager-issuer-1",
					"rpaas.extensions.tsuru.io/instance-name":    "my-instance",
				}, cert.Labels)

				assert.Equal(t, cmv1.CertificateSpec{
					IssuerRef: cmmeta.ObjectReference{
						Name:  "issuer-1",
						Group: "cert-manager.io",
						Kind:  "Issuer",
					},
					SecretName: cert.Name,
					DNSNames:   []string{"my-instance.rpaasv2.example.org"},
				}, cert.Spec)
			},
		},

		"when DNSes, ips and issuer are changed, should delete the former certificate and create a new one": {
			instance: &v1alpha1.RpaasInstance{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "my-instance-2",
					Namespace: "rpaasv2",
				},
				Spec: v1alpha1.RpaasInstanceSpec{
					DynamicCertificates: &v1alpha1.DynamicCertificates{
						CertManager: &v1alpha1.CertManager{
							Issuer:      "cluster-issuer-1",
							DNSNames:    []string{"my-instance-2.example.com", "app1.example.com"},
							IPAddresses: []string{"2001:db8:dead:beef::"},
						},
					},
				},
			},
			assert: func(t *testing.T, cli client.Client, instance *v1alpha1.RpaasInstance) {
				var cert cmv1.Certificate
				err := cli.Get(context.TODO(), types.NamespacedName{
					Name:      fmt.Sprintf("%s-%s-%s", instance.Name, CertManagerCertificateName, "issuer-1"),
					Namespace: instance.Namespace,
				}, &cert)
				require.Error(t, err)
				assert.True(t, k8serrors.IsNotFound(err))

				err = cli.Get(context.TODO(), types.NamespacedName{
					Name:      fmt.Sprintf("%s-%s-%s", instance.Name, CertManagerCertificateName, "cluster-issuer-1"),
					Namespace: instance.Namespace,
				}, &cert)
				require.NoError(t, err)

				assert.Equal(t, []metav1.OwnerReference{
					{
						APIVersion:         "extensions.tsuru.io/v1alpha1",
						Kind:               "RpaasInstance",
						Name:               "my-instance-2",
						Controller:         func(b bool) *bool { return &b }(true),
						BlockOwnerDeletion: func(b bool) *bool { return &b }(true),
					},
				}, cert.OwnerReferences)

				assert.Equal(t, cmv1.CertificateSpec{
					IssuerRef: cmmeta.ObjectReference{
						Name:  "cluster-issuer-1",
						Group: "cert-manager.io",
						Kind:  "ClusterIssuer",
					},
					SecretName:  cert.Name,
					DNSNames:    []string{"my-instance-2.example.com", "app1.example.com"},
					IPAddresses: []string{"2001:db8:dead:beef::"},
				}, cert.Spec)
			},
		},

		"when cert manager field is removed, should delete certificate": {
			instance: &v1alpha1.RpaasInstance{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "my-instance-2",
					Namespace: "rpaasv2",
				},
				Spec: v1alpha1.RpaasInstanceSpec{},
			},
			assert: func(t *testing.T, cli client.Client, instance *v1alpha1.RpaasInstance) {
				var cert cmv1.Certificate
				err := cli.Get(context.TODO(), types.NamespacedName{
					Name:      instance.Name,
					Namespace: instance.Namespace,
				}, &cert)

				assert.Error(t, err)
				assert.True(t, k8serrors.IsNotFound(err))

				assert.Nil(t, instance.Spec.TLS)
				_, found := instance.Spec.PodTemplate.Annotations["rpaas.extensions.tsuru.io/cert-manager-certificate-sha256"]
				assert.False(t, found)

				var s corev1.Secret
				err = cli.Get(context.TODO(), types.NamespacedName{
					Name:      "my-instance-2-certificates",
					Namespace: instance.Namespace,
				}, &s)
				assert.Error(t, err)
				assert.True(t, k8serrors.IsNotFound(err))

				err = cli.Get(context.TODO(), types.NamespacedName{
					Name:      "my-instance-2-cert-manager",
					Namespace: instance.Namespace,
				}, &s)
				assert.Error(t, err)
				assert.True(t, k8serrors.IsNotFound(err))
			},
		},

		"issuer not found, should return error": {
			instance: &v1alpha1.RpaasInstance{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "my-instance",
					Namespace: "rpaasv2",
				},
				Spec: v1alpha1.RpaasInstanceSpec{
					DynamicCertificates: &v1alpha1.DynamicCertificates{
						CertManager: &v1alpha1.CertManager{
							Issuer: "not-found-issuer",
						},
					},
				},
			},
			expectedError: `there is no "not-found-issuer" certificate issuer`,
		},

		"when certificate is ready, should update the rpaasinstance's certificate secret with newer one": {
			instance: &v1alpha1.RpaasInstance{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "my-instance-3",
					Namespace: "rpaasv2",
				},
				Spec: v1alpha1.RpaasInstanceSpec{
					DynamicCertificates: &v1alpha1.DynamicCertificates{
						CertManager: &v1alpha1.CertManager{
							Issuer:   "issuer-1",
							DNSNames: []string{"my-instance-3.example.com"},
						},
					},
				},
			},
			assert: func(t *testing.T, cli client.Client, instance *v1alpha1.RpaasInstance) {
				assert.Equal(t, []nginxv1alpha1.NginxTLS{
					{SecretName: "my-instance-3-cert-manager-issuer-1", Hosts: []string{"www.example.com", "www.example.org", "www.example.test"}},
				}, instance.Spec.TLS)

				assert.Equal(t, "a0610da4d1958cfa7c375870e2c1bac796e84f509bbd989fa5a7c0e040965f28", instance.Spec.PodTemplate.Annotations["rpaas.extensions.tsuru.io/cert-manager-issue-certificate-sha256"])
				assert.Equal(t, "e644183deec75208c5fc53b4afb98e471ee290c7e7e10c5b95caff6851346132", instance.Spec.PodTemplate.Annotations["rpaas.extensions.tsuru.io/cert-manager-issuer-1-key-sha256"])

				var cert cmv1.Certificate
				err := cli.Get(context.TODO(), types.NamespacedName{
					Name:      "my-instance-3-cert-manager-issuer-1",
					Namespace: instance.Namespace,
				}, &cert)
				require.NoError(t, err)

				var s corev1.Secret
				err = cli.Get(context.TODO(), types.NamespacedName{
					Name:      instance.Spec.TLS[0].SecretName,
					Namespace: instance.Namespace,
				}, &s)
				require.NoError(t, err)

				assert.Equal(t, "my-instance-3", s.Labels["rpaas.extensions.tsuru.io/instance-name"])
				assert.Equal(t, "cert-manager-issuer-1", s.Labels["rpaas.extensions.tsuru.io/certificate-name"])
				assert.Equal(t, "a0610da4d1958cfa7c375870e2c1bac796e84f509bbd989fa5a7c0e040965f28", util.SHA256(s.Data["tls.crt"]))
				assert.Equal(t, "e644183deec75208c5fc53b4afb98e471ee290c7e7e10c5b95caff6851346132", util.SHA256(s.Data["tls.key"]))
			},
		},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			allResources := append(resources, tt.instance)

			cli := fake.NewClientBuilder().
				WithScheme(runtime.NewScheme()).
				WithRuntimeObjects(allResources...).
				Build()

			err := reconcileCertManager(context.TODO(), cli, tt.instance, tt.instance)

			if tt.expectedError != "" {
				assert.EqualError(t, err, tt.expectedError)
				return
			}

			require.NoError(t, err)
			tt.assert(t, cli, tt.instance)
		})
	}
}

func Test_CertManagerCertificateName(t *testing.T) {
	tests := []struct {
		request  v1alpha1.CertManager
		expected string
	}{
		{
			request:  v1alpha1.CertManager{Issuer: "my-issuer-01"},
			expected: "cert-manager-my-issuer-01",
		},
		{
			request:  v1alpha1.CertManager{Issuer: "my-custom-issuer.kind.example.com"},
			expected: "cert-manager-my-custom-issuer-kind-example-com",
		},
		{
			request:  v1alpha1.CertManager{Issuer: "my-custom.ClusterIssuer.example.com"},
			expected: "cert-manager-my-custom-clusterissuer-example-com",
		},
	}

	for _, tt := range tests {
		t.Run(fmt.Sprintf("%v == %q", tt.request, tt.expected), func(t *testing.T) {
			got := cmCertificateName(tt.request)
			assert.Equal(t, tt.expected, got)
		})
	}
}
