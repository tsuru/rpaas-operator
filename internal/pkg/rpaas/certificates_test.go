// Copyright 2021 tsuru authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package rpaas

import (
	"context"
	"testing"

	cmv1 "github.com/jetstack/cert-manager/pkg/apis/certmanager/v1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tsuru/rpaas-operator/api/v1alpha1"
	"github.com/tsuru/rpaas-operator/internal/config"
	clientTypes "github.com/tsuru/rpaas-operator/pkg/rpaas/client/types"
	rpaasruntime "github.com/tsuru/rpaas-operator/pkg/runtime"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func Test_k8sRpaasManager_UpdateCertManagerRequest(t *testing.T) {
	resources := []runtime.Object{
		&v1alpha1.RpaasInstance{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "my-instance-1",
				Namespace: "rpaasv2",
			},
		},

		&cmv1.ClusterIssuer{
			ObjectMeta: metav1.ObjectMeta{
				Name: "issuer-1",
				Annotations: map[string]string{
					allowedDNSZonesAnnotation: "example.com,example.org",
				},
			},
		},
		&cmv1.ClusterIssuer{
			ObjectMeta: metav1.ObjectMeta{
				Name: "default-issuer",
			},
		},
	}

	tests := map[string]struct {
		instanceName  string
		certManager   clientTypes.CertManager
		cfg           config.RpaasConfig
		expectedError string
		assert        func(t *testing.T, cli client.Client)
	}{
		"with integration disabled": {
			expectedError: "Cert Manager integration not enabled",
		},

		"request without issuer and no default issuer": {
			instanceName: "my-instance-1",
			certManager: clientTypes.CertManager{
				DNSNames: []string{"my-instance-1.example.com"},
			},
			cfg: config.RpaasConfig{
				EnableCertManager: true,
			},
			expectedError: "Cert Manager issuer cannot be empty",
		},

		"request without DNSes and IP addresses, should return error": {
			instanceName: "my-instance-1",
			certManager: clientTypes.CertManager{
				Issuer: "issuer-1",
			},
			cfg: config.RpaasConfig{
				EnableCertManager: true,
			},
			expectedError: "you should provide a list of DNS names or IP addresses",
		},

		"using default certificate issuer from configs": {
			instanceName: "my-instance-1",
			certManager: clientTypes.CertManager{
				DNSNames:    []string{"my-instance-1.example.com"},
				IPAddresses: []string{"169.196.100.1"},
			},
			cfg: config.RpaasConfig{
				EnableCertManager:        true,
				DefaultCertManagerIssuer: "default-issuer",
			},
			assert: func(t *testing.T, cli client.Client) {
				var instance v1alpha1.RpaasInstance
				err := cli.Get(context.TODO(), types.NamespacedName{
					Name:      "my-instance-1",
					Namespace: "rpaasv2",
				}, &instance)
				require.NoError(t, err)

				assert.Nil(t, instance.Spec.DynamicCertificates.CertManager)
				assert.Equal(t, []v1alpha1.CertManager{{
					Issuer:      "default-issuer",
					DNSNames:    []string{"my-instance-1.example.com"},
					IPAddresses: []string{"169.196.100.1"},
				}}, instance.Spec.DynamicCertificates.CertManagerRequests)
			},
		},

		"with forbidden DNS names": {
			instanceName: "my-instance-1",
			certManager: clientTypes.CertManager{
				DNSNames: []string{"my-instance-1.example.com", "my-instance-1.example.org", "wrong.io", "wrong.com"},
			},
			cfg: config.RpaasConfig{
				EnableCertManager:        true,
				DefaultCertManagerIssuer: "issuer-1",
			},
			expectedError: "there is some DNS name with forbidden suffix (invalid ones: wrong.io, wrong.com - allowed DNS suffixes: example.com, example.org)",
		},

		"using wrong certificate issuer from configs": {
			instanceName: "my-instance-1",
			certManager: clientTypes.CertManager{
				Issuer:      "not-found-issuer",
				DNSNames:    []string{"my-instance-1.example.com"},
				IPAddresses: []string{"169.196.100.1"},
			},
			cfg: config.RpaasConfig{
				EnableCertManager: true,
			},
			expectedError: "there is no Issuer or ClusterIssuer with \"not-found-issuer\" name",
		},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			cfg := config.Get()
			config.Set(tt.cfg)
			defer func() { config.Set(cfg) }()

			client := fake.NewClientBuilder().
				WithScheme(rpaasruntime.NewScheme()).
				WithRuntimeObjects(resources...).
				Build()

			manager := &k8sRpaasManager{cli: client}

			err := manager.UpdateCertManagerRequest(context.TODO(), tt.instanceName, tt.certManager)
			if tt.expectedError != "" {
				assert.EqualError(t, err, tt.expectedError)
				return
			}
			require.NoError(t, err)
			require.NotNil(t, tt.assert)
			tt.assert(t, client)
		})
	}
}

