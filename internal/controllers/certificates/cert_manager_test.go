// Copyright 2021 tsuru authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package certificates

import (
	"context"
	"testing"

	cmv1 "github.com/jetstack/cert-manager/pkg/apis/certmanager/v1"
	cmmeta "github.com/jetstack/cert-manager/pkg/apis/meta/v1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	k8sruntime "k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	"github.com/tsuru/rpaas-operator/api/v1alpha1"
	"github.com/tsuru/rpaas-operator/pkg/runtime"
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

		&cmv1.Certificate{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "my-instance-2",
				Namespace: "rpaasv2",
			},
			Spec: cmv1.CertificateSpec{
				IssuerRef: cmmeta.ObjectReference{
					Name:  "issuer-1",
					Kind:  "Issuer",
					Group: "cert-manager.io",
				},
				SecretName: "another-secret",
				DNSNames:   []string{"my-instance-2.example.com"},
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
					AutoCertificates: &v1alpha1.AutoCertificates{
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
					Name:      instance.Name,
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

				assert.Equal(t, cmv1.CertificateSpec{
					IssuerRef: cmmeta.ObjectReference{
						Name:  "issuer-1",
						Group: "cert-manager.io",
						Kind:  "Issuer",
					},
					SecretName:  "my-instance-cert-manager",
					DNSNames:    []string{"my-instance.example.com"},
					IPAddresses: []string{"169.196.1.100"},
				}, cert.Spec)
			},
		},

		"when DNSes, ips and issuer are changed, certificate should be updated according to": {
			instance: &v1alpha1.RpaasInstance{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "my-instance-2",
					Namespace: "rpaasv2",
				},
				Spec: v1alpha1.RpaasInstanceSpec{
					AutoCertificates: &v1alpha1.AutoCertificates{
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
					Name:      instance.Name,
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
					SecretName:  "my-instance-2-cert-manager",
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
			},
		},

		"issuer not found, should return error": {
			instance: &v1alpha1.RpaasInstance{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "my-instance",
					Namespace: "rpaasv2",
				},
				Spec: v1alpha1.RpaasInstanceSpec{
					AutoCertificates: &v1alpha1.AutoCertificates{
						CertManager: &v1alpha1.CertManager{
							Issuer: "not-found-issuer",
						},
					},
				},
			},
			expectedError: `there is no Issuer or ClusterIssuer with "not-found-issuer" name`,
		},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			cli := fake.NewClientBuilder().
				WithScheme(runtime.NewScheme()).
				WithRuntimeObjects(resources...).
				Build()

			err := reconcileCertManager(context.TODO(), cli, tt.instance)

			if tt.expectedError != "" {
				assert.EqualError(t, err, tt.expectedError)
				return
			}

			require.NoError(t, err)
			tt.assert(t, cli, tt.instance)
		})
	}
}