func Test_k8sRpaasManager_DeleteCertManagerRequest(t *testing.T) {
	resources := []runtime.Object{
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
				DynamicCertificates: &v1alpha1.DynamicCertificates{
					CertManager: &v1alpha1.CertManager{
						Issuer:   "my-issuer",
						DNSNames: []string{"my-instance-2.example.com"},
					},
					CertManagerRequests: []v1alpha1.CertManager{
						{
							Issuer:   "default-issuer",
							DNSNames: []string{"my-instance.example.org", "www.my-instance.example.org"},
						},
					},
				},
			},
		},
	}

	tests := map[string]struct {
		cfg           config.RpaasConfig
		instanceName  string
		issuer        string
		expectedError string
		assert        func(*testing.T, client.Client)
	}{
		"when issuer is not provided": {
			instanceName:  "my-instance-1",
			expectedError: "cert-manager issuer cannot be empty",
		},

		"removing request using default issuer": {
			cfg: config.RpaasConfig{
				DefaultCertManagerIssuer: "default-issuer",
			},
			instanceName: "my-instance-2",
			assert: func(t *testing.T, cli client.Client) {
				var instance v1alpha1.RpaasInstance
				require.NoError(t, cli.Get(context.TODO(), types.NamespacedName{Name: "my-instance-2", Namespace: "rpaasv2"}, &instance))
				require.NotNil(t, instance.Spec.DynamicCertificates)
				assert.NotNil(t, instance.Spec.DynamicCertificates.CertManager)
				assert.Len(t, instance.Spec.DynamicCertificates.CertManagerRequests, 0)
			},
		},

		"removing request from specific issuer": {
			instanceName: "my-instance-2",
			issuer:       "my-issuer",
			assert: func(t *testing.T, cli client.Client) {
				var instance v1alpha1.RpaasInstance
				require.NoError(t, cli.Get(context.TODO(), types.NamespacedName{Name: "my-instance-2", Namespace: "rpaasv2"}, &instance))
				require.NotNil(t, instance.Spec.DynamicCertificates)
				assert.Nil(t, instance.Spec.DynamicCertificates.CertManager)
				assert.Len(t, instance.Spec.DynamicCertificates.CertManagerRequests, 1)
			},
		},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			oldCfg := config.Get()
			defer config.Set(oldCfg)

			config.Set(tt.cfg)

			client := fake.NewClientBuilder().
				WithScheme(rpaasruntime.NewScheme()).
				WithRuntimeObjects(resources...).
				Build()

			manager := &k8sRpaasManager{cli: client}

			err := manager.DeleteCertManagerRequest(context.TODO(), tt.instanceName, tt.issuer)
			if tt.expectedError != "" {
				assert.EqualError(t, err, tt.expectedError)
				return
			}

			require.NoError(t, err)
			require.NotNil(t, tt.assert)

			tt.assert(t, client)
		})
	}
}
