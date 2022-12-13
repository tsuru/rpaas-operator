// Copyright 2019 tsuru authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package rpaas

import (
	"context"
	"crypto/tls"
	"fmt"
	"net/http"
	"regexp"
	"testing"
	"time"

	"github.com/imdario/mergo"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	nginxv1alpha1 "github.com/tsuru/nginx-operator/api/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	k8sErrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/validation"
	metricsv1beta1 "k8s.io/metrics/pkg/apis/metrics/v1beta1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	osb "sigs.k8s.io/go-open-service-broker-client/v2"

	"github.com/tsuru/rpaas-operator/api/v1alpha1"
	"github.com/tsuru/rpaas-operator/internal/config"
	"github.com/tsuru/rpaas-operator/internal/controllers/certificates"
	nginxManager "github.com/tsuru/rpaas-operator/internal/pkg/rpaas/nginx"
	clientTypes "github.com/tsuru/rpaas-operator/pkg/rpaas/client/types"
)

var (
	rsaCertificateInPEM = `-----BEGIN CERTIFICATE-----
MIIBkDCCATqgAwIBAgIRAMSjo93UEsGj+o2eIlzWNy4wDQYJKoZIhvcNAQELBQAw
EjEQMA4GA1UEChMHQWNtZSBDbzAeFw0yMDA4MTIyMDI3NDZaFw0yMTA4MTIyMDI3
NDZaMBIxEDAOBgNVBAoTB0FjbWUgQ28wXDANBgkqhkiG9w0BAQEFAANLADBIAkEA
s3dnWuieG330c2eykPY+J0V4QA9HhdBu3v9lthl98suovwyu0OT5+1Z08a7jzvg4
uXMndqvAtsTziyAIParbGQIDAQABo2swaTAOBgNVHQ8BAf8EBAMCBaAwEwYDVR0l
BAwwCgYIKwYBBQUHAwEwDAYDVR0TAQH/BAIwADA0BgNVHREELTArgglsb2NhbGhv
c3SCC2V4YW1wbGUuY29tghFhbm90aGVyLW5hbWUudGVzdDANBgkqhkiG9w0BAQsF
AANBACs5SDH+/F69gHCA9u0pecSu4m3X4rbsaIh8JtsKEcu5ZZds/sneQCmPNMdX
fbMpGtSYnl7faM2998SQyZdRG3Y=
-----END CERTIFICATE-----
`

	rsaPrivateKeyInPEM = `-----BEGIN PRIVATE KEY-----
MIIBVQIBADANBgkqhkiG9w0BAQEFAASCAT8wggE7AgEAAkEAs3dnWuieG330c2ey
kPY+J0V4QA9HhdBu3v9lthl98suovwyu0OT5+1Z08a7jzvg4uXMndqvAtsTziyAI
ParbGQIDAQABAkBsFECeMvDkxZnt1klnm6Qaqm+cxJbiM4BRs6VhYUDEcnbG8avN
MmpVklT8XF05q6TnKBu7hYdtp8LGUzESPBbxAiEAw0hWWzJTzvQnPY9m6n83sr2B
qxha+CMiwlKqW8EBkNcCIQDrRCsWbB2PVO//YIIUnlWCXBAvoBQAQpYgAWQ3dydF
jwIgM/dsA5jA9LHEP32JxZ1VFRuZBg7VJnMzLMMS0pfp8sECIQDR82qUPvV+NKlc
eE59gfMDO49CQRO4S7PXagZ6LP5B5wIhAKIa9I9OkA1O8PJEXg2lfYHRnHOvZAqH
0NTbXH+sPIfT
-----END PRIVATE KEY-----
`

	ecdsaCertificateInPEM = `-----BEGIN CERTIFICATE-----
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

	ecdsaPrivateKeyInPEM = `-----BEGIN EC PRIVATE KEY-----
MHcCAQEEIIrYSSNQFaA2Hwf1duRSxKtLYX5CB04fSeQ6tF1aY/PuoAoGCCqGSM49
AwEHoUQDQgAEPR3tU2Fta9ktY+6P9G0cWO+0kETA6SFs38GecTyudlHz6xvCdz8q
EKTcWGekdmdDPsHloRNtsiCa697B2O9IFA==
-----END EC PRIVATE KEY-----`
)

type fakeCacheManager struct {
	purgeCacheFunc func(host, path string, port int32, preservePath bool, extraHeaders http.Header) (bool, error)
}

func (f fakeCacheManager) PurgeCache(host, path string, port int32, preservePath bool, extraHeaders http.Header) (bool, error) {
	if f.purgeCacheFunc != nil {
		return f.purgeCacheFunc(host, path, port, preservePath, extraHeaders)
	}
	return false, nil
}

func Test_k8sRpaasManager_DeleteBlock(t *testing.T) {
	tests := []struct {
		name      string
		instance  string
		block     string
		resources func() []runtime.Object
		assertion func(*testing.T, error, v1alpha1.RpaasInstance)
	}{
		{
			name:     "when block does not exist",
			instance: "my-instance",
			block:    "unknown-block",
			resources: func() []runtime.Object {
				return []runtime.Object{newEmptyRpaasInstance()}
			},
			assertion: func(t *testing.T, err error, _ v1alpha1.RpaasInstance) {
				assert.Error(t, err)
				assert.Equal(t, NotFoundError{Msg: "block \"unknown-block\" not found"}, err)
			},
		},
		{
			name:     "when removing the last remaining block",
			instance: "another-instance",
			block:    "http",
			resources: func() []runtime.Object {
				instance := newEmptyRpaasInstance()
				instance.Name = "another-instance"
				instance.Spec.Blocks = map[v1alpha1.BlockType]v1alpha1.Value{
					v1alpha1.BlockTypeHTTP: {
						Value: "# Some NGINX configuration at HTTP scope",
					},
				}
				return []runtime.Object{instance}
			},
			assertion: func(t *testing.T, err error, instance v1alpha1.RpaasInstance) {
				assert.NoError(t, err)
				assert.Nil(t, instance.Spec.Blocks)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			manager := &k8sRpaasManager{cli: fake.NewClientBuilder().WithScheme(newScheme()).WithRuntimeObjects(tt.resources()...).Build()}
			err := manager.DeleteBlock(context.TODO(), tt.instance, tt.block)
			var instance v1alpha1.RpaasInstance
			if err == nil {
				err1 := manager.cli.Get(context.TODO(), types.NamespacedName{
					Name:      tt.instance,
					Namespace: getServiceName(),
				}, &instance)
				require.NoError(t, err1)
			}
			tt.assertion(t, err, instance)
		})
	}
}

func Test_k8sRpaasManager_ListBlocks(t *testing.T) {
	tests := []struct {
		name      string
		resources func() []runtime.Object
		instance  string
		assertion func(t *testing.T, err error, blocks []ConfigurationBlock)
	}{
		{
			name: "when instance not found",
			resources: func() []runtime.Object {
				return []runtime.Object{}
			},
			instance: "unknown-instance",
			assertion: func(t *testing.T, err error, blocks []ConfigurationBlock) {
				assert.Error(t, err)
				assert.True(t, IsNotFoundError(err))
			},
		},
		{
			name: "when instance has no blocks",
			resources: func() []runtime.Object {
				return []runtime.Object{
					newEmptyRpaasInstance(),
				}
			},
			instance: "my-instance",
			assertion: func(t *testing.T, err error, blocks []ConfigurationBlock) {
				assert.NoError(t, err)
				assert.Nil(t, blocks)
			},
		},
		{
			name: "when instance has two blocks from different sources",
			resources: func() []runtime.Object {
				instance := newEmptyRpaasInstance()
				instance.Spec.Blocks = map[v1alpha1.BlockType]v1alpha1.Value{
					v1alpha1.BlockTypeHTTP: {
						Value: "# some NGINX conf at http context",
					},
					v1alpha1.BlockTypeServer: {
						ValueFrom: &v1alpha1.ValueSource{
							ConfigMapKeyRef: &corev1.ConfigMapKeySelector{
								LocalObjectReference: corev1.LocalObjectReference{
									Name: "my-instance-blocks",
								},
								Key: "server",
							},
						},
					},
				}
				cm := &corev1.ConfigMap{
					TypeMeta: metav1.TypeMeta{
						APIVersion: "v1",
						Kind:       "ConfigMap",
					},
					ObjectMeta: metav1.ObjectMeta{
						Name:      "my-instance-blocks",
						Namespace: getServiceName(),
					},
					Data: map[string]string{
						"server": "# some NGINX conf at server context",
					},
				}
				return []runtime.Object{instance, cm}
			},
			instance: "my-instance",
			assertion: func(t *testing.T, err error, blocks []ConfigurationBlock) {
				assert.NoError(t, err)
				assert.Equal(t, []ConfigurationBlock{
					{Name: "http", Content: "# some NGINX conf at http context"},
					{Name: "server", Content: "# some NGINX conf at server context"},
				}, blocks)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			manager := &k8sRpaasManager{cli: fake.NewClientBuilder().WithScheme(newScheme()).WithRuntimeObjects(tt.resources()...).Build()}
			blocks, err := manager.ListBlocks(context.TODO(), tt.instance)
			tt.assertion(t, err, blocks)
		})
	}
}

func Test_k8sRpaasManager_UpdateBlock(t *testing.T) {
	tests := []struct {
		name      string
		resources func() []runtime.Object
		instance  string
		block     ConfigurationBlock
		assertion func(t *testing.T, err error, instance *v1alpha1.RpaasInstance)
	}{
		{
			name: "when instance is not found",
			resources: func() []runtime.Object {
				return []runtime.Object{}
			},
			instance: "my-instance",
			block:    ConfigurationBlock{Name: "http", Content: "# some NGINX configuration"},
			assertion: func(t *testing.T, err error, _ *v1alpha1.RpaasInstance) {
				assert.Error(t, err)
				assert.True(t, IsNotFoundError(err))
			},
		},
		{
			name: "when block name is not allowed",
			resources: func() []runtime.Object {
				return []runtime.Object{
					newEmptyRpaasInstance(),
				}
			},
			instance: "my-instance",
			block:    ConfigurationBlock{Name: "unknown block"},
			assertion: func(t *testing.T, err error, _ *v1alpha1.RpaasInstance) {
				assert.Error(t, err)
				assert.Equal(t, ValidationError{Msg: "block \"unknown block\" is not allowed"}, err)
			},
		},
		{
			name: "when adding an HTTP block",
			resources: func() []runtime.Object {
				return []runtime.Object{
					newEmptyRpaasInstance(),
				}
			},
			instance: "my-instance",
			block:    ConfigurationBlock{Name: "http", Content: "# my custom http configuration"},
			assertion: func(t *testing.T, err error, instance *v1alpha1.RpaasInstance) {
				require.NoError(t, err)
				assert.Equal(t, map[v1alpha1.BlockType]v1alpha1.Value{
					v1alpha1.BlockTypeHTTP: {
						Value: "# my custom http configuration",
					},
				}, instance.Spec.Blocks)
			},
		},
		{
			name: "when updating an root block",
			resources: func() []runtime.Object {
				instance := newEmptyRpaasInstance()
				instance.Spec.Blocks = map[v1alpha1.BlockType]v1alpha1.Value{
					v1alpha1.BlockTypeRoot: {Value: "# some old root configuration"},
				}
				return []runtime.Object{instance}
			},
			instance: "my-instance",
			block:    ConfigurationBlock{Name: "root", Content: "# my custom http configuration"},
			assertion: func(t *testing.T, err error, instance *v1alpha1.RpaasInstance) {
				require.NoError(t, err)
				assert.Equal(t, map[v1alpha1.BlockType]v1alpha1.Value{
					v1alpha1.BlockTypeRoot: {
						Value: "# my custom http configuration",
					},
				}, instance.Spec.Blocks)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			manager := &k8sRpaasManager{
				cli: fake.NewClientBuilder().WithScheme(newScheme()).WithRuntimeObjects(tt.resources()...).Build(),
			}
			err := manager.UpdateBlock(context.TODO(), tt.instance, tt.block)
			var instance v1alpha1.RpaasInstance
			if err == nil {
				err1 := manager.cli.Get(context.TODO(), types.NamespacedName{Name: tt.instance, Namespace: getServiceName()}, &instance)
				require.NoError(t, err1)
			}
			tt.assertion(t, err, &instance)
		})
	}
}

func Test_k8sRpaasManager_GetCertificates(t *testing.T) {
	resources := []runtime.Object{
		&v1alpha1.RpaasInstance{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "my-instance-1",
				Namespace: "rpaasv2",
			},
			Spec: v1alpha1.RpaasInstanceSpec{
				TLS: []nginxv1alpha1.NginxTLS{
					{SecretName: "my-instance-certs-abc123"},
				},
			},
		},

		&corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "my-instance-certs-abc123",
				Namespace: "rpaasv2",
				Labels: map[string]string{
					certificates.CertificateNameLabel: "default",
				},
			},
			Data: map[string][]byte{
				"tls.crt": []byte(`-----BEGIN CERTIFICATE-----
MIIB9TCCAV6gAwIBAgIRAIpoagB8BUn8x36iyvafmC0wDQYJKoZIhvcNAQELBQAw
EjEQMA4GA1UEChMHQWNtZSBDbzAeFw0xOTAzMjYyMDIxMzlaFw0yMDAzMjUyMDIx
MzlaMBIxEDAOBgNVBAoTB0FjbWUgQ28wgZ8wDQYJKoZIhvcNAQEBBQADgY0AMIGJ
AoGBAOIsM9LhHqI3oBhHDCGZkGKgiI72ghnLr5UpaA3I9U7np/LPzt/JpWRG4wjF
5Var2IRPGoNwLcdybFW0YTqvw1wNY88q9BcpwS5PeV7uWyZqWafdSxxveaG6VeCH
YFMqopOKri4kJ4sZB9WS3xMlGZXK6zHPwA4xPtuVEND+LI17AgMBAAGjSzBJMA4G
A1UdDwEB/wQEAwIFoDATBgNVHSUEDDAKBggrBgEFBQcDATAMBgNVHRMBAf8EAjAA
MBQGA1UdEQQNMAuCCWxvY2FsaG9zdDANBgkqhkiG9w0BAQsFAAOBgQCaF9zDYoPh
4KmqxFI3KB+cl8Z/0y0txxH4vqlnByBBiCLpPzivcCRFlT1bGPVJOLsyd/BdOset
yTcvMUPbnEPXZMR4Dsbzzjco1JxMSvZgkhm85gAlwNGjFZrMXqO8G5R/gpWN3UUc
7likRQOu7q61DlicQAZXRnOh6BbKaq1clg==
-----END CERTIFICATE-----`),
				"tls.key": []byte(`-----BEGIN RSA PRIVATE KEY-----
MIICXQIBAAKBgQDiLDPS4R6iN6AYRwwhmZBioIiO9oIZy6+VKWgNyPVO56fyz87f
yaVkRuMIxeVWq9iETxqDcC3HcmxVtGE6r8NcDWPPKvQXKcEuT3le7lsmalmn3Usc
b3mhulXgh2BTKqKTiq4uJCeLGQfVkt8TJRmVyusxz8AOMT7blRDQ/iyNewIDAQAB
AoGBAI05gJqayyALj8HZCzAnzUpoZxytvAsTbm27TyfcZaCBchNhwxFlvgphYP5n
Y468+xOSuUF9WHiDcDYLzfJxMZAqmuS+D/IREYDkcrGVT1MXfSCkNaFVqG52+hLZ
GmGsy8+KsJnDJ1HYmwfSnaTj3L8+Bf2Hg291Yb1caRH9+5vBAkEA7P5N3cSN73Fa
HwaWzqkaY75mCR4TpRi27YWGA3wdQek2G71HiSbCOxrWOymvgoNRi6M/sdrP5PTt
JAFxC+pd8QJBAPRPvS0Tm/0lMIZ0q7jxyoW/gKDzokmSszopdlvSU53lN06vaYdK
XyTvqOO95nJx0DjkdM26QojJlSueMTitJisCQDuxNfWku0dTGqrz4uo8p5v16gdj
3vjXh8O9vOqFyWy/i9Ri0XDXJVbzxH/0WPObld+BB9sJTRHTKyPFhS7GIlECQDZ8
chxTez6BxMi3zHR6uEgL5Yv/yfnOldoq1RK1XaChNix+QnLBy2ZZbLkd6P8tEtsd
WE9pct0+193ace/J7fECQQDAhwHBpJjhM+k97D92akneKXIUBo+Egr5E5qF9/g5I
sM5FaDCEIJVbWjPDluxUGbVOQlFHsJs+pZv0Anf9DPwU
-----END RSA PRIVATE KEY-----`),
			},
		},

		&corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "my-instance-certs-abc456",
				Namespace: "rpaasv2",
				Labels: map[string]string{
					certificates.CertificateNameLabel: "www.example.com",
				},
			},
			Data: map[string][]byte{
				"tls.crt": []byte(`-----BEGIN CERTIFICATE-----
MIIB9TCCAV6gAwIBAgIRAIpoagB8BUn8x36iyvafmC0wDQYJKoZIhvcNAQELBQAw
EjEQMA4GA1UEChMHQWNtZSBDbzAeFw0xOTAzMjYyMDIxMzlaFw0yMDAzMjUyMDIx
MzlaMBIxEDAOBgNVBAoTB0FjbWUgQ28wgZ8wDQYJKoZIhvcNAQEBBQADgY0AMIGJ
AoGBAOIsM9LhHqI3oBhHDCGZkGKgiI72ghnLr5UpaA3I9U7np/LPzt/JpWRG4wjF
5Var2IRPGoNwLcdybFW0YTqvw1wNY88q9BcpwS5PeV7uWyZqWafdSxxveaG6VeCH
YFMqopOKri4kJ4sZB9WS3xMlGZXK6zHPwA4xPtuVEND+LI17AgMBAAGjSzBJMA4G
A1UdDwEB/wQEAwIFoDATBgNVHSUEDDAKBggrBgEFBQcDATAMBgNVHRMBAf8EAjAA
MBQGA1UdEQQNMAuCCWxvY2FsaG9zdDANBgkqhkiG9w0BAQsFAAOBgQCaF9zDYoPh
4KmqxFI3KB+cl8Z/0y0txxH4vqlnByBBiCLpPzivcCRFlT1bGPVJOLsyd/BdOset
yTcvMUPbnEPXZMR4Dsbzzjco1JxMSvZgkhm85gAlwNGjFZrMXqO8G5R/gpWN3UUc
7likRQOu7q61DlicQAZXRnOh6BbKaq1clg==
-----END CERTIFICATE-----`),
				"tls.key": []byte(`-----BEGIN RSA PRIVATE KEY-----
MIICXQIBAAKBgQDiLDPS4R6iN6AYRwwhmZBioIiO9oIZy6+VKWgNyPVO56fyz87f
yaVkRuMIxeVWq9iETxqDcC3HcmxVtGE6r8NcDWPPKvQXKcEuT3le7lsmalmn3Usc
b3mhulXgh2BTKqKTiq4uJCeLGQfVkt8TJRmVyusxz8AOMT7blRDQ/iyNewIDAQAB
AoGBAI05gJqayyALj8HZCzAnzUpoZxytvAsTbm27TyfcZaCBchNhwxFlvgphYP5n
Y468+xOSuUF9WHiDcDYLzfJxMZAqmuS+D/IREYDkcrGVT1MXfSCkNaFVqG52+hLZ
GmGsy8+KsJnDJ1HYmwfSnaTj3L8+Bf2Hg291Yb1caRH9+5vBAkEA7P5N3cSN73Fa
HwaWzqkaY75mCR4TpRi27YWGA3wdQek2G71HiSbCOxrWOymvgoNRi6M/sdrP5PTt
JAFxC+pd8QJBAPRPvS0Tm/0lMIZ0q7jxyoW/gKDzokmSszopdlvSU53lN06vaYdK
XyTvqOO95nJx0DjkdM26QojJlSueMTitJisCQDuxNfWku0dTGqrz4uo8p5v16gdj
3vjXh8O9vOqFyWy/i9Ri0XDXJVbzxH/0WPObld+BB9sJTRHTKyPFhS7GIlECQDZ8
chxTez6BxMi3zHR6uEgL5Yv/yfnOldoq1RK1XaChNix+QnLBy2ZZbLkd6P8tEtsd
WE9pct0+193ace/J7fECQQDAhwHBpJjhM+k97D92akneKXIUBo+Egr5E5qF9/g5I
sM5FaDCEIJVbWjPDluxUGbVOQlFHsJs+pZv0Anf9DPwU
-----END RSA PRIVATE KEY-----`),
			},
		},
	}

	tests := map[string]struct {
		name          string
		instanceName  string
		expectedError string
		expected      []CertificateData
	}{
		"instance not found": {
			instanceName:  "instance-not-found",
			expectedError: "rpaas instance \"instance-not-found\" not found",
		},

		"getting an existing certificate": {
			instanceName: "my-instance-1",
			expected: []CertificateData{
				{
					Name: "default",
					Certificate: `-----BEGIN CERTIFICATE-----
MIIB9TCCAV6gAwIBAgIRAIpoagB8BUn8x36iyvafmC0wDQYJKoZIhvcNAQELBQAw
EjEQMA4GA1UEChMHQWNtZSBDbzAeFw0xOTAzMjYyMDIxMzlaFw0yMDAzMjUyMDIx
MzlaMBIxEDAOBgNVBAoTB0FjbWUgQ28wgZ8wDQYJKoZIhvcNAQEBBQADgY0AMIGJ
AoGBAOIsM9LhHqI3oBhHDCGZkGKgiI72ghnLr5UpaA3I9U7np/LPzt/JpWRG4wjF
5Var2IRPGoNwLcdybFW0YTqvw1wNY88q9BcpwS5PeV7uWyZqWafdSxxveaG6VeCH
YFMqopOKri4kJ4sZB9WS3xMlGZXK6zHPwA4xPtuVEND+LI17AgMBAAGjSzBJMA4G
A1UdDwEB/wQEAwIFoDATBgNVHSUEDDAKBggrBgEFBQcDATAMBgNVHRMBAf8EAjAA
MBQGA1UdEQQNMAuCCWxvY2FsaG9zdDANBgkqhkiG9w0BAQsFAAOBgQCaF9zDYoPh
4KmqxFI3KB+cl8Z/0y0txxH4vqlnByBBiCLpPzivcCRFlT1bGPVJOLsyd/BdOset
yTcvMUPbnEPXZMR4Dsbzzjco1JxMSvZgkhm85gAlwNGjFZrMXqO8G5R/gpWN3UUc
7likRQOu7q61DlicQAZXRnOh6BbKaq1clg==
-----END CERTIFICATE-----`,
					Key: `-----BEGIN RSA PRIVATE KEY-----
MIICXQIBAAKBgQDiLDPS4R6iN6AYRwwhmZBioIiO9oIZy6+VKWgNyPVO56fyz87f
yaVkRuMIxeVWq9iETxqDcC3HcmxVtGE6r8NcDWPPKvQXKcEuT3le7lsmalmn3Usc
b3mhulXgh2BTKqKTiq4uJCeLGQfVkt8TJRmVyusxz8AOMT7blRDQ/iyNewIDAQAB
AoGBAI05gJqayyALj8HZCzAnzUpoZxytvAsTbm27TyfcZaCBchNhwxFlvgphYP5n
Y468+xOSuUF9WHiDcDYLzfJxMZAqmuS+D/IREYDkcrGVT1MXfSCkNaFVqG52+hLZ
GmGsy8+KsJnDJ1HYmwfSnaTj3L8+Bf2Hg291Yb1caRH9+5vBAkEA7P5N3cSN73Fa
HwaWzqkaY75mCR4TpRi27YWGA3wdQek2G71HiSbCOxrWOymvgoNRi6M/sdrP5PTt
JAFxC+pd8QJBAPRPvS0Tm/0lMIZ0q7jxyoW/gKDzokmSszopdlvSU53lN06vaYdK
XyTvqOO95nJx0DjkdM26QojJlSueMTitJisCQDuxNfWku0dTGqrz4uo8p5v16gdj
3vjXh8O9vOqFyWy/i9Ri0XDXJVbzxH/0WPObld+BB9sJTRHTKyPFhS7GIlECQDZ8
chxTez6BxMi3zHR6uEgL5Yv/yfnOldoq1RK1XaChNix+QnLBy2ZZbLkd6P8tEtsd
WE9pct0+193ace/J7fECQQDAhwHBpJjhM+k97D92akneKXIUBo+Egr5E5qF9/g5I
sM5FaDCEIJVbWjPDluxUGbVOQlFHsJs+pZv0Anf9DPwU
-----END RSA PRIVATE KEY-----`,
				},
			},
		},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			client := fake.NewClientBuilder().
				WithScheme(newScheme()).
				WithRuntimeObjects(resources...).
				Build()

			got, err := (&k8sRpaasManager{cli: client}).GetCertificates(context.TODO(), tt.instanceName)
			if tt.expectedError != "" {
				assert.EqualError(t, err, tt.expectedError)
				return
			}

			assert.Equal(t, tt.expected, got)
		})
	}
}

func Test_k8sRpaasManager_DeleteCertificate(t *testing.T) {
	rsaCertPem := `-----BEGIN CERTIFICATE-----
MIIB9TCCAV6gAwIBAgIRAIpoagB8BUn8x36iyvafmC0wDQYJKoZIhvcNAQELBQAw
EjEQMA4GA1UEChMHQWNtZSBDbzAeFw0xOTAzMjYyMDIxMzlaFw0yMDAzMjUyMDIx
MzlaMBIxEDAOBgNVBAoTB0FjbWUgQ28wgZ8wDQYJKoZIhvcNAQEBBQADgY0AMIGJ
AoGBAOIsM9LhHqI3oBhHDCGZkGKgiI72ghnLr5UpaA3I9U7np/LPzt/JpWRG4wjF
5Var2IRPGoNwLcdybFW0YTqvw1wNY88q9BcpwS5PeV7uWyZqWafdSxxveaG6VeCH
YFMqopOKri4kJ4sZB9WS3xMlGZXK6zHPwA4xPtuVEND+LI17AgMBAAGjSzBJMA4G
A1UdDwEB/wQEAwIFoDATBgNVHSUEDDAKBggrBgEFBQcDATAMBgNVHRMBAf8EAjAA
MBQGA1UdEQQNMAuCCWxvY2FsaG9zdDANBgkqhkiG9w0BAQsFAAOBgQCaF9zDYoPh
4KmqxFI3KB+cl8Z/0y0txxH4vqlnByBBiCLpPzivcCRFlT1bGPVJOLsyd/BdOset
yTcvMUPbnEPXZMR4Dsbzzjco1JxMSvZgkhm85gAlwNGjFZrMXqO8G5R/gpWN3UUc
7likRQOu7q61DlicQAZXRnOh6BbKaq1clg==
-----END CERTIFICATE-----`

	rsaKeyPem := `-----BEGIN RSA PRIVATE KEY-----
MIICXQIBAAKBgQDiLDPS4R6iN6AYRwwhmZBioIiO9oIZy6+VKWgNyPVO56fyz87f
yaVkRuMIxeVWq9iETxqDcC3HcmxVtGE6r8NcDWPPKvQXKcEuT3le7lsmalmn3Usc
b3mhulXgh2BTKqKTiq4uJCeLGQfVkt8TJRmVyusxz8AOMT7blRDQ/iyNewIDAQAB
AoGBAI05gJqayyALj8HZCzAnzUpoZxytvAsTbm27TyfcZaCBchNhwxFlvgphYP5n
Y468+xOSuUF9WHiDcDYLzfJxMZAqmuS+D/IREYDkcrGVT1MXfSCkNaFVqG52+hLZ
GmGsy8+KsJnDJ1HYmwfSnaTj3L8+Bf2Hg291Yb1caRH9+5vBAkEA7P5N3cSN73Fa
HwaWzqkaY75mCR4TpRi27YWGA3wdQek2G71HiSbCOxrWOymvgoNRi6M/sdrP5PTt
JAFxC+pd8QJBAPRPvS0Tm/0lMIZ0q7jxyoW/gKDzokmSszopdlvSU53lN06vaYdK
XyTvqOO95nJx0DjkdM26QojJlSueMTitJisCQDuxNfWku0dTGqrz4uo8p5v16gdj
3vjXh8O9vOqFyWy/i9Ri0XDXJVbzxH/0WPObld+BB9sJTRHTKyPFhS7GIlECQDZ8
chxTez6BxMi3zHR6uEgL5Yv/yfnOldoq1RK1XaChNix+QnLBy2ZZbLkd6P8tEtsd
WE9pct0+193ace/J7fECQQDAhwHBpJjhM+k97D92akneKXIUBo+Egr5E5qF9/g5I
sM5FaDCEIJVbWjPDluxUGbVOQlFHsJs+pZv0Anf9DPwU
-----END RSA PRIVATE KEY-----`

	ecdsaCertPem := `-----BEGIN CERTIFICATE-----
JUNDACERTJUNDACERT
-----END CERTIFICATE-----`

	ecdsaKeyPem := `-----BEGIN EC PRIVATE KEY-----
JUNDAKEYJUNDAKEYJUNDAKEY
-----END EC PRIVATE KEY-----`

	instance1 := newEmptyRpaasInstance()

	instance2 := newEmptyRpaasInstance()
	instance2.Name = "another-instance"
	instance2.Spec.TLS = []nginxv1alpha1.NginxTLS{
		{SecretName: "another-instance-certs-01"},
		{SecretName: "another-instance-certs-02"},
	}

	secret1 := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "another-instance-certs-01",
			Namespace: instance2.Namespace,
			Labels: map[string]string{
				"rpaas.extensions.tsuru.io/certificate-name": "default",
				"rpaas.extensions.tsuru.io/instance-name":    "another-instance",
			},
		},
		Data: map[string][]byte{
			"tls.crt": []byte(rsaCertPem),
			"tls.key": []byte(rsaKeyPem),
		},
	}

	secret2 := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "another-instance-certs-02",
			Namespace: instance2.Namespace,
			Labels: map[string]string{
				"rpaas.extensions.tsuru.io/certificate-name": "junda",
				"rpaas.extensions.tsuru.io/instance-name":    "another-instance",
			},
		},
		Data: map[string][]byte{
			"tls.crt": []byte(ecdsaCertPem),
			"tls.key": []byte(ecdsaKeyPem),
		},
	}

	instance3 := newEmptyRpaasInstance()
	instance3.Name = "no-spec-cert"

	instance4 := newEmptyRpaasInstance()
	instance4.Name = "one-cert"
	instance4.Spec.TLS = []nginxv1alpha1.NginxTLS{
		{SecretName: "one-cert-secret-certs-02"},
	}

	secret3 := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "one-cert-secret-certs-02",
			Namespace: instance4.Namespace,
			Labels: map[string]string{
				"rpaas.extensions.tsuru.io/certificate-name": "default",
				"rpaas.extensions.tsuru.io/instance-name":    "one-cert",
			},
		},
		Data: map[string][]byte{
			"tls.crt": []byte(rsaCertPem),
			"tls.key": []byte(rsaKeyPem),
		},
	}

	resources := []runtime.Object{instance1, instance2, instance3, instance4, secret1, secret2, secret3}

	tests := map[string]struct {
		certName      string
		instanceName  string
		expectedError string
		assert        func(t *testing.T, c client.Client)
	}{
		"instance not found": {
			instanceName:  "instance-not-found",
			expectedError: "rpaas instance \"instance-not-found\" not found",
		},

		"instance without certificates": {
			instanceName:  "no-spec-cert",
			expectedError: "no certificate bound to instance \"no-spec-cert\"",
		},

		"instance with only one, default certificate": {
			instanceName: "one-cert",
			assert: func(t *testing.T, c client.Client) {
				var instance v1alpha1.RpaasInstance
				err := c.Get(context.TODO(), types.NamespacedName{Name: "one-cert", Namespace: getServiceName()}, &instance)
				require.NoError(t, err)
				assert.Nil(t, instance.Spec.TLS)
			},
		},

		"instance with two certificates, should remain one": {
			instanceName: "another-instance",
			certName:     "junda",
			assert: func(t *testing.T, c client.Client) {
				var instance v1alpha1.RpaasInstance
				err := c.Get(context.TODO(), types.NamespacedName{Name: "another-instance", Namespace: getServiceName()}, &instance)
				require.NoError(t, err)
				assert.Equal(t, []nginxv1alpha1.NginxTLS{{SecretName: "another-instance-certs-01"}}, instance.Spec.TLS)
			},
		},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			client := fake.NewClientBuilder().
				WithScheme(newScheme()).
				WithRuntimeObjects(resources...).
				Build()

			err := (&k8sRpaasManager{cli: client}).DeleteCertificate(context.TODO(), tt.instanceName, tt.certName)
			if tt.expectedError != "" {
				assert.EqualError(t, err, tt.expectedError)
				return
			}

			require.NoError(t, err)
			require.NotNil(t, tt.assert, "you should pass an assert func")
			tt.assert(t, client)
		})
	}
}

func Test_k8sRpaasManager_UpdateCertificate(t *testing.T) {
	ecdsaCertPem := `-----BEGIN CERTIFICATE-----
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

	ecdsaKeyPem := `-----BEGIN EC PRIVATE KEY-----
MHcCAQEEIIrYSSNQFaA2Hwf1duRSxKtLYX5CB04fSeQ6tF1aY/PuoAoGCCqGSM49
AwEHoUQDQgAEPR3tU2Fta9ktY+6P9G0cWO+0kETA6SFs38GecTyudlHz6xvCdz8q
EKTcWGekdmdDPsHloRNtsiCa697B2O9IFA==
-----END EC PRIVATE KEY-----`

	ecdsaCertificate, err := tls.X509KeyPair([]byte(ecdsaCertPem), []byte(ecdsaKeyPem))
	require.NoError(t, err)

	rsaCertPem := `-----BEGIN CERTIFICATE-----
MIIEYTCCAsmgAwIBAgIRAMpoigdN2QozxdUrgn3FqScwDQYJKoZIhvcNAQELBQAw
gZExHjAcBgNVBAoTFW1rY2VydCBkZXZlbG9wbWVudCBDQTEzMDEGA1UECwwqd2ls
c29uLmp1bmlvckBGVkZGSzAxMVE3MlggKFdpbHNvbiBKdW5pb3IpMTowOAYDVQQD
DDFta2NlcnQgd2lsc29uLmp1bmlvckBGVkZGSzAxMVE3MlggKFdpbHNvbiBKdW5p
b3IpMB4XDTIxMDgyNzE4MTkzM1oXDTIzMTEyNzE4MTkzM1owXjEnMCUGA1UEChMe
bWtjZXJ0IGRldmVsb3BtZW50IGNlcnRpZmljYXRlMTMwMQYDVQQLDCp3aWxzb24u
anVuaW9yQEZWRkZLMDExUTcyWCAoV2lsc29uIEp1bmlvcikwggEiMA0GCSqGSIb3
DQEBAQUAA4IBDwAwggEKAoIBAQC1xLsiPA93LH+Xppz9MCgkAQEFzs0ajz+RKYww
W9VHebeQlV8l6oRcY6WdsdZvcM9HdUlbvcQEmHrDE1dcWuablAc39agafTuSMBh+
dlloZeM4z3lURP9lsf7uoLrw8aBbDZ9vkWr5XlqfROePjkMSUvTal4GwsodA/hcZ
xVVetmf/CLimVefv5n5tE2X4UK5G26AjJVCkufh0lJabwWd51XRBmoVBhLn60O34
lZOWsRKJHRn3Obwgg65pSIbwwIP6E95NUJvG+Z5xm7vSzXezJ+pUtdTvhRhnnvvl
w4BJEFcSKmVrYXnC6Zncb5weQ6cSmfL2uJFYAgQjA1WyVMmRAgMBAAGjZjBkMA4G
A1UdDwEB/wQEAwIFoDATBgNVHSUEDDAKBggrBgEFBQcDATAfBgNVHSMEGDAWgBSp
9D7s6aeJcjYxy4ql7ME3khsYyTAcBgNVHREEFTATghFycGFhcy1vcGVyYXRvci5p
bzANBgkqhkiG9w0BAQsFAAOCAYEAqwqqWb8JGQPP2ubzFktNdUMr2KeIhkiBAsdA
ff/QDZ4/5gzGw/irbsvPAowz7Y4Kn9vF7E2kfh5I8N6AQhVnrsAPmVH8PDdisf95
71pVSfGz71b5noe4rqbOT1WYCqdLjt5qht5LKHSVgtm2xOESyoDuCQd20Vj808xV
8vtsiK+B5TtD0V9v0Ckf9H0Ngk+jWSJLQUtODN8SSxzzMSBPIYoMQ6m5KcM6FKTW
5J5MiHER7NraW284CtDSOG/2DBjW/9+iTzDlBZhgzpmWHkUQpS2RSox/b+giiYaz
cbfqOhKusQonR7bcCyKphrAQkG/sjXJ6HcBj6WVQhVhrxhu939SWaJ3a6s35DHc+
p8/zWWtbEat9jrFT83ej8GB5RbyIHRRncHQ51ymM/bAW/F7G74mPPHVfK0Y1sNdY
ix3plWG7WNMHkxHT9IuU8/ieycCJp0jshm9obbM7MCMp3WrZmfUYq2cbuZiD0Upy
xbFwana3DCXVZv8lJl4vPiGRV2wK
-----END CERTIFICATE-----`

	rsaKeyPem := `-----BEGIN RSA PRIVATE KEY-----
MIIEpQIBAAKCAQEAtcS7IjwPdyx/l6ac/TAoJAEBBc7NGo8/kSmMMFvVR3m3kJVf
JeqEXGOlnbHWb3DPR3VJW73EBJh6wxNXXFrmm5QHN/WoGn07kjAYfnZZaGXjOM95
VET/ZbH+7qC68PGgWw2fb5Fq+V5an0Tnj45DElL02peBsLKHQP4XGcVVXrZn/wi4
plXn7+Z+bRNl+FCuRtugIyVQpLn4dJSWm8FnedV0QZqFQYS5+tDt+JWTlrESiR0Z
9zm8IIOuaUiG8MCD+hPeTVCbxvmecZu70s13syfqVLXU74UYZ5775cOASRBXEipl
a2F5wumZ3G+cHkOnEpny9riRWAIEIwNVslTJkQIDAQABAoIBAQCgOrKnZABSEmTL
HvEmc0v/KO9o2jqNHhdv9AsDGgCxSAdbqYC9YLSa6LA2iWy4wd3GZQpsl6RyVKNq
0OLio7LDFEMkviUYbKqVnFYLLHJ2B9K74pBWi5gDYluSqRcBjE3J1gFkjPCar+T8
nvXs4wAW6A+1nXaSd12pGhLtAbnWiUHMhMhBFWZkNDb4oGRhEhWDCpO6XtKtMmUg
Amo0E86OGqdyWwbmPjZC9TXvZODpn8pheoFQ2V13kHtMt3kXj3PxJjA6RKcy61Xz
IKn44W0Jb1ktXzf0kzro1z6byLMQA/Qg8v1Zchdk+UPfQiXKIm26iPZSdrKgGbVZ
4UVPvmTBAoGBAOQob/p9WSE4kYN58q+5HPC0g6+Mg2Tg3DiNlzvqtHLolaQgI8d7
meeiwwOGU80kMpLo7ScX+61Bx1ODPEA5xnUSbm4gaU/c8pKbUdaJaklCb924L7Gw
MtSiYPXhRveV3s6SuShec7SuAu0kT04NWuYnNnfdTn6oToOzH6DvQA7pAoGBAMvz
GuCi2XGGv8F+jD3VWrKkvwsq4h3o08g/wmCWZFVH2TVgc9eUajOFeRUjnHpWbp3l
CWgB1Z/e+s5K/mTTQT6hsX56QtPB2eDxzwUeXwMN3lQVvXljWfEnOMnFBAe6g49l
1DbkvU44jEFCFwDikfsO45cN7IZ7MqIl0DQBpcxpAoGBAJBzvbn1JLowy4hXbDyv
UdBgKcO6jfIPn746fxbTWZ4q/ZslMiH5co7CcP/JS0NleJOk34lR2Olv7RhFzZ7I
NYsnuT0GTkbfF8GUjvLqm514b8UL+T5h1Tzk9ciW8cyNWbymDo6thkpNpdKom4FK
WVPAXe7z8d+lBdjCTvMgpwkJAoGANopep7AlIjz8zsv+yRJjXN690Ei5i3IWILkc
TCQr1LqQFbwjfoVMGVcaWFLbp8OxdTwo1c2XyVciD0Ty3xe3nP40rzQW5vYyQ/um
dyH2GqT8zdO6hdnR1bG9eAfd2gtA33pF1CA7l817hIAeEriEfXUv29d3Z0dO9RnT
ofTG1/ECgYEA3IHIZS3zEMgKmMAAHme6UvxJxQOiz+CUAKQUUTB+3YSBT3NOtonV
sR2uspXuam+kC900f+vXJPVcNI4rtoSelYIbmdGt4Pn/TqUKGk1qRrs7paLI8Iw0
x2cJyBkkBQV9WB34oGtnZzQ0nKtzsY6FVlNGSeyCJ3OD2dHXO5komJY=
-----END RSA PRIVATE KEY-----`

	rsaCertificate, err := tls.X509KeyPair([]byte(rsaCertPem), []byte(rsaKeyPem))
	require.NoError(t, err)

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
				TLS: []nginxv1alpha1.NginxTLS{
					{SecretName: "my-instance-2-certs-abc123", Hosts: []string{"rpaas-operator.io"}},
				},
			},
		},
		&corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "my-instance-2-certs-abc123",
				Namespace: "rpaasv2",
				Labels: map[string]string{
					"rpaas.extensions.tsuru.io/certificate-name": "default",
					"rpaas.extensions.tsuru.io/instance-name":    "my-instance-2",
				},
			},
			Data: map[string][]byte{
				"tls.crt": []byte(rsaCertPem),
				"tls.key": []byte(rsaKeyPem),
			},
		},
	}

	tests := map[string]struct {
		instanceName    string
		certificateName string
		certificate     tls.Certificate
		expectedError   string
		assert          func(t *testing.T, c client.Client)
	}{
		"instance not found": {
			instanceName:  "instance-not-found",
			certificate:   ecdsaCertificate,
			expectedError: "rpaas instance \"instance-not-found\" not found",
		},

		"adding a new certificate without name, should use default name \"default\"": {
			instanceName: "my-instance-1",
			certificate:  rsaCertificate,
			assert: func(t *testing.T, c client.Client) {
				var instance v1alpha1.RpaasInstance
				err := c.Get(context.TODO(), types.NamespacedName{Name: "my-instance-1", Namespace: "rpaasv2"}, &instance)
				require.NoError(t, err)

				require.Len(t, instance.Spec.TLS, 1)
				tls := instance.Spec.TLS[0]
				assert.Regexp(t, `^my-instance-1-certs-`, tls.SecretName)
				assert.Equal(t, []string{"rpaas-operator.io"}, tls.Hosts)
			},
		},

		"adding a new certificate with duplicated DNS name": {
			instanceName:    "my-instance-2",
			certificateName: "lets-duplicate",
			certificate:     rsaCertificate,
			expectedError:   `certificate DNS name is forbidden: you cannot use a already used dns name, currently in use use in "default" certificate`,
		},

		"adding a new certificate with a custom name": {
			instanceName:    "my-instance-1",
			certificateName: "custom-name",
			certificate:     ecdsaCertificate,
			assert: func(t *testing.T, c client.Client) {
				var instance v1alpha1.RpaasInstance
				err := c.Get(context.TODO(), types.NamespacedName{Name: "my-instance-1", Namespace: getServiceName()}, &instance)
				require.NoError(t, err)
				assert.NotNil(t, instance.Spec.TLS)
			},
		},

		"updating an existing certificate from RSA to ECDSA": {
			instanceName: "my-instance-2",
			certificate:  ecdsaCertificate,
			assert: func(t *testing.T, c client.Client) {
				var instance v1alpha1.RpaasInstance
				err := c.Get(context.Background(), types.NamespacedName{Name: "my-instance-2", Namespace: getServiceName()}, &instance)
				require.NoError(t, err)
			},
		},

		"adding multiple certificates": {
			instanceName:    "my-instance-2",
			certificateName: "custom-name",
			certificate:     ecdsaCertificate,
			assert: func(t *testing.T, c client.Client) {
				var instance v1alpha1.RpaasInstance
				err := c.Get(context.Background(), types.NamespacedName{Name: "my-instance-2", Namespace: getServiceName()}, &instance)
				require.NoError(t, err)
				assert.NotNil(t, instance.Spec.TLS)
			},
		},

		"invalid certificate name": {
			instanceName:    "my-instance-1",
			certificate:     ecdsaCertificate,
			certificateName: `../not@valid.config_map~key`,
			expectedError:   `certificate name is not valid: a valid config key must consist of alphanumeric characters, '-', '_' or '.' (e.g. 'key.name',  or 'KEY_NAME',  or 'key-name', regex used for validation is '[-._a-zA-Z0-9]+')`,
		},

		`setting certificate with name "cert-manager"`: {
			instanceName:    "my-instance-1",
			certificate:     ecdsaCertificate,
			certificateName: "cert-manager",
			expectedError:   `certificate name is forbidden: name should not begin with "cert-manager"`,
		},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			client := fake.NewClientBuilder().
				WithScheme(newScheme()).
				WithRuntimeObjects(resources...).
				Build()

			err := (&k8sRpaasManager{cli: client}).UpdateCertificate(context.TODO(), tt.instanceName, tt.certificateName, tt.certificate)
			if tt.expectedError != "" {
				assert.EqualError(t, err, tt.expectedError)
				return
			}

			assert.NoError(t, err)
			tt.assert(t, client)
		})
	}
}

func newEmptyRpaasInstance() *v1alpha1.RpaasInstance {
	return &v1alpha1.RpaasInstance{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "extensions.tsuru.io/v1alpha1",
			Kind:       "RpaasInstance",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "my-instance",
			Namespace: getServiceName(),
		},
		Spec: v1alpha1.RpaasInstanceSpec{},
	}
}

func newEmptyLocations() *corev1.ConfigMap {
	return &corev1.ConfigMap{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "v1",
			Kind:       "ConfigMap",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "my-locations",
			Namespace: getServiceName(),
		},
	}
}

func Test_k8sRpaasManager_GetInstanceAddress(t *testing.T) {
	testCases := []struct {
		name      string
		resources func() []runtime.Object
		instance  string
		assertion func(*testing.T, string, error)
	}{
		{
			name: "when the Service has type LoadBalancer and already has an external IP, should returns the provided extenal IP",
			resources: func() []runtime.Object {
				instance := newEmptyRpaasInstance()
				return []runtime.Object{
					instance,
					&nginxv1alpha1.Nginx{
						ObjectMeta: metav1.ObjectMeta{
							Name:      instance.Name,
							Namespace: instance.Namespace,
						},
						Status: nginxv1alpha1.NginxStatus{
							Services: []nginxv1alpha1.ServiceStatus{
								{Name: instance.Name + "-service"},
							},
						},
					},
					&corev1.Service{
						ObjectMeta: metav1.ObjectMeta{
							Name:      instance.Name + "-service",
							Namespace: instance.Namespace,
						},
						Spec: corev1.ServiceSpec{
							Type:      corev1.ServiceTypeLoadBalancer,
							ClusterIP: "10.1.1.9",
						},
						Status: corev1.ServiceStatus{
							LoadBalancer: corev1.LoadBalancerStatus{
								Ingress: []corev1.LoadBalancerIngress{
									{IP: "10.1.2.3"},
								},
							},
						},
					},
				}
			},
			instance: "my-instance",
			assertion: func(t *testing.T, address string, err error) {
				assert.NoError(t, err)
				assert.Equal(t, address, "10.1.2.3")
			},
		},
		{
			name: "when the Service has type LoadBalancer and already has an external Hostname, but does not have an IP",
			resources: func() []runtime.Object {
				instance := newEmptyRpaasInstance()
				return []runtime.Object{
					instance,
					&nginxv1alpha1.Nginx{
						ObjectMeta: metav1.ObjectMeta{
							Name:      instance.Name,
							Namespace: instance.Namespace,
						},
						Status: nginxv1alpha1.NginxStatus{
							Services: []nginxv1alpha1.ServiceStatus{
								{Name: instance.Name + "-service"},
							},
						},
					},
					&corev1.Service{
						ObjectMeta: metav1.ObjectMeta{
							Name:      instance.Name + "-service",
							Namespace: instance.Namespace,
						},
						Spec: corev1.ServiceSpec{
							Type:      corev1.ServiceTypeLoadBalancer,
							ClusterIP: "10.1.1.9",
						},
						Status: corev1.ServiceStatus{
							LoadBalancer: corev1.LoadBalancerStatus{
								Ingress: []corev1.LoadBalancerIngress{
									{Hostname: "my-lb.my-provider.io"},
								},
							},
						},
					},
				}
			},
			instance: "my-instance",
			assertion: func(t *testing.T, address string, err error) {
				assert.NoError(t, err)
				assert.Equal(t, address, "my-lb.my-provider.io")
			},
		},
		{
			name: "when the Service has type LoadBalancer with no external IP provided, should returns an empty address",
			resources: func() []runtime.Object {
				instance := newEmptyRpaasInstance()
				return []runtime.Object{
					instance,
					&nginxv1alpha1.Nginx{
						ObjectMeta: metav1.ObjectMeta{
							Name:      instance.Name,
							Namespace: instance.Namespace,
						},
						Status: nginxv1alpha1.NginxStatus{
							Services: []nginxv1alpha1.ServiceStatus{
								{Name: instance.Name + "-service"},
							},
						},
					},
					&corev1.Service{
						ObjectMeta: metav1.ObjectMeta{
							Name:      instance.Name + "-service",
							Namespace: instance.Namespace,
						},
						Spec: corev1.ServiceSpec{
							Type:      corev1.ServiceTypeLoadBalancer,
							ClusterIP: "10.1.1.9",
						},
					},
				}
			},
			instance: "my-instance",
			assertion: func(t *testing.T, address string, err error) {
				assert.NoError(t, err)
				assert.Equal(t, address, "")
			},
		},
		{
			name: "when the Service is ClusterIP type, should returns the ClusterIP address",
			resources: func() []runtime.Object {
				instance := newEmptyRpaasInstance()
				instance.Name = "another-instance"
				return []runtime.Object{
					instance,
					&nginxv1alpha1.Nginx{
						ObjectMeta: metav1.ObjectMeta{
							Name:      instance.Name,
							Namespace: instance.Namespace,
						},
						Status: nginxv1alpha1.NginxStatus{
							Services: []nginxv1alpha1.ServiceStatus{
								{Name: instance.Name + "-service"},
							},
						},
					},
					&corev1.Service{
						ObjectMeta: metav1.ObjectMeta{
							Name:      instance.Name + "-service",
							Namespace: instance.Namespace,
						},
						Spec: corev1.ServiceSpec{
							Type:      corev1.ServiceTypeClusterIP,
							ClusterIP: "10.1.1.9",
						},
					},
				}
			},
			instance: "another-instance",
			assertion: func(t *testing.T, address string, err error) {
				assert.NoError(t, err)
				assert.Equal(t, address, "10.1.1.9")
			},
		},
		{
			name: "when Nginx object has no Services under Status field, should returns an empty address",
			resources: func() []runtime.Object {
				instance := newEmptyRpaasInstance()
				instance.Name = "instance3"
				return []runtime.Object{
					instance,
					&nginxv1alpha1.Nginx{
						ObjectMeta: metav1.ObjectMeta{
							Name:      instance.Name,
							Namespace: instance.Namespace,
						},
						Status: nginxv1alpha1.NginxStatus{},
					},
				}
			},
			instance: "instance3",
			assertion: func(t *testing.T, address string, err error) {
				assert.NoError(t, err)
				assert.Equal(t, address, "")
			},
		},
		{
			name: "when Nginx object is not found, should returns an empty address",
			resources: func() []runtime.Object {
				instance := newEmptyRpaasInstance()
				instance.Name = "instance4"
				return []runtime.Object{
					instance,
				}
			},
			instance: "instance4",
			assertion: func(t *testing.T, address string, err error) {
				assert.NoError(t, err)
				assert.Equal(t, address, "")
			},
		},
		{
			name: "when RpaasInstance is not found, should returns an NotFoundError",
			resources: func() []runtime.Object {
				return []runtime.Object{}
			},
			instance: "not-found-instance",
			assertion: func(t *testing.T, address string, err error) {
				assert.Error(t, err)
				assert.True(t, IsNotFoundError(err))
			},
		},
	}

	for _, tt := range testCases {
		t.Run(tt.name, func(t *testing.T) {
			manager := &k8sRpaasManager{
				cli: fake.NewClientBuilder().WithScheme(newScheme()).WithRuntimeObjects(tt.resources()...).Build(),
			}
			address, err := manager.GetInstanceAddress(context.Background(), tt.instance)
			tt.assertion(t, address, err)
		})
	}
}

func Test_k8sRpaasManager_GetInstanceStatus(t *testing.T) {
	instance1 := newEmptyRpaasInstance()
	instance2 := newEmptyRpaasInstance()
	instance2.ObjectMeta.Name = "instance2"
	instance3 := newEmptyRpaasInstance()
	instance3.ObjectMeta.Name = "instance3"
	instance4 := newEmptyRpaasInstance()
	instance4.ObjectMeta.Name = "instance4"
	instance5 := newEmptyRpaasInstance()
	instance5.ObjectMeta.Name = "instance5"

	nginx1 := &nginxv1alpha1.Nginx{
		ObjectMeta: instance1.ObjectMeta,
		Status: nginxv1alpha1.NginxStatus{
			PodSelector: "nginx.tsuru.io/app=nginx,nginx.tsuru.io/resource-name=my-instance",
		},
	}
	nginx2 := &nginxv1alpha1.Nginx{
		ObjectMeta: instance2.ObjectMeta,
		Status: nginxv1alpha1.NginxStatus{
			PodSelector: "nginx.tsuru.io/app=nginx,nginx.tsuru.io/resource-name=instance2",
		},
	}
	nginx3 := &nginxv1alpha1.Nginx{
		ObjectMeta: instance3.ObjectMeta,
		Status: nginxv1alpha1.NginxStatus{
			PodSelector: "nginx.tsuru.io/app=nginx,nginx.tsuru.io/resource-name=instance3",
		},
	}
	nginx4 := &nginxv1alpha1.Nginx{
		ObjectMeta: instance5.ObjectMeta,
		Status: nginxv1alpha1.NginxStatus{
			PodSelector: "nginx.tsuru.io/app=nginx,nginx.tsuru.io/resource-name=instance5",
		},
	}

	pod1 := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "pod1",
			Namespace: instance1.Namespace,
			Labels: map[string]string{
				"nginx.tsuru.io/app":           "nginx",
				"nginx.tsuru.io/resource-name": "my-instance",
			},
			UID: types.UID("pod1-uid"),
		},
		Status: corev1.PodStatus{
			PodIP: "10.0.0.1",
		},
	}
	pod2 := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "pod2",
			Namespace: instance1.Namespace,
			Labels: map[string]string{
				"nginx.tsuru.io/app":           "nginx",
				"nginx.tsuru.io/resource-name": "my-instance",
			},
			UID: types.UID("pod2-uid"),
		},
		Status: corev1.PodStatus{
			PodIP: "10.0.0.2",
		},
	}
	pod4 := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "pod4",
			Namespace: instance5.Namespace,
			Labels: map[string]string{
				"nginx.tsuru.io/app":           "nginx",
				"nginx.tsuru.io/resource-name": "instance5",
			},
			UID: types.UID("pod4-uid"),
		},
		Status: corev1.PodStatus{
			PodIP: "10.0.0.9",
			ContainerStatuses: []corev1.ContainerStatus{
				{
					Ready: false,
				},
			},
		},
	}
	evt1 := &corev1.Event{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "pod1.1",
			Namespace: instance1.Namespace,
		},
		InvolvedObject: corev1.ObjectReference{
			Kind: "Pod",
			UID:  pod1.ObjectMeta.UID,
			Name: pod1.ObjectMeta.Name,
		},
		Source: corev1.EventSource{
			Component: "c1",
		},
		Message: "msg1",
	}
	evt2 := &corev1.Event{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "pod1.2",
			Namespace: instance1.Namespace,
		},
		InvolvedObject: corev1.ObjectReference{
			UID:  pod1.ObjectMeta.UID,
			Name: pod1.ObjectMeta.Name,
			Kind: "Pod",
		},
		Source: corev1.EventSource{
			Component: "c2",
			Host:      "h1",
		},
		Message: "msg2",
	}

	resources := []runtime.Object{instance1, instance2, instance3, instance4, instance5, nginx1, nginx2, nginx3, nginx4, pod1, pod2, pod4, evt1, evt2}

	testCases := []struct {
		instance  string
		assertion func(*testing.T, PodStatusMap, error)
	}{
		{
			"my-instance",
			func(t *testing.T, podMap PodStatusMap, err error) {
				assert.NoError(t, err)
				assert.Equal(t, podMap, PodStatusMap{
					"pod1": PodStatus{
						Running: true,
						Status:  "msg1 [c1]\nmsg2 [c2, h1]",
						Address: "10.0.0.1",
					},
					"pod2": PodStatus{
						Running: true,
						Status:  "",
						Address: "10.0.0.2",
					},
				})
			},
		},
		{
			"instance3",
			func(t *testing.T, podMap PodStatusMap, err error) {
				assert.NoError(t, err)
				assert.Equal(t, podMap, PodStatusMap{})
			},
		},
		{
			"instance4",
			func(t *testing.T, podMap PodStatusMap, err error) {
				assert.Error(t, err)
				assert.True(t, IsNotFoundError(err))
			},
		},
		{
			"instance5",
			func(t *testing.T, podMap PodStatusMap, err error) {
				assert.NoError(t, err)
				assert.Equal(t, podMap, PodStatusMap{
					"pod4": PodStatus{
						Running: false,
						Address: "10.0.0.9",
					},
				})
			},
		},
		{
			"not-found-instance",
			func(t *testing.T, podMap PodStatusMap, err error) {
				assert.Error(t, err)
				assert.True(t, IsNotFoundError(err))
			},
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.instance, func(t *testing.T) {
			fakeCli := fake.NewClientBuilder().WithScheme(newScheme()).WithRuntimeObjects(resources...).Build()
			manager := &k8sRpaasManager{
				cli: fakeCli,
			}
			_, podMap, err := manager.GetInstanceStatus(context.Background(), testCase.instance)
			testCase.assertion(t, podMap, err)
		})
	}
}

func Test_k8sRpaasManager_PurgeCache(t *testing.T) {
	instance1 := newEmptyRpaasInstance()
	instance1.ObjectMeta.Name = "my-instance"
	instance2 := newEmptyRpaasInstance()
	instance2.ObjectMeta.Name = "not-running-instance"
	nginx1 := &nginxv1alpha1.Nginx{
		ObjectMeta: instance1.ObjectMeta,
		Status: nginxv1alpha1.NginxStatus{
			PodSelector: "nginx.tsuru.io/app=nginx,nginx.tsuru.io/resource-name=my-instance",
		},
	}
	nginx2 := &nginxv1alpha1.Nginx{
		ObjectMeta: instance2.ObjectMeta,
		Status: nginxv1alpha1.NginxStatus{
			PodSelector: "nginx.tsuru.io/app=nginx,nginx.tsuru.io/resource-name=not-running-instance",
		},
	}
	pod1 := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "my-instance-pod-1",
			Namespace: instance1.Namespace,
			Labels: map[string]string{
				"nginx.tsuru.io/app":           "nginx",
				"nginx.tsuru.io/resource-name": "my-instance",
			},
		},
		Status: corev1.PodStatus{
			PodIP:             "10.0.0.9",
			ContainerStatuses: []corev1.ContainerStatus{{Ready: true}},
		},
	}
	pod2 := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "my-instance-pod-2",
			Namespace: instance1.Namespace,
			Labels: map[string]string{
				"nginx.tsuru.io/app":           "nginx",
				"nginx.tsuru.io/resource-name": "my-instance",
			},
		},
		Status: corev1.PodStatus{
			PodIP:             "10.0.0.10",
			ContainerStatuses: []corev1.ContainerStatus{{Ready: true}},
		},
	}
	pod3 := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "not-running-instance-pod",
			Namespace: instance2.Namespace,
			Labels: map[string]string{
				"nginx.tsuru.io/app":           "nginx",
				"nginx.tsuru.io/resource-name": "not-running-instance",
			},
		},
		Status: corev1.PodStatus{
			PodIP:             "10.0.0.11",
			ContainerStatuses: []corev1.ContainerStatus{{Ready: false}},
		},
	}

	scheme := newScheme()
	resources := []runtime.Object{instance1, instance2, nginx1, nginx2, pod1, pod2, pod3}

	tests := []struct {
		name         string
		instance     string
		args         PurgeCacheArgs
		cacheManager fakeCacheManager
		assertion    func(t *testing.T, count int, err error)
	}{
		{
			name:         "return NotFoundError when instance is not found",
			instance:     "not-found-instance",
			args:         PurgeCacheArgs{Path: "/index.html"},
			cacheManager: fakeCacheManager{},
			assertion: func(t *testing.T, count int, err error) {
				assert.Error(t, err)
				expected := NotFoundError{Msg: "rpaas instance \"not-found-instance\" not found"}
				assert.Equal(t, expected, err)
			},
		},
		{
			name:         "return ValidationError path parameter was not provided",
			instance:     "my-instance",
			args:         PurgeCacheArgs{},
			cacheManager: fakeCacheManager{},
			assertion: func(t *testing.T, count int, err error) {
				assert.Error(t, err)
				expected := ValidationError{Msg: "path is required"}
				assert.Equal(t, expected, err)
			},
		},
		{
			name:         "return 0 when instance doesn't have any running pods",
			instance:     "not-running-instance",
			args:         PurgeCacheArgs{Path: "/index.html"},
			cacheManager: fakeCacheManager{},
			assertion: func(t *testing.T, count int, err error) {
				assert.NoError(t, err)
				assert.Equal(t, 0, count)
			},
		},
		{
			name:         "return the number of unsuccessfully purged instances",
			instance:     "my-instance",
			args:         PurgeCacheArgs{Path: "/index.html"},
			cacheManager: fakeCacheManager{},
			assertion: func(t *testing.T, count int, err error) {
				assert.NoError(t, err)
				assert.Equal(t, 0, count)
			},
		},
		{
			name:     "return the number of nginx instances where cache was purged and error",
			instance: "my-instance",
			args:     PurgeCacheArgs{Path: "/index.html"},
			cacheManager: fakeCacheManager{
				purgeCacheFunc: func(host, path string, port int32, preservePath bool, extraHeaders http.Header) (bool, error) {
					if host == "10.0.0.9" {
						return false, nginxManager.NginxError{Msg: "some nginx error"}
					}
					return true, nil
				},
			},
			assertion: func(t *testing.T, count int, err error) {
				assert.EqualError(t, err, "1 error occurred:\n\t* pod 10.0.0.9 failed: some nginx error\n\n")
				assert.Equal(t, 1, count)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fakeCli := fake.NewClientBuilder().WithScheme(scheme).WithRuntimeObjects(resources...).Build()
			manager := &k8sRpaasManager{
				cli:          fakeCli,
				cacheManager: tt.cacheManager,
			}
			count, err := manager.PurgeCache(context.Background(), tt.instance, tt.args)
			tt.assertion(t, count, err)
		})
	}
}

func Test_k8sRpaasManager_BindApp(t *testing.T) {
	instance1 := newEmptyRpaasInstance()

	instance2 := newEmptyRpaasInstance()
	instance2.Name = "another-instance"
	instance2.Spec.Binds = []v1alpha1.Bind{{Host: "app2.tsuru.example.com"}}

	instance3 := newEmptyRpaasInstance()
	instance3.Name = "clustered-instance"
	instance3.ObjectMeta.Labels = map[string]string{
		"rpaas.extensions.tsuru.io/cluster-name": "cluster-01",
	}

	scheme := newScheme()
	resources := []runtime.Object{instance1, instance2, instance3}

	tests := []struct {
		name      string
		instance  string
		args      BindAppArgs
		assertion func(t *testing.T, err error, got v1alpha1.RpaasInstance)
	}{
		{
			name:     "when instance not found",
			instance: "not-found-instance",
			assertion: func(t *testing.T, err error, _ v1alpha1.RpaasInstance) {
				assert.Error(t, err)
				expected := NotFoundError{Msg: "rpaas instance \"not-found-instance\" not found"}
				assert.Equal(t, expected, err)
			},
		},
		{
			name:     "when AppHosts field is not defined",
			instance: "my-instance",
			args:     BindAppArgs{},
			assertion: func(t *testing.T, err error, _ v1alpha1.RpaasInstance) {
				assert.Error(t, err)
				expected := &ValidationError{Msg: "application hosts cannot be empty"}
				assert.Equal(t, expected, err)
			},
		},
		{
			name:     "when AppHosts field is blank",
			instance: "my-instance",
			args: BindAppArgs{
				AppHosts: []string{""},
			},
			assertion: func(t *testing.T, err error, _ v1alpha1.RpaasInstance) {
				assert.Error(t, err)
				expected := &ValidationError{Msg: "application hosts cannot be empty"}
				assert.Equal(t, expected, err)
			},
		},
		{
			name:     "when instance successfully bound with an application",
			instance: "my-instance",
			args: BindAppArgs{
				AppHosts: []string{
					"app1.tsuru.example.com",
				},
			},
			assertion: func(t *testing.T, err error, ri v1alpha1.RpaasInstance) {
				assert.NoError(t, err)
				assert.Equal(t, "app1.tsuru.example.com", ri.Spec.Binds[0].Host)
			},
		},
		{
			name:     "when instance already bound with another application",
			instance: "another-instance",
			args: BindAppArgs{
				AppHosts: []string{
					"app1.tsuru.example.com",
				},
			},
			assertion: func(t *testing.T, err error, ri v1alpha1.RpaasInstance) {
				assert.NoError(t, err)
				app1 := ri.Spec.Binds[0]
				assert.Equal(t, "app2.tsuru.example.com", app1.Host)
				app2 := ri.Spec.Binds[1]
				assert.Equal(t, "app1.tsuru.example.com", app2.Host)
			},
		},
		{
			name:     "when instance already bound with the application",
			instance: "another-instance",
			args: BindAppArgs{
				AppHosts: []string{
					"app2.tsuru.example.com",
				},
			},
			assertion: func(t *testing.T, err error, ri v1alpha1.RpaasInstance) {
				assert.Error(t, err)
				expected := &ConflictError{Msg: "instance already bound with this application"}
				assert.Equal(t, expected, err)
			},
		},

		{
			name:     "when clustered application bound in same cluster",
			instance: "clustered-instance",
			args: BindAppArgs{
				AppHosts: []string{
					"app2.tsuru.example.com",
				},
				AppInternalHosts: []string{
					"tcp://app2.example.cluster.svc.local:8888",
				},
				AppClusterName: "cluster-01",
			},
			assertion: func(t *testing.T, err error, ri v1alpha1.RpaasInstance) {
				assert.NoError(t, err)
				app1 := ri.Spec.Binds[0]
				assert.Equal(t, "app2.example.cluster.svc.local:8888", app1.Host)
			},
		},

		{
			name:     "when clustered application bound from other cluster",
			instance: "clustered-instance",
			args: BindAppArgs{
				AppHosts: []string{
					"app2.tsuru.example.com",
				},
				AppInternalHosts: []string{
					"tcp://app2.example.cluster.svc.local:8888",
				},
				AppClusterName: "cluster-02",
			},
			assertion: func(t *testing.T, err error, ri v1alpha1.RpaasInstance) {
				assert.NoError(t, err)
				app1 := ri.Spec.Binds[0]
				assert.Equal(t, "app2.tsuru.example.com", app1.Host)
			},
		},

		{
			name:     "when clustered application bound with no internal addresses",
			instance: "clustered-instance",
			args: BindAppArgs{
				AppHosts: []string{
					"app2.tsuru.example.com",
				},
				AppInternalHosts: []string{},
				AppClusterName:   "cluster-01",
			},
			assertion: func(t *testing.T, err error, ri v1alpha1.RpaasInstance) {
				assert.Error(t, err)
				expected := &ValidationError{Msg: "application internal hosts cannot be empty"}
				assert.Equal(t, expected, err)
			},
		},

		{
			name:     "when clustered application bound with a blank internal addresses",
			instance: "clustered-instance",
			args: BindAppArgs{
				AppHosts: []string{
					"app2.tsuru.example.com",
				},
				AppInternalHosts: []string{
					"",
				},
				AppClusterName: "cluster-01",
			},
			assertion: func(t *testing.T, err error, ri v1alpha1.RpaasInstance) {
				assert.Error(t, err)
				expected := &ValidationError{Msg: "application internal hosts cannot be empty"}
				assert.Equal(t, expected, err)
			},
		},

		{
			name:     "when clustered application bound with udp service",
			instance: "clustered-instance",
			args: BindAppArgs{
				AppHosts: []string{
					"app2.tsuru.example.com",
				},
				AppInternalHosts: []string{
					"udp://app2.example.svc.cluster.local:4000",
				},
				AppClusterName: "cluster-01",
			},
			assertion: func(t *testing.T, err error, ri v1alpha1.RpaasInstance) {
				assert.Error(t, err)
				expected := &ValidationError{Msg: "Unsupported host: \"udp://app2.example.svc.cluster.local:4000\""}
				assert.Equal(t, expected, err)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			manager := &k8sRpaasManager{cli: fake.NewClientBuilder().WithScheme(scheme).WithRuntimeObjects(resources...).Build()}
			bindAppErr := manager.BindApp(context.Background(), tt.instance, tt.args)

			var instance v1alpha1.RpaasInstance

			if bindAppErr == nil {
				require.NoError(t, manager.cli.Get(context.Background(), types.NamespacedName{
					Name:      tt.instance,
					Namespace: getServiceName(),
				}, &instance))
			}

			tt.assertion(t, bindAppErr, instance)
		})
	}
}

func Test_k8sRpaasManager_UnbindApp(t *testing.T) {
	instance1 := newEmptyRpaasInstance()

	instance2 := newEmptyRpaasInstance()
	instance2.Name = "another-instance"
	instance2.Spec.Binds = make([]v1alpha1.Bind, 1)
	instance2.Spec.Binds[0] = v1alpha1.Bind{Host: "app2.tsuru.example.com", Name: "app2"}

	instance3 := newEmptyRpaasInstance()
	instance3.Name = "instance-with-two-apps"
	instance3.Spec.Binds = make([]v1alpha1.Bind, 2)
	instance3.Spec.Binds[0] = v1alpha1.Bind{Host: "app2.tsuru.example.com", Name: "app2"}
	instance3.Spec.Binds[1] = v1alpha1.Bind{Host: "app3.tsuru.example.com", Name: "app3"}

	scheme := newScheme()
	resources := []runtime.Object{instance1, instance2, instance3}

	tests := []struct {
		name      string
		appName   string
		instance  string
		assertion func(t *testing.T, err error, got v1alpha1.RpaasInstance)
	}{
		{
			name:     "when instance not found",
			instance: "not-found-instance",
			appName:  "",
			assertion: func(t *testing.T, err error, _ v1alpha1.RpaasInstance) {
				assert.Error(t, err)
				expected := NotFoundError{Msg: "rpaas instance \"not-found-instance\" not found"}
				assert.Equal(t, expected, err)
			},
		},
		{
			name:     "when instance is not bound to an application",
			instance: "my-instance",
			appName:  "fake-app",
			assertion: func(t *testing.T, err error, _ v1alpha1.RpaasInstance) {
				assert.Error(t, err)
				expected := &NotFoundError{Msg: "app not found in instance bind list"}
				assert.Equal(t, expected, err)
			},
		},
		{
			name:     "when no app name is specified",
			instance: "my-instance",
			appName:  "",
			assertion: func(t *testing.T, err error, _ v1alpha1.RpaasInstance) {
				assert.Error(t, err)
				expected := &ValidationError{Msg: "must specify an app name"}
				assert.Equal(t, expected, err)
			},
		},

		{
			name:     "when instance is successfully unbound",
			instance: "another-instance",
			appName:  "app2",
			assertion: func(t *testing.T, err error, ri v1alpha1.RpaasInstance) {
				assert.NoError(t, err)
				assert.Equal(t, 0, len(ri.Spec.Binds))
			},
		},
		{
			name:     "instance bound with two apps, remaining app should become default",
			instance: "instance-with-two-apps",
			appName:  "app2",
			assertion: func(t *testing.T, err error, ri v1alpha1.RpaasInstance) {
				assert.NoError(t, err)
				assert.Equal(t, 1, len(ri.Spec.Binds))
				assert.Equal(t, "app3.tsuru.example.com", ri.Spec.Binds[0].Host)
				assert.Equal(t, "app3", ri.Spec.Binds[0].Name)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			manager := &k8sRpaasManager{cli: fake.NewClientBuilder().WithScheme(scheme).WithRuntimeObjects(resources...).Build()}
			unbindAppErr := manager.UnbindApp(context.Background(), tt.instance, tt.appName)

			var instance v1alpha1.RpaasInstance

			if unbindAppErr == nil {
				require.NoError(t, manager.cli.Get(context.Background(), types.NamespacedName{
					Name:      tt.instance,
					Namespace: getServiceName(),
				}, &instance))
			}

			tt.assertion(t, unbindAppErr, instance)
		})
	}
}

func Test_k8sRpaasManager_DeleteRoute(t *testing.T) {
	instance1 := newEmptyRpaasInstance()

	instance2 := newEmptyRpaasInstance()
	instance2.Name = "another-instance"
	instance2.Spec.Locations = []v1alpha1.Location{
		{
			Path: "/path1",
			Content: &v1alpha1.Value{
				Value: "# My NGINX config for /path1 location",
			},
		},
		{
			Path:        "/path2",
			Destination: "app2.tsuru.example.com",
		},
	}

	instance3 := newEmptyRpaasInstance()
	instance3.Name = "new-instance"
	instance3.Spec.Locations = []v1alpha1.Location{
		{
			Path: "/my/custom/path",
			Content: &v1alpha1.Value{
				ValueFrom: &v1alpha1.ValueSource{
					ConfigMapKeyRef: &corev1.ConfigMapKeySelector{
						LocalObjectReference: corev1.LocalObjectReference{
							Name: "my-custom-config",
						},
						Key: "just-another-key",
					},
				},
			},
		},
	}

	cm := newEmptyLocations()
	cm.Name = "my-locations-config"
	cm.Data = map[string]string{
		"just-another-key": "# Some NGINX custom conf",
	}

	scheme := newScheme()
	resources := []runtime.Object{instance1, instance2, instance3}

	tests := []struct {
		name      string
		instance  string
		path      string
		assertion func(t *testing.T, err error, ri *v1alpha1.RpaasInstance)
	}{
		{
			name:     "when instance not found",
			instance: "not-found-instance",
			path:     "/path",
			assertion: func(t *testing.T, err error, _ *v1alpha1.RpaasInstance) {
				assert.Error(t, err)
				assert.True(t, IsNotFoundError(err))
			},
		},
		{
			name:     "when locations is nil",
			instance: "my-instance",
			path:     "/path/unknown",
			assertion: func(t *testing.T, err error, _ *v1alpha1.RpaasInstance) {
				assert.Error(t, err)
				assert.True(t, IsNotFoundError(err))
				assert.Equal(t, &NotFoundError{Msg: "path does not exist"}, err)
			},
		},
		{
			name:     "when path does not exist",
			instance: "my-instance",
			path:     "/path/unknown",
			assertion: func(t *testing.T, err error, _ *v1alpha1.RpaasInstance) {
				assert.Error(t, err)
				assert.True(t, IsNotFoundError(err))
				assert.Equal(t, &NotFoundError{Msg: "path does not exist"}, err)
			},
		},
		{
			name:     "when removing a route with destination",
			instance: "another-instance",
			path:     "/path2",
			assertion: func(t *testing.T, err error, ri *v1alpha1.RpaasInstance) {
				assert.NoError(t, err)
				assert.Len(t, ri.Spec.Locations, 1)
				assert.NotEqual(t, "/path2", ri.Spec.Locations[0].Path)
			},
		},
		{
			name:     "when removing a route with custom configuration",
			instance: "another-instance",
			path:     "/path1",
			assertion: func(t *testing.T, err error, ri *v1alpha1.RpaasInstance) {
				assert.NoError(t, err)
				assert.Len(t, ri.Spec.Locations, 1)
				assert.NotEqual(t, "/path1", ri.Spec.Locations[0])
			},
		},
		{
			name:     "when removing the last location",
			instance: "new-instance",
			path:     "/my/custom/path",
			assertion: func(t *testing.T, err error, ri *v1alpha1.RpaasInstance) {
				assert.NoError(t, err)
				assert.Nil(t, ri.Spec.Locations)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			manager := &k8sRpaasManager{cli: fake.NewClientBuilder().WithScheme(scheme).WithRuntimeObjects(resources...).Build()}
			err := manager.DeleteRoute(context.Background(), tt.instance, tt.path)
			var ri v1alpha1.RpaasInstance
			if err == nil {
				require.NoError(t, manager.cli.Get(context.Background(), types.NamespacedName{Name: tt.instance, Namespace: getServiceName()}, &ri))
			}
			tt.assertion(t, err, &ri)
		})
	}
}

func Test_k8sRpaasManager_GetRoutes(t *testing.T) {
	boolPointer := func(b bool) *bool {
		return &b
	}

	instance1 := newEmptyRpaasInstance()

	instance2 := newEmptyRpaasInstance()
	instance2.Name = "another-instance"
	instance2.Spec.Locations = []v1alpha1.Location{
		{
			Path: "/path1",
			Content: &v1alpha1.Value{
				Value: "# My NGINX config for /path1 location",
			},
		},
		{
			Path:        "/path2",
			Destination: "app2.tsuru.example.com",
		},
		{
			Path:        "/path3",
			Destination: "app3.tsuru.example.com",
			ForceHTTPS:  true,
		},
		{
			Path: "/path4",
			Content: &v1alpha1.Value{
				ValueFrom: &v1alpha1.ValueSource{
					ConfigMapKeyRef: &corev1.ConfigMapKeySelector{
						LocalObjectReference: corev1.LocalObjectReference{
							Name: "my-locations",
						},
						Key: "path4",
					},
					Namespace: getServiceName(),
				},
			},
		},
		{
			Path: "/path5",
			Content: &v1alpha1.Value{
				ValueFrom: &v1alpha1.ValueSource{
					ConfigMapKeyRef: &corev1.ConfigMapKeySelector{
						LocalObjectReference: corev1.LocalObjectReference{
							Name: "unknown-configmap",
						},
						Key: "path4",
					},
					Namespace: getServiceName(),
				},
			},
		},
		{
			Path: "/path6",
			Content: &v1alpha1.Value{
				ValueFrom: &v1alpha1.ValueSource{
					ConfigMapKeyRef: &corev1.ConfigMapKeySelector{
						LocalObjectReference: corev1.LocalObjectReference{
							Name: "my-locations",
						},
						Key: "unknown-key",
					},
					Namespace: getServiceName(),
				},
			},
		},
	}

	instance3 := newEmptyRpaasInstance()
	instance3.Name = "instance3"
	instance3.Spec.Locations = []v1alpha1.Location{
		{
			Path: "/path1",
			Content: &v1alpha1.Value{
				ValueFrom: &v1alpha1.ValueSource{
					ConfigMapKeyRef: &corev1.ConfigMapKeySelector{
						LocalObjectReference: corev1.LocalObjectReference{
							Name: "my-locations",
						},
						Key:      "unknown-key",
						Optional: boolPointer(false),
					},
					Namespace: getServiceName(),
				},
			},
		},
	}

	instance4 := newEmptyRpaasInstance()
	instance4.Name = "instance4"
	instance4.Spec.Locations = []v1alpha1.Location{
		{
			Path: "/path1",
			Content: &v1alpha1.Value{
				ValueFrom: &v1alpha1.ValueSource{
					ConfigMapKeyRef: &corev1.ConfigMapKeySelector{
						LocalObjectReference: corev1.LocalObjectReference{
							Name: "unknown-configmap",
						},
						Key:      "unknown-key",
						Optional: boolPointer(false),
					},
					Namespace: getServiceName(),
				},
			},
		},
	}

	cm := newEmptyLocations()
	cm.Name = "my-locations"
	cm.Data = map[string]string{
		"path4": "# My NGINX config for /path4 location",
	}

	scheme := newScheme()
	resources := []runtime.Object{instance1, instance2, instance3, instance4, cm}

	tests := []struct {
		name      string
		instance  string
		assertion func(t *testing.T, err error, routes []Route)
	}{
		{
			name:     "when instance not found",
			instance: "not-found-instance",
			assertion: func(t *testing.T, err error, _ []Route) {
				assert.Error(t, err)
				assert.True(t, IsNotFoundError(err))
			},
		},
		{
			name:     "when instance has no custom routes",
			instance: "my-instance",
			assertion: func(t *testing.T, err error, routes []Route) {
				assert.NoError(t, err)
				assert.Len(t, routes, 0)
			},
		},
		{
			name:     "when instance contains multiple locations and with content comes from different sources",
			instance: "another-instance",
			assertion: func(t *testing.T, err error, routes []Route) {
				assert.NoError(t, err)
				assert.Equal(t, []Route{
					{
						Path:    "/path1",
						Content: "# My NGINX config for /path1 location",
					},
					{
						Path:        "/path2",
						Destination: "app2.tsuru.example.com",
					},
					{
						Path:        "/path3",
						Destination: "app3.tsuru.example.com",
						HTTPSOnly:   true,
					},
					{
						Path:    "/path4",
						Content: "# My NGINX config for /path4 location",
					},
				}, routes)
			},
		},
		{
			name:     "when a required value is not in the ConfigMap",
			instance: "instance3",
			assertion: func(t *testing.T, err error, routes []Route) {
				assert.Error(t, err)
			},
		},
		{
			name:     "when a ConfigMap of a required value is not found",
			instance: "instance4",
			assertion: func(t *testing.T, err error, routes []Route) {
				assert.Error(t, err)
				assert.True(t, k8sErrors.IsNotFound(err))
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			manager := &k8sRpaasManager{cli: fake.NewClientBuilder().WithScheme(scheme).WithRuntimeObjects(resources...).Build()}
			routes, err := manager.GetRoutes(context.Background(), tt.instance)
			tt.assertion(t, err, routes)
		})
	}
}

func Test_k8sRpaasManager_UpdateRoute(t *testing.T) {
	instance1 := newEmptyRpaasInstance()

	instance2 := newEmptyRpaasInstance()
	instance2.Name = "another-instance"
	instance2.Spec.Locations = []v1alpha1.Location{
		{
			Path: "/path1",
			Content: &v1alpha1.Value{
				Value: "# My NGINX config for /path1 location",
			},
		},
		{
			Path:        "/path2",
			Destination: "app2.tsuru.example.com",
		},
		{
			Path:        "/path3",
			Destination: "app2.tsuru.example.com",
			ForceHTTPS:  true,
		},
		{
			Path: "/path4",
			Content: &v1alpha1.Value{
				ValueFrom: &v1alpha1.ValueSource{
					ConfigMapKeyRef: &corev1.ConfigMapKeySelector{
						LocalObjectReference: corev1.LocalObjectReference{
							Name: "my-locations",
						},
						Key: "path1",
					},
				},
			},
		},
	}

	cm := newEmptyLocations()
	cm.Name = "another-instance-locations"
	cm.Data = map[string]string{
		"_path1": "# My NGINX config for /path1 location",
	}

	scheme := newScheme()
	resources := []runtime.Object{instance1, instance2, cm}

	config.Set(config.RpaasConfig{
		ConfigDenyPatterns: []regexp.Regexp{
			*regexp.MustCompile(`forbidden1.*?2`),
			*regexp.MustCompile(`forbidden2.*?3`),
		},
	})
	defer config.Set(config.RpaasConfig{})

	tests := []struct {
		name      string
		instance  string
		route     Route
		assertion func(t *testing.T, err error, ri *v1alpha1.RpaasInstance, locations *corev1.ConfigMap)
	}{
		{
			name:     "when instance not found",
			instance: "instance-not-found",
			assertion: func(t *testing.T, err error, _ *v1alpha1.RpaasInstance, _ *corev1.ConfigMap) {
				assert.Error(t, err)
				assert.True(t, IsNotFoundError(err))
			},
		},
		{
			name:     "when path is not defined",
			instance: "my-instance",
			assertion: func(t *testing.T, err error, _ *v1alpha1.RpaasInstance, _ *corev1.ConfigMap) {
				assert.Error(t, err)
				assert.True(t, IsValidationError(err))
				assert.Equal(t, &ValidationError{Msg: "path is required"}, err)
			},
		},
		{
			name:     "when path is not valid",
			instance: "my-instance",
			route: Route{
				Path: "../../passwd",
			},
			assertion: func(t *testing.T, err error, _ *v1alpha1.RpaasInstance, _ *corev1.ConfigMap) {
				assert.Error(t, err)
				assert.True(t, IsValidationError(err))
				assert.Equal(t, &ValidationError{Msg: "invalid path format"}, err)
			},
		},
		{
			name:     "when both content and destination are not defined",
			instance: "my-instance",
			route: Route{
				Path: "/my/custom/path",
			},
			assertion: func(t *testing.T, err error, _ *v1alpha1.RpaasInstance, _ *corev1.ConfigMap) {
				assert.Error(t, err)
				assert.True(t, IsValidationError(err))
				assert.Equal(t, &ValidationError{Msg: "either content or destination are required"}, err)
			},
		},
		{
			name:     "when content and destination are defined at same time",
			instance: "my-instance",
			route: Route{
				Path:        "/my/custom/path",
				Destination: "app2.tsuru.example.com",
				Content:     "# My NGINX config at location context",
			},
			assertion: func(t *testing.T, err error, _ *v1alpha1.RpaasInstance, _ *corev1.ConfigMap) {
				assert.Error(t, err)
				assert.True(t, IsValidationError(err))
				assert.Equal(t, &ValidationError{Msg: "cannot set both content and destination"}, err)
			},
		},
		{
			name:     "when content and httpsOnly are defined at same time",
			instance: "my-instance",
			route: Route{
				Path:      "/my/custom/path",
				Content:   "# My NGINX config",
				HTTPSOnly: true,
			},
			assertion: func(t *testing.T, err error, _ *v1alpha1.RpaasInstance, _ *corev1.ConfigMap) {
				assert.Error(t, err)
				assert.True(t, IsValidationError(err))
				assert.Equal(t, &ValidationError{Msg: "cannot set both content and httpsonly"}, err)
			},
		},
		{
			name:     "when adding a new route with destination and httpsOnly",
			instance: "my-instance",
			route: Route{
				Path:        "/my/custom/path",
				Destination: "app2.tsuru.example.com",
				HTTPSOnly:   true,
			},
			assertion: func(t *testing.T, err error, ri *v1alpha1.RpaasInstance, _ *corev1.ConfigMap) {
				assert.NoError(t, err)
				assert.Equal(t, []v1alpha1.Location{
					{
						Path:        "/my/custom/path",
						Destination: "app2.tsuru.example.com",
						ForceHTTPS:  true,
					},
				}, ri.Spec.Locations)
			},
		},
		{
			name:     "when adding a route with custom NGINX config",
			instance: "my-instance",
			route: Route{
				Path:    "/custom/path",
				Content: "# My custom NGINX config",
			},
			assertion: func(t *testing.T, err error, ri *v1alpha1.RpaasInstance, _ *corev1.ConfigMap) {
				assert.NoError(t, err)
				assert.Len(t, ri.Spec.Locations, 1)
				assert.Equal(t, "/custom/path", ri.Spec.Locations[0].Path)
				assert.Equal(t, "# My custom NGINX config", ri.Spec.Locations[0].Content.Value)
			},
		},
		{
			name:     "when updating destination and httpsOnly fields of an existing route",
			instance: "another-instance",
			route: Route{
				Path:        "/path2",
				Destination: "another-app.tsuru.example.com",
				HTTPSOnly:   true,
			},
			assertion: func(t *testing.T, err error, ri *v1alpha1.RpaasInstance, locations *corev1.ConfigMap) {
				assert.NoError(t, err)
				assert.Len(t, ri.Spec.Locations, 4)
				assert.Equal(t, v1alpha1.Location{
					Path:        "/path2",
					Destination: "another-app.tsuru.example.com",
					ForceHTTPS:  true,
				}, ri.Spec.Locations[1])
			},
		},
		{
			name:     "when updating the NGINX configuration content",
			instance: "another-instance",
			route: Route{
				Path:    "/path1",
				Content: "# My new NGINX configuration",
			},
			assertion: func(t *testing.T, err error, ri *v1alpha1.RpaasInstance, locations *corev1.ConfigMap) {
				assert.NoError(t, err)
				assert.Equal(t, v1alpha1.Location{
					Path: "/path1",
					Content: &v1alpha1.Value{
						Value: "# My new NGINX configuration",
					},
				}, ri.Spec.Locations[0])
			},
		},
		{
			name:     "when updating a route to use destination instead of content",
			instance: "another-instance",
			route: Route{
				Path:        "/path1",
				Destination: "app1.tsuru.example.com",
				HTTPSOnly:   true,
			},
			assertion: func(t *testing.T, err error, ri *v1alpha1.RpaasInstance, _ *corev1.ConfigMap) {
				assert.NoError(t, err)
				assert.Equal(t, v1alpha1.Location{
					Path:        "/path1",
					Destination: "app1.tsuru.example.com",
					ForceHTTPS:  true,
				}, ri.Spec.Locations[0])
			},
		},
		{
			name:     "when updating a route to use destination instead of content",
			instance: "another-instance",
			route: Route{
				Path:    "/path2",
				Content: "# My new NGINX configuration",
			},
			assertion: func(t *testing.T, err error, ri *v1alpha1.RpaasInstance, _ *corev1.ConfigMap) {
				assert.NoError(t, err)
				assert.Equal(t, v1alpha1.Location{
					Path: "/path2",
					Content: &v1alpha1.Value{
						Value: "# My new NGINX configuration",
					},
				}, ri.Spec.Locations[1])
			},
		},
		{
			name:     "when updating a route which its Content was into ConfigMap",
			instance: "another-instance",
			route: Route{
				Path:    "/path4",
				Content: "# My new NGINX configuration",
			},
			assertion: func(t *testing.T, err error, ri *v1alpha1.RpaasInstance, _ *corev1.ConfigMap) {
				assert.NoError(t, err)
				assert.Equal(t, v1alpha1.Location{
					Path: "/path4",
					Content: &v1alpha1.Value{
						Value: "# My new NGINX configuration",
					},
				}, ri.Spec.Locations[3])
			},
		},
		{
			name:     "when updating a route which its Content contains forbidden patterns",
			instance: "another-instance",
			route: Route{
				Path:    "/",
				Content: "# My new NGINX configuration\na forbidden2abc3 other\ntest",
			},
			assertion: func(t *testing.T, err error, ri *v1alpha1.RpaasInstance, _ *corev1.ConfigMap) {
				assert.EqualError(t, err, `content contains the forbidden pattern "forbidden2.*?3"`)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			manager := &k8sRpaasManager{cli: fake.NewClientBuilder().WithScheme(scheme).WithRuntimeObjects(resources...).Build()}
			err := manager.UpdateRoute(context.Background(), tt.instance, tt.route)
			ri := &v1alpha1.RpaasInstance{}
			if err == nil {
				newErr := manager.cli.Get(context.Background(), types.NamespacedName{Name: tt.instance, Namespace: getServiceName()}, ri)
				require.NoError(t, newErr)
			}
			tt.assertion(t, err, ri, nil)
		})
	}
}

func Test_getPlan(t *testing.T) {
	tests := []struct {
		name      string
		plan      string
		resources []runtime.Object
		assertion func(t *testing.T, err error, p *v1alpha1.RpaasPlan)
	}{
		{
			name:      "when plan does not exist",
			plan:      "unknown-plan",
			resources: []runtime.Object{},
			assertion: func(t *testing.T, err error, p *v1alpha1.RpaasPlan) {
				assert.Error(t, err)
				assert.Equal(t, NotFoundError{Msg: "plan \"unknown-plan\" not found"}, err)
			},
		},
		{
			name: "when plan is found by name",
			plan: "xxl",
			resources: []runtime.Object{
				&v1alpha1.RpaasPlan{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "xxl",
						Namespace: getServiceName(),
					},
				},
			},
			assertion: func(t *testing.T, err error, p *v1alpha1.RpaasPlan) {
				assert.NoError(t, err)
				assert.NotNil(t, p)
				assert.Equal(t, p.Name, "xxl")
			},
		},
		{
			name: "when plan is not set and there is a default plan",
			resources: []runtime.Object{
				&v1alpha1.RpaasPlan{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "some-default-plan",
						Namespace: getServiceName(),
					},
					Spec: v1alpha1.RpaasPlanSpec{
						Default: true,
					},
				},
			},
			assertion: func(t *testing.T, err error, p *v1alpha1.RpaasPlan) {
				assert.NoError(t, err)
				assert.NotNil(t, p)
				assert.Equal(t, p.Name, "some-default-plan")
			},
		},
		{
			name: "when plan is not set and there is no default plan",
			resources: []runtime.Object{
				&v1alpha1.RpaasPlan{
					ObjectMeta: metav1.ObjectMeta{Name: "plan1"},
				},
				&v1alpha1.RpaasPlan{
					ObjectMeta: metav1.ObjectMeta{Name: "plan2"},
				},
			},
			assertion: func(t *testing.T, err error, p *v1alpha1.RpaasPlan) {
				assert.Error(t, err)
				assert.Equal(t, NotFoundError{Msg: "no default plan found"}, err)
			},
		},
		{
			name: "when plan is not set and there are more than one default plan",
			resources: []runtime.Object{
				&v1alpha1.RpaasPlan{
					ObjectMeta: metav1.ObjectMeta{Name: "plan1"},
					Spec: v1alpha1.RpaasPlanSpec{
						Default: true,
					},
				},
				&v1alpha1.RpaasPlan{
					ObjectMeta: metav1.ObjectMeta{Name: "plan2"},
					Spec: v1alpha1.RpaasPlanSpec{
						Default: true,
					},
				},
			},
			assertion: func(t *testing.T, err error, p *v1alpha1.RpaasPlan) {
				assert.Error(t, err)
				assert.Error(t, ConflictError{Msg: "several default plans found: [plan1, plan2]"}, err)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			manager := &k8sRpaasManager{cli: fake.NewClientBuilder().WithScheme(newScheme()).WithRuntimeObjects(tt.resources...).Build()}
			p, err := manager.getPlan(context.Background(), tt.plan)
			tt.assertion(t, err, p)
		})
	}
}

func Test_isPathValid(t *testing.T) {
	tests := []struct {
		path     string
		expected bool
	}{
		{
			path:     "../../passwd",
			expected: false,
		},
		{
			path:     "/bin/bash",
			expected: false,
		},
		{
			path:     "./subdir/file.txt",
			expected: true,
		},
		{
			path:     "..data/test",
			expected: false,
		},
		{
			path:     "subdir/my-file..txt",
			expected: false,
		},
		{
			path:     "my-file.txt",
			expected: true,
		},
		{
			path:     "path/to/my/file.txt",
			expected: true,
		},
		{
			path:     ".my-hidden-file",
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run("", func(t *testing.T) {
			assert.Equal(t, tt.expected, isPathValid(tt.path))
		})
	}
}

func Test_convertPathToConfigMapKey(t *testing.T) {
	tests := []struct {
		path     string
		expected string
	}{
		{
			path:     "path/to/my-file.txt",
			expected: "path_to_my-file.txt",
		},
		{
			path:     "FILE@master.html",
			expected: "FILE_master.html",
		},
		{
			path:     "my new index.html",
			expected: "my_new_index.html",
		},
	}

	for _, tt := range tests {
		t.Run("", func(t *testing.T) {
			assert.Equal(t, tt.expected, convertPathToConfigMapKey(tt.path))
		})
	}
}

func Test_k8sRpaasManager_CreateInstance(t *testing.T) {
	resources := []runtime.Object{
		&v1alpha1.RpaasPlan{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "plan1",
				Namespace: getServiceName(),
			},
			Spec: v1alpha1.RpaasPlanSpec{
				Default: true,
			},
		},
		&v1alpha1.RpaasPlan{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "plan1",
				Namespace: "my-rpaasv2",
			},
			Spec: v1alpha1.RpaasPlanSpec{
				Default: true,
			},
		},
		&v1alpha1.RpaasInstance{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "r0",
				Namespace: getServiceName(),
				Labels: map[string]string{
					"rpaas.extensions.tsuru.io/service-name":  "rpaasv2",
					"rpaas.extensions.tsuru.io/instance-name": "r0",
					"rpaas_service":  "rpaasv2",
					"rpaas_instance": "r0",
				},
			},
			Spec: v1alpha1.RpaasInstanceSpec{},
		},
		&v1alpha1.RpaasFlavor{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "strawberry",
				Namespace: getServiceName(),
			},
			Spec: v1alpha1.RpaasFlavorSpec{
				Description: "aaaaa",
			},
		},
	}
	one := int32(1)
	tests := []struct {
		name          string
		args          CreateArgs
		expected      v1alpha1.RpaasInstance
		expectedError string
		extraConfig   config.RpaasConfig
		clusterName   string // to simulate a multi-cluster environment
		poolName      string // to simulate a pool-namespaced environment
	}{
		{
			name:          "without name",
			expectedError: `name is required`,
		},
		{
			name:          "name with length greater than 30 chars",
			args:          CreateArgs{Name: "some-awesome-great-instance-name"},
			expectedError: `instance name cannot length up than 30 chars`,
		},
		{
			name:          "name is not a valid DNS label name to Kubernetes",
			args:          CreateArgs{Name: `¯\_(ツ)_/¯`},
			expectedError: `instance name is not valid: a lowercase RFC 1123 label must consist of lower case alphanumeric characters or '-', and must start and end with an alphanumeric character (e.g. 'my-name',  or '123-abc', regex used for validation is '[a-z0-9]([-a-z0-9]*[a-z0-9])?')`,
		},
		{
			name:          "without team",
			args:          CreateArgs{Name: "r1"},
			expectedError: `team name is required`,
		},
		{
			name:          "invalid plan",
			args:          CreateArgs{Name: "r1", Team: "t1", Plan: "aaaaa"},
			expectedError: `invalid plan`,
		},
		{
			name:          "invalid flavor",
			args:          CreateArgs{Name: "r1", Team: "t1", Parameters: map[string]interface{}{"flavors": map[string]interface{}{"0": "aaaaa"}}},
			expectedError: `flavor "aaaaa" not found`,
		},
		{
			name:          "duplicated flavor",
			args:          CreateArgs{Name: "r1", Team: "t1", Parameters: map[string]interface{}{"flavors": map[string]interface{}{"0": "strawberry", "1": "strawberry"}}},
			expectedError: `flavor "strawberry" only can be set once`,
		},
		{
			name:          "instance already exists",
			args:          CreateArgs{Name: "r0", Team: "t2"},
			expectedError: `rpaas instance named "r0" already exists`,
		},
		{
			name: "simplest",
			args: CreateArgs{Name: "r1", Team: "t1"},
			expected: v1alpha1.RpaasInstance{
				TypeMeta: metav1.TypeMeta{
					Kind:       "RpaasInstance",
					APIVersion: "extensions.tsuru.io/v1alpha1",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:            "r1",
					Namespace:       "rpaasv2",
					ResourceVersion: "1",
					Annotations: map[string]string{
						"rpaas.extensions.tsuru.io/description": "",
						"rpaas.extensions.tsuru.io/tags":        "",
						"rpaas.extensions.tsuru.io/team-owner":  "t1",
					},
					Labels: map[string]string{
						"rpaas.extensions.tsuru.io/service-name":  "rpaasv2",
						"rpaas.extensions.tsuru.io/instance-name": "r1",
						"rpaas.extensions.tsuru.io/team-owner":    "t1",
						"rpaas_service":                           "rpaasv2",
						"rpaas_instance":                          "r1",
					},
				},
				Spec: v1alpha1.RpaasInstanceSpec{
					Replicas: &one,
					PlanName: "plan1",
					Service: &nginxv1alpha1.NginxService{
						Labels: map[string]string{
							"rpaas.extensions.tsuru.io/service-name":  "rpaasv2",
							"rpaas.extensions.tsuru.io/instance-name": "r1",
							"rpaas.extensions.tsuru.io/team-owner":    "t1",
							"rpaas_service":                           "rpaasv2",
							"rpaas_instance":                          "r1",
						},
					},
					PodTemplate: nginxv1alpha1.NginxPodTemplateSpec{
						Labels: map[string]string{
							"rpaas.extensions.tsuru.io/service-name":  "rpaasv2",
							"rpaas.extensions.tsuru.io/instance-name": "r1",
							"rpaas.extensions.tsuru.io/team-owner":    "t1",
							"rpaas_service":                           "rpaasv2",
							"rpaas_instance":                          "r1",
						},
					},
				},
			},
		},
		{
			name:        "w/ custom number of replicas",
			args:        CreateArgs{Name: "r1", Team: "t1"},
			extraConfig: config.RpaasConfig{NewInstanceReplicas: 3},
			expected: v1alpha1.RpaasInstance{
				TypeMeta: metav1.TypeMeta{
					Kind:       "RpaasInstance",
					APIVersion: "extensions.tsuru.io/v1alpha1",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:            "r1",
					Namespace:       "rpaasv2",
					ResourceVersion: "1",
					Annotations: map[string]string{
						"rpaas.extensions.tsuru.io/description": "",
						"rpaas.extensions.tsuru.io/tags":        "",
						"rpaas.extensions.tsuru.io/team-owner":  "t1",
					},
					Labels: map[string]string{
						"rpaas.extensions.tsuru.io/service-name":  "rpaasv2",
						"rpaas.extensions.tsuru.io/instance-name": "r1",
						"rpaas.extensions.tsuru.io/team-owner":    "t1",
						"rpaas_service":                           "rpaasv2",
						"rpaas_instance":                          "r1",
					},
				},
				Spec: v1alpha1.RpaasInstanceSpec{
					Replicas: func(n int32) *int32 { return &n }(3),
					PlanName: "plan1",
					Service: &nginxv1alpha1.NginxService{
						Labels: map[string]string{
							"rpaas.extensions.tsuru.io/service-name":  "rpaasv2",
							"rpaas.extensions.tsuru.io/instance-name": "r1",
							"rpaas.extensions.tsuru.io/team-owner":    "t1",
							"rpaas_service":                           "rpaasv2",
							"rpaas_instance":                          "r1",
						},
					},
					PodTemplate: nginxv1alpha1.NginxPodTemplateSpec{
						Labels: map[string]string{
							"rpaas.extensions.tsuru.io/service-name":  "rpaasv2",
							"rpaas.extensions.tsuru.io/instance-name": "r1",
							"rpaas.extensions.tsuru.io/team-owner":    "t1",
							"rpaas_service":                           "rpaasv2",
							"rpaas_instance":                          "r1",
						},
					},
				},
			},
		},
		{
			name:        "multi-cluster",
			args:        CreateArgs{Name: "r1", Team: "t1"},
			clusterName: "cluster-01",
			expected: v1alpha1.RpaasInstance{
				TypeMeta: metav1.TypeMeta{
					Kind:       "RpaasInstance",
					APIVersion: "extensions.tsuru.io/v1alpha1",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:            "r1",
					Namespace:       "rpaasv2",
					ResourceVersion: "1",
					Annotations: map[string]string{
						"rpaas.extensions.tsuru.io/description":  "",
						"rpaas.extensions.tsuru.io/tags":         "",
						"rpaas.extensions.tsuru.io/team-owner":   "t1",
						"rpaas.extensions.tsuru.io/cluster-name": "cluster-01",
					},
					Labels: map[string]string{
						"rpaas.extensions.tsuru.io/service-name":  "rpaasv2",
						"rpaas.extensions.tsuru.io/instance-name": "r1",
						"rpaas.extensions.tsuru.io/team-owner":    "t1",
						"rpaas.extensions.tsuru.io/cluster-name":  "cluster-01",
						"rpaas_service":                           "rpaasv2",
						"rpaas_instance":                          "r1",
					},
				},
				Spec: v1alpha1.RpaasInstanceSpec{
					Replicas: &one,
					PlanName: "plan1",
					Service: &nginxv1alpha1.NginxService{
						Labels: map[string]string{
							"rpaas.extensions.tsuru.io/service-name":  "rpaasv2",
							"rpaas.extensions.tsuru.io/instance-name": "r1",
							"rpaas.extensions.tsuru.io/team-owner":    "t1",
							"rpaas.extensions.tsuru.io/cluster-name":  "cluster-01",
							"rpaas_service":                           "rpaasv2",
							"rpaas_instance":                          "r1",
						},
					},
					PodTemplate: nginxv1alpha1.NginxPodTemplateSpec{
						Labels: map[string]string{
							"rpaas.extensions.tsuru.io/service-name":  "rpaasv2",
							"rpaas.extensions.tsuru.io/instance-name": "r1",
							"rpaas.extensions.tsuru.io/team-owner":    "t1",
							"rpaas.extensions.tsuru.io/cluster-name":  "cluster-01",
							"rpaas_service":                           "rpaasv2",
							"rpaas_instance":                          "r1",
						},
					},
				},
			},
		},
		{
			name:        "pool-namespaced",
			args:        CreateArgs{Name: "r1", Team: "t1"},
			clusterName: "cluster-01",
			poolName:    "my-pool",
			extraConfig: config.RpaasConfig{
				NamespacedInstances: true,
			},
			expected: v1alpha1.RpaasInstance{
				TypeMeta: metav1.TypeMeta{
					Kind:       "RpaasInstance",
					APIVersion: "extensions.tsuru.io/v1alpha1",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:            "r1",
					Namespace:       "rpaasv2-my-pool",
					ResourceVersion: "1",
					Annotations: map[string]string{
						"rpaas.extensions.tsuru.io/description":  "",
						"rpaas.extensions.tsuru.io/tags":         "",
						"rpaas.extensions.tsuru.io/team-owner":   "t1",
						"rpaas.extensions.tsuru.io/cluster-name": "cluster-01",
					},
					Labels: map[string]string{
						"rpaas.extensions.tsuru.io/service-name":  "rpaasv2",
						"rpaas.extensions.tsuru.io/instance-name": "r1",
						"rpaas.extensions.tsuru.io/team-owner":    "t1",
						"rpaas.extensions.tsuru.io/cluster-name":  "cluster-01",
						"rpaas_service":                           "rpaasv2",
						"rpaas_instance":                          "r1",
					},
				},
				Spec: v1alpha1.RpaasInstanceSpec{
					Replicas:      &one,
					PlanName:      "plan1",
					PlanNamespace: "rpaasv2",
					Service: &nginxv1alpha1.NginxService{
						Labels: map[string]string{
							"rpaas.extensions.tsuru.io/service-name":  "rpaasv2",
							"rpaas.extensions.tsuru.io/instance-name": "r1",
							"rpaas.extensions.tsuru.io/team-owner":    "t1",
							"rpaas.extensions.tsuru.io/cluster-name":  "cluster-01",
							"rpaas_service":                           "rpaasv2",
							"rpaas_instance":                          "r1",
						},
					},
					PodTemplate: nginxv1alpha1.NginxPodTemplateSpec{
						Labels: map[string]string{
							"rpaas.extensions.tsuru.io/service-name":  "rpaasv2",
							"rpaas.extensions.tsuru.io/instance-name": "r1",
							"rpaas.extensions.tsuru.io/team-owner":    "t1",
							"rpaas.extensions.tsuru.io/cluster-name":  "cluster-01",
							"rpaas_service":                           "rpaasv2",
							"rpaas_instance":                          "r1",
						},
					},
				},
			},
		},
		{
			name:        "pool-namespaced-custom-service",
			args:        CreateArgs{Name: "r1", Team: "t1"},
			clusterName: "cluster-01",
			poolName:    "my-pool",
			extraConfig: config.RpaasConfig{
				NamespacedInstances: true,
				ServiceName:         "my-rpaasv2",
			},
			expected: v1alpha1.RpaasInstance{
				TypeMeta: metav1.TypeMeta{
					Kind:       "RpaasInstance",
					APIVersion: "extensions.tsuru.io/v1alpha1",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:            "r1",
					Namespace:       "my-rpaasv2-my-pool",
					ResourceVersion: "1",
					Annotations: map[string]string{
						"rpaas.extensions.tsuru.io/description":  "",
						"rpaas.extensions.tsuru.io/tags":         "",
						"rpaas.extensions.tsuru.io/team-owner":   "t1",
						"rpaas.extensions.tsuru.io/cluster-name": "cluster-01",
					},
					Labels: map[string]string{
						"rpaas.extensions.tsuru.io/service-name":  "my-rpaasv2",
						"rpaas.extensions.tsuru.io/instance-name": "r1",
						"rpaas.extensions.tsuru.io/team-owner":    "t1",
						"rpaas.extensions.tsuru.io/cluster-name":  "cluster-01",
						"rpaas_service":                           "my-rpaasv2",
						"rpaas_instance":                          "r1",
					},
				},
				Spec: v1alpha1.RpaasInstanceSpec{
					Replicas:      &one,
					PlanName:      "plan1",
					PlanNamespace: "my-rpaasv2",
					Service: &nginxv1alpha1.NginxService{
						Labels: map[string]string{
							"rpaas.extensions.tsuru.io/service-name":  "my-rpaasv2",
							"rpaas.extensions.tsuru.io/instance-name": "r1",
							"rpaas.extensions.tsuru.io/team-owner":    "t1",
							"rpaas.extensions.tsuru.io/cluster-name":  "cluster-01",
							"rpaas_service":                           "my-rpaasv2",
							"rpaas_instance":                          "r1",
						},
					},
					PodTemplate: nginxv1alpha1.NginxPodTemplateSpec{
						Labels: map[string]string{
							"rpaas.extensions.tsuru.io/service-name":  "my-rpaasv2",
							"rpaas.extensions.tsuru.io/instance-name": "r1",
							"rpaas.extensions.tsuru.io/team-owner":    "t1",
							"rpaas.extensions.tsuru.io/cluster-name":  "cluster-01",
							"rpaas_service":                           "my-rpaasv2",
							"rpaas_instance":                          "r1",
						},
					},
				},
			},
		},
		{
			name: "with override",
			args: CreateArgs{Name: "r1", Team: "t1", Tags: []string{`plan-override={"config": {"cacheEnabled": false}}`}},
			expected: v1alpha1.RpaasInstance{
				TypeMeta: metav1.TypeMeta{
					Kind:       "RpaasInstance",
					APIVersion: "extensions.tsuru.io/v1alpha1",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:            "r1",
					Namespace:       "rpaasv2",
					ResourceVersion: "1",
					Annotations: map[string]string{
						"rpaas.extensions.tsuru.io/description": "",
						"rpaas.extensions.tsuru.io/tags":        `plan-override={"config": {"cacheEnabled": false}}`,
						"rpaas.extensions.tsuru.io/team-owner":  "t1",
					},
					Labels: map[string]string{
						"rpaas.extensions.tsuru.io/service-name":  "rpaasv2",
						"rpaas.extensions.tsuru.io/instance-name": "r1",
						"rpaas.extensions.tsuru.io/team-owner":    "t1",
						"rpaas_service":                           "rpaasv2",
						"rpaas_instance":                          "r1",
					},
				},
				Spec: v1alpha1.RpaasInstanceSpec{
					Replicas: &one,
					PlanName: "plan1",
					Service: &nginxv1alpha1.NginxService{
						Labels: map[string]string{
							"rpaas.extensions.tsuru.io/service-name":  "rpaasv2",
							"rpaas.extensions.tsuru.io/instance-name": "r1",
							"rpaas.extensions.tsuru.io/team-owner":    "t1",
							"rpaas_service":                           "rpaasv2",
							"rpaas_instance":                          "r1",
						},
					},
					PlanTemplate: &v1alpha1.RpaasPlanSpec{
						Config: v1alpha1.NginxConfig{
							CacheEnabled: v1alpha1.Bool(false),
						},
					},
					PodTemplate: nginxv1alpha1.NginxPodTemplateSpec{
						Labels: map[string]string{
							"rpaas.extensions.tsuru.io/service-name":  "rpaasv2",
							"rpaas.extensions.tsuru.io/instance-name": "r1",
							"rpaas.extensions.tsuru.io/team-owner":    "t1",
							"rpaas_service":                           "rpaasv2",
							"rpaas_instance":                          "r1",
						},
					},
				},
			},
		},
		{
			name: "with flavor",
			args: CreateArgs{Name: "r1", Team: "t1", Parameters: map[string]interface{}{"flavors": map[string]interface{}{"0": "strawberry"}}},
			expected: v1alpha1.RpaasInstance{
				TypeMeta: metav1.TypeMeta{
					Kind:       "RpaasInstance",
					APIVersion: "extensions.tsuru.io/v1alpha1",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:            "r1",
					Namespace:       "rpaasv2",
					ResourceVersion: "1",
					Annotations: map[string]string{
						"rpaas.extensions.tsuru.io/tags":        "",
						"rpaas.extensions.tsuru.io/description": "",
						"rpaas.extensions.tsuru.io/team-owner":  "t1",
					},
					Labels: map[string]string{
						"rpaas.extensions.tsuru.io/service-name":  "rpaasv2",
						"rpaas.extensions.tsuru.io/instance-name": "r1",
						"rpaas.extensions.tsuru.io/team-owner":    "t1",
						"rpaas_service":                           "rpaasv2",
						"rpaas_instance":                          "r1",
					},
				},
				Spec: v1alpha1.RpaasInstanceSpec{
					Replicas: &one,
					PlanName: "plan1",
					Flavors:  []string{"strawberry"},
					Service: &nginxv1alpha1.NginxService{
						Labels: map[string]string{
							"rpaas.extensions.tsuru.io/service-name":  "rpaasv2",
							"rpaas.extensions.tsuru.io/instance-name": "r1",
							"rpaas.extensions.tsuru.io/team-owner":    "t1",
							"rpaas_service":                           "rpaasv2",
							"rpaas_instance":                          "r1",
						},
					},
					PodTemplate: nginxv1alpha1.NginxPodTemplateSpec{
						Labels: map[string]string{
							"rpaas.extensions.tsuru.io/service-name":  "rpaasv2",
							"rpaas.extensions.tsuru.io/instance-name": "r1",
							"rpaas.extensions.tsuru.io/team-owner":    "t1",
							"rpaas_service":                           "rpaasv2",
							"rpaas_instance":                          "r1",
						},
					},
				},
			},
		},
		{
			name: "with team affinity",
			args: CreateArgs{Name: "r1", Team: "team-one"},
			expected: v1alpha1.RpaasInstance{
				TypeMeta: metav1.TypeMeta{
					Kind:       "RpaasInstance",
					APIVersion: "extensions.tsuru.io/v1alpha1",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:            "r1",
					Namespace:       "rpaasv2",
					ResourceVersion: "1",
					Annotations: map[string]string{
						"rpaas.extensions.tsuru.io/description": "",
						"rpaas.extensions.tsuru.io/tags":        "",
						"rpaas.extensions.tsuru.io/team-owner":  "team-one",
					},
					Labels: map[string]string{
						"rpaas.extensions.tsuru.io/service-name":  "rpaasv2",
						"rpaas.extensions.tsuru.io/instance-name": "r1",
						"rpaas.extensions.tsuru.io/team-owner":    "team-one",
						"rpaas_service":                           "rpaasv2",
						"rpaas_instance":                          "r1",
					},
				},
				Spec: v1alpha1.RpaasInstanceSpec{
					Replicas: &one,
					PlanName: "plan1",
					Service: &nginxv1alpha1.NginxService{
						Labels: map[string]string{
							"rpaas.extensions.tsuru.io/service-name":  "rpaasv2",
							"rpaas.extensions.tsuru.io/instance-name": "r1",
							"rpaas.extensions.tsuru.io/team-owner":    "team-one",
							"rpaas_service":                           "rpaasv2",
							"rpaas_instance":                          "r1",
						},
					},
					PodTemplate: nginxv1alpha1.NginxPodTemplateSpec{
						Affinity: &corev1.Affinity{
							NodeAffinity: &corev1.NodeAffinity{
								RequiredDuringSchedulingIgnoredDuringExecution: &corev1.NodeSelector{
									NodeSelectorTerms: []corev1.NodeSelectorTerm{
										{
											MatchExpressions: []corev1.NodeSelectorRequirement{
												{
													Key:      "machine-type",
													Operator: corev1.NodeSelectorOpIn,
													Values:   []string{"ultra-fast-io"},
												},
											},
										},
									},
								},
							},
						},
						Labels: map[string]string{
							"rpaas.extensions.tsuru.io/service-name":  "rpaasv2",
							"rpaas.extensions.tsuru.io/instance-name": "r1",
							"rpaas.extensions.tsuru.io/team-owner":    "team-one",
							"rpaas_service":                           "rpaasv2",
							"rpaas_instance":                          "r1",
						},
					},
				},
			},
		},
		{
			name: "with load balancer name",
			args: CreateArgs{Name: "r1", Team: "t1", Parameters: map[string]interface{}{"lb-name": "my-example.example"}},
			expected: v1alpha1.RpaasInstance{
				TypeMeta: metav1.TypeMeta{
					Kind:       "RpaasInstance",
					APIVersion: "extensions.tsuru.io/v1alpha1",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:            "r1",
					Namespace:       "rpaasv2",
					ResourceVersion: "1",
					Annotations: map[string]string{
						"rpaas.extensions.tsuru.io/description": "",
						"rpaas.extensions.tsuru.io/tags":        "",
						"rpaas.extensions.tsuru.io/team-owner":  "t1",
					},
					Labels: map[string]string{
						"rpaas.extensions.tsuru.io/service-name":  "rpaasv2",
						"rpaas.extensions.tsuru.io/instance-name": "r1",
						"rpaas.extensions.tsuru.io/team-owner":    "t1",
						"rpaas_service":                           "rpaasv2",
						"rpaas_instance":                          "r1",
					},
				},
				Spec: v1alpha1.RpaasInstanceSpec{
					Replicas: &one,
					PlanName: "plan1",
					Service: &nginxv1alpha1.NginxService{
						Annotations: map[string]string{
							"cloudprovider.example/lb-name": "my-example.example",
						},
						Labels: map[string]string{
							"rpaas.extensions.tsuru.io/service-name":  "rpaasv2",
							"rpaas.extensions.tsuru.io/instance-name": "r1",
							"rpaas.extensions.tsuru.io/team-owner":    "t1",
							"rpaas_service":                           "rpaasv2",
							"rpaas_instance":                          "r1",
						},
					},
					PodTemplate: nginxv1alpha1.NginxPodTemplateSpec{
						Labels: map[string]string{
							"rpaas.extensions.tsuru.io/service-name":  "rpaasv2",
							"rpaas.extensions.tsuru.io/instance-name": "r1",
							"rpaas.extensions.tsuru.io/team-owner":    "t1",
							"rpaas_service":                           "rpaasv2",
							"rpaas_instance":                          "r1",
						},
					},
				},
			},
		},
		{
			name:        "with custom annotations only set allowed ones",
			args:        CreateArgs{Name: "r1", Team: "t1", Parameters: map[string]interface{}{"annotations": "{\"my-custom-annotation\": \"my-value\"}"}},
			extraConfig: config.RpaasConfig{ForbiddenAnnotationsPrefixes: []string{"rpaas.extensions.tsuru.io"}},
			expected: v1alpha1.RpaasInstance{
				TypeMeta: metav1.TypeMeta{
					Kind:       "RpaasInstance",
					APIVersion: "extensions.tsuru.io/v1alpha1",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:            "r1",
					Namespace:       "rpaasv2",
					ResourceVersion: "1",
					Annotations: map[string]string{
						"rpaas.extensions.tsuru.io/tags":        "",
						"rpaas.extensions.tsuru.io/description": "",
						"rpaas.extensions.tsuru.io/team-owner":  "t1",
						"my-custom-annotation":                  "my-value",
					},
					Labels: map[string]string{
						"rpaas.extensions.tsuru.io/service-name":  "rpaasv2",
						"rpaas.extensions.tsuru.io/instance-name": "r1",
						"rpaas.extensions.tsuru.io/team-owner":    "t1",
						"rpaas_service":                           "rpaasv2",
						"rpaas_instance":                          "r1",
					},
				},
				Spec: v1alpha1.RpaasInstanceSpec{
					Replicas: &one,
					PlanName: "plan1",
					Service: &nginxv1alpha1.NginxService{
						Labels: map[string]string{
							"rpaas.extensions.tsuru.io/service-name":  "rpaasv2",
							"rpaas.extensions.tsuru.io/instance-name": "r1",
							"rpaas.extensions.tsuru.io/team-owner":    "t1",
							"rpaas_service":                           "rpaasv2",
							"rpaas_instance":                          "r1",
						},
					},
					PodTemplate: nginxv1alpha1.NginxPodTemplateSpec{
						Labels: map[string]string{
							"rpaas.extensions.tsuru.io/service-name":  "rpaasv2",
							"rpaas.extensions.tsuru.io/instance-name": "r1",
							"rpaas.extensions.tsuru.io/team-owner":    "t1",
							"rpaas_service":                           "rpaasv2",
							"rpaas_instance":                          "r1",
						},
					},
				},
			},
		},
		{
			name:          "with custom annotations and forbidden prefixes",
			args:          CreateArgs{Name: "r1", Team: "t1", Parameters: map[string]interface{}{"annotations": "{\"my-custom-annotation\": \"my-value\", \"rpaas.extensions.tsuru.io/tags\": \"tag1,tag2\", \"rpaas.extensions.tsuru.io/description\": \"my description\"}"}},
			extraConfig:   config.RpaasConfig{ForbiddenAnnotationsPrefixes: []string{"rpaas.extensions.tsuru.io"}},
			expectedError: `annotation "rpaas.extensions.tsuru.io/tags" is not allowed`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			baseConfig := config.RpaasConfig{
				NewInstanceReplicas:      1,
				ServiceName:              "rpaasv2",
				LoadBalancerNameLabelKey: "cloudprovider.example/lb-name",
				TeamAffinity: map[string]corev1.Affinity{
					"team-one": {
						NodeAffinity: &corev1.NodeAffinity{
							RequiredDuringSchedulingIgnoredDuringExecution: &corev1.NodeSelector{
								NodeSelectorTerms: []corev1.NodeSelectorTerm{
									{
										MatchExpressions: []corev1.NodeSelectorRequirement{
											{
												Key:      "machine-type",
												Operator: corev1.NodeSelectorOpIn,
												Values:   []string{"ultra-fast-io"},
											},
										},
									},
								},
							},
						},
					},
				},
			}
			mergo.MergeWithOverwrite(&baseConfig, tt.extraConfig)
			config.Set(baseConfig)
			defer config.Set(config.RpaasConfig{})
			scheme := newScheme()
			manager := &k8sRpaasManager{
				cli: fake.NewClientBuilder().WithScheme(scheme).WithRuntimeObjects(resources...).Build(), clusterName: tt.clusterName,
				poolName: tt.poolName,
			}
			err := manager.CreateInstance(context.Background(), tt.args)
			if tt.expectedError != "" {
				assert.EqualError(t, err, tt.expectedError)
				return
			}

			require.NoError(t, err)
			result, err := manager.GetInstance(context.Background(), tt.args.Name)
			require.NoError(t, err)
			assert.Equal(t, &tt.expected, result)
		})
	}
}

func Test_k8sRpaasManager_UpdateInstance(t *testing.T) {
	cfg := config.Get()
	defer func() { config.Set(cfg) }()
	config.Set(config.RpaasConfig{LoadBalancerNameLabelKey: "cloudprovider.example/lb-name", ForbiddenAnnotationsPrefixes: []string{"rpaas.extensions.tsuru.io"}})

	instance1 := newEmptyRpaasInstance()
	instance1.Name = "instance1"
	instance1.Labels = labelsForRpaasInstance(instance1.Name)
	instance1.Labels["rpaas.extensions.tsuru.io/team-owner"] = "team-one"
	instance1.Annotations = map[string]string{
		"rpaas.extensions.tsuru.io/description": "Description about instance1",
		"rpaas.extensions.tsuru.io/tags":        "tag1,tag2",
	}
	instance1.Spec.PlanName = "plan1"

	instance2 := newEmptyRpaasInstance()
	instance2.Name = "instance2"
	instance2.Labels = labelsForRpaasInstance(instance1.Name)
	instance2.Spec.PlanName = "plan1"
	instance2.Spec.Flavors = []string{"feature-create-only"}

	podLabels := mergeMap(instance1.Labels, map[string]string{"pod-label-1": "v1"})

	instance1.Spec.PodTemplate = nginxv1alpha1.NginxPodTemplateSpec{
		Annotations: instance1.Annotations,
		Labels:      podLabels,
	}

	plan1 := &v1alpha1.RpaasPlan{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "extensions.tsuru.io/v1alpha1",
			Kind:       "RpaasPlan",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "plan1",
			Namespace: getServiceName(),
		},
	}

	plan2 := &v1alpha1.RpaasPlan{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "extensions.tsuru.io/v1alpha1",
			Kind:       "RpaasPlan",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "plan2",
			Namespace: getServiceName(),
		},
	}

	creationOnlyFlavor := &v1alpha1.RpaasFlavor{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "feature-create-only",
			Namespace: getServiceName(),
		},
		Spec: v1alpha1.RpaasFlavorSpec{
			CreationOnly: true,
			Description:  "aaaaa",
		},
	}

	resources := []runtime.Object{instance1, instance2, plan1, plan2, creationOnlyFlavor}

	tests := []struct {
		name      string
		instance  string
		args      UpdateInstanceArgs
		assertion func(t *testing.T, err error, instance *v1alpha1.RpaasInstance)
	}{
		{
			name:     "when the newer plan does not exist",
			instance: "instance1",
			args: UpdateInstanceArgs{
				Plan: "not-found",
			},
			assertion: func(t *testing.T, err error, instance *v1alpha1.RpaasInstance) {
				require.Error(t, err)
				assert.Equal(t, &ValidationError{Msg: "invalid plan", Internal: NotFoundError{Msg: `plan "not-found" not found`}}, err)
			},
		},
		{
			name:     "when tries to add creationOnly flavor",
			instance: "instance1",
			args: UpdateInstanceArgs{
				Description: "Another description",
				Plan:        "plan2",
				Tags:        []string{"flavor:feature-create-only"},
				Team:        "team-two",
				Parameters: map[string]interface{}{
					"lb-name": "my-instance.example",
				},
			},
			assertion: func(t *testing.T, err error, instance *v1alpha1.RpaasInstance) {
				require.Error(t, err)
				assert.EqualError(t, err, `flavor "feature-create-only" can used only in the creation of instance`)
			},
		},
		{
			name:     "when tries to remove a creationOnly flavor",
			instance: "instance2",
			args: UpdateInstanceArgs{
				Description: "Another description",
				Plan:        "plan2",
				Tags:        []string{},
				Team:        "team-two",
				Parameters: map[string]interface{}{
					"lb-name": "my-instance.example",
				},
			},
			assertion: func(t *testing.T, err error, instance *v1alpha1.RpaasInstance) {
				require.Error(t, err)
				assert.Equal(t, `cannot unset flavor "feature-create-only" as it is a creation only flavor`, err.Error())
			},
		},
		{
			name:     "when tries to set an invalid annotation format",
			instance: "instance1",
			args: UpdateInstanceArgs{
				Description: "Another description",
				Plan:        "plan2",
				Tags:        []string{},
				Team:        "team-two",
				Parameters: map[string]interface{}{
					"annotations": "{\"test.io/_my-custom-annotation\": \"my-value\"}",
				},
			},
			assertion: func(t *testing.T, err error, instance *v1alpha1.RpaasInstance) {
				require.Error(t, err)
				assert.Equal(t, fmt.Sprintf(`invalid annotation "test.io/_my-custom-annotation": %v`, validation.IsValidLabelValue("test.io/_my-custom-annotation")), err.Error())
			},
		},
		{
			name:     "when successfully updating an instance",
			instance: "instance1",
			args: UpdateInstanceArgs{
				Description: "Another description",
				Plan:        "plan2",
				Tags:        []string{"tag3", "tag4", "tag5", `plan-override={"image": "my.registry.test/nginx:latest"}`},
				Team:        "team-two",
				Parameters: map[string]interface{}{
					"lb-name":     "my-instance.example",
					"annotations": "{\"my-custom-annotation\": \"my-value\"}",
				},
			},
			assertion: func(t *testing.T, err error, instance *v1alpha1.RpaasInstance) {
				require.NoError(t, err)
				require.NotNil(t, instance)
				assert.Equal(t, "plan2", instance.Spec.PlanName)
				require.NotNil(t, instance.Labels)
				assert.Equal(t, "team-two", instance.Labels["rpaas.extensions.tsuru.io/team-owner"])
				require.NotNil(t, instance.Annotations)
				assert.Equal(t, "my-value", instance.Annotations["my-custom-annotation"])
				assert.Equal(t, "Another description", instance.Annotations["rpaas.extensions.tsuru.io/description"])
				assert.Equal(t, `plan-override={"image": "my.registry.test/nginx:latest"},tag3,tag4,tag5`, instance.Annotations["rpaas.extensions.tsuru.io/tags"])
				assert.Equal(t, "team-two", instance.Annotations["rpaas.extensions.tsuru.io/team-owner"])
				require.NotNil(t, instance.Spec.PodTemplate)
				assert.Equal(t, "v1", instance.Spec.PodTemplate.Labels["pod-label-1"])
				assert.Equal(t, "team-two", instance.Spec.PodTemplate.Labels["rpaas.extensions.tsuru.io/team-owner"])
				assert.Equal(t, &v1alpha1.RpaasPlanSpec{Image: "my.registry.test/nginx:latest"}, instance.Spec.PlanTemplate)
				assert.Equal(t, instance.Spec.Service.Annotations["cloudprovider.example/lb-name"], "my-instance.example")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			manager := &k8sRpaasManager{
				cli: fake.NewClientBuilder().
					WithScheme(newScheme()).
					WithRuntimeObjects(resources...).
					Build(),
			}

			err := manager.UpdateInstance(context.TODO(), tt.instance, tt.args)
			instance := new(v1alpha1.RpaasInstance)
			if err == nil {
				nerr := manager.cli.Get(context.TODO(), types.NamespacedName{Name: tt.instance, Namespace: getServiceName()}, instance)
				require.NoError(t, nerr)
			}
			tt.assertion(t, err, instance)
		})
	}
}

func Test_k8sRpaasManager_GetFlavors(t *testing.T) {
	tests := []struct {
		resources []runtime.Object
		expected  []Flavor
	}{
		{
			resources: []runtime.Object{},
			expected:  nil,
		},
		{
			resources: []runtime.Object{
				&v1alpha1.RpaasFlavor{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "mint",
						Namespace: getServiceName(),
					},
					Spec: v1alpha1.RpaasFlavorSpec{
						Description: "Awesome description about mint flavor",
					},
				},
				&v1alpha1.RpaasFlavor{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "mango",
						Namespace: getServiceName(),
					},
					Spec: v1alpha1.RpaasFlavorSpec{
						Description: "Just a human readable description to mango flavor",
					},
				},
				&v1alpha1.RpaasFlavor{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "default",
						Namespace: getServiceName(),
					},
					Spec: v1alpha1.RpaasFlavorSpec{
						Default:     true,
						Description: "Default flavor that should not appear on flavors list",
					},
				},
			},
			expected: []Flavor{
				{
					Name:        "mango",
					Description: "Just a human readable description to mango flavor",
				},
				{
					Name:        "mint",
					Description: "Awesome description about mint flavor",
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run("", func(t *testing.T) {
			manager := &k8sRpaasManager{
				cli: fake.NewClientBuilder().WithScheme(newScheme()).WithRuntimeObjects(tt.resources...).Build(),
			}

			flavors, err := manager.GetFlavors(context.TODO())
			require.NoError(t, err)
			assert.Equal(t, tt.expected, flavors)
		})
	}
}

func newScheme() *runtime.Scheme {
	scheme := runtime.NewScheme()
	corev1.AddToScheme(scheme)
	networkingv1.AddToScheme(scheme)
	v1alpha1.SchemeBuilder.AddToScheme(scheme)
	metricsv1beta1.SchemeBuilder.AddToScheme(scheme)
	nginxv1alpha1.SchemeBuilder.AddToScheme(scheme)
	return scheme
}

func pointerToInt32(x int32) *int32 {
	return &x
}

func Test_k8sRpaasManager_GetAutoscale(t *testing.T) {
	scheme := runtime.NewScheme()
	corev1.AddToScheme(scheme)
	v1alpha1.SchemeBuilder.AddToScheme(scheme)
	nginxv1alpha1.SchemeBuilder.AddToScheme(scheme)

	instance1 := newEmptyRpaasInstance()
	instance1.Spec.Autoscale = &v1alpha1.RpaasInstanceAutoscaleSpec{
		MaxReplicas:                       3,
		MinReplicas:                       pointerToInt32(1),
		TargetCPUUtilizationPercentage:    pointerToInt32(70),
		TargetMemoryUtilizationPercentage: pointerToInt32(1024),
	}

	resources := []runtime.Object{instance1}

	testCases := []struct {
		instance  string
		assertion func(*testing.T, *clientTypes.Autoscale, error, *k8sRpaasManager)
	}{
		{
			instance: "my-invalid-instance",
			assertion: func(t *testing.T, s *clientTypes.Autoscale, err error, m *k8sRpaasManager) {
				assert.Error(t, NotFoundError{
					Msg: `instance not found`,
				}, err)
				assert.Nil(t, s)
			},
		},
		{
			instance: "my-instance",
			assertion: func(t *testing.T, s *clientTypes.Autoscale, err error, m *k8sRpaasManager) {
				assert.NoError(t, err)

				expectedAutoscale := &clientTypes.Autoscale{
					MaxReplicas: pointerToInt32(3),
					MinReplicas: pointerToInt32(1),
					CPU:         pointerToInt32(70),
					Memory:      pointerToInt32(1024),
				}
				assert.Equal(t, expectedAutoscale, s)
			},
		},
	}

	for _, tt := range testCases {
		t.Run("", func(t *testing.T) {
			manager := &k8sRpaasManager{cli: fake.NewClientBuilder().WithScheme(scheme).WithRuntimeObjects(resources...).Build()}
			autoscale, err := manager.GetAutoscale(context.Background(), tt.instance)
			tt.assertion(t, autoscale, err, manager)
		})
	}
}

func Test_k8sRpaasManager_CreateAutoscale(t *testing.T) {
	scheme := runtime.NewScheme()
	corev1.AddToScheme(scheme)
	v1alpha1.SchemeBuilder.AddToScheme(scheme)
	nginxv1alpha1.SchemeBuilder.AddToScheme(scheme)

	instance1 := newEmptyRpaasInstance()

	instance2 := newEmptyRpaasInstance()
	instance2.Name = "another-instance"
	instance2.Spec.Autoscale = &v1alpha1.RpaasInstanceAutoscaleSpec{
		MaxReplicas: 10,
	}

	resources := []runtime.Object{instance1, instance2}

	testCases := []struct {
		instance  string
		autoscale clientTypes.Autoscale
		assertion func(*testing.T, error, *k8sRpaasManager)
	}{
		{
			instance: "my-invalid-instance",
			autoscale: clientTypes.Autoscale{
				MaxReplicas: pointerToInt32(10),
			},
			assertion: func(t *testing.T, err error, m *k8sRpaasManager) {
				assert.Error(t, NotFoundError{
					Msg: `instance not found`,
				}, err)
			},
		},
		{
			instance: "my-instance",
			autoscale: clientTypes.Autoscale{
				MaxReplicas: pointerToInt32(0),
			},
			assertion: func(t *testing.T, err error, m *k8sRpaasManager) {
				assert.Error(t, ValidationError{
					Msg: `max replicas is required`,
				}, err)
			},
		},
		{
			instance: "another-instance",
			autoscale: clientTypes.Autoscale{
				MaxReplicas: pointerToInt32(0),
			},
			assertion: func(t *testing.T, err error, m *k8sRpaasManager) {
				assert.Error(t, ValidationError{
					Msg: `Autoscale already created`,
				}, err)
			},
		},
		{
			instance: "my-instance",
			autoscale: clientTypes.Autoscale{
				MaxReplicas: pointerToInt32(10),
				MinReplicas: pointerToInt32(5),
				CPU:         pointerToInt32(2),
				Memory:      pointerToInt32(1024),
			},
			assertion: func(t *testing.T, err error, m *k8sRpaasManager) {
				assert.NoError(t, err)

				instance := v1alpha1.RpaasInstance{}
				err = m.cli.Get(context.Background(), types.NamespacedName{Name: "my-instance", Namespace: getServiceName()}, &instance)
				require.NoError(t, err)

				assert.NotEqual(t, nil, instance.Spec.Autoscale)
				expectedAutoscale := &v1alpha1.RpaasInstanceAutoscaleSpec{
					MaxReplicas:                       10,
					MinReplicas:                       pointerToInt32(5),
					TargetCPUUtilizationPercentage:    pointerToInt32(2),
					TargetMemoryUtilizationPercentage: pointerToInt32(1024),
				}
				assert.Equal(t, expectedAutoscale, instance.Spec.Autoscale)
			},
		},
	}

	for _, tt := range testCases {
		t.Run("", func(t *testing.T) {
			manager := &k8sRpaasManager{cli: fake.NewClientBuilder().WithScheme(scheme).WithRuntimeObjects(resources...).Build()}
			err := manager.CreateAutoscale(context.Background(), tt.instance, &tt.autoscale)
			tt.assertion(t, err, manager)
		})
	}
}

func Test_k8sRpaasManager_UpdateAutoscale(t *testing.T) {
	scheme := runtime.NewScheme()
	corev1.AddToScheme(scheme)
	v1alpha1.SchemeBuilder.AddToScheme(scheme)
	nginxv1alpha1.SchemeBuilder.AddToScheme(scheme)

	instance1 := newEmptyRpaasInstance()
	instance1.Spec.Autoscale = &v1alpha1.RpaasInstanceAutoscaleSpec{
		MaxReplicas:                       3,
		MinReplicas:                       pointerToInt32(1),
		TargetCPUUtilizationPercentage:    pointerToInt32(70),
		TargetMemoryUtilizationPercentage: pointerToInt32(1024),
	}

	instance2 := newEmptyRpaasInstance()
	instance2.Name = "another-instance"
	instance2.Spec.Autoscale = &v1alpha1.RpaasInstanceAutoscaleSpec{
		MaxReplicas: 10,
	}

	instance3 := newEmptyRpaasInstance()
	instance3.Name = "noscale-instance"
	instance3.Spec.Autoscale = nil

	resources := []runtime.Object{instance1, instance2, instance3}

	testCases := []struct {
		instance  string
		autoscale clientTypes.Autoscale
		assertion func(*testing.T, error, *k8sRpaasManager)
	}{
		{
			instance: "my-invalid-instance",
			autoscale: clientTypes.Autoscale{
				MaxReplicas: pointerToInt32(10),
			},
			assertion: func(t *testing.T, err error, m *k8sRpaasManager) {
				assert.Error(t, NotFoundError{
					Msg: `instance not found`,
				}, err)
			},
		},
		{
			instance: "my-instance",
			autoscale: clientTypes.Autoscale{
				MaxReplicas: pointerToInt32(0),
			},
			assertion: func(t *testing.T, err error, m *k8sRpaasManager) {
				assert.Error(t, ValidationError{
					Msg: `max replicas is required`,
				}, err)
			},
		},
		{
			instance: "noscale-instance",
			autoscale: clientTypes.Autoscale{
				MaxReplicas: pointerToInt32(10),
			},
			assertion: func(t *testing.T, err error, m *k8sRpaasManager) {
				assert.NoError(t, err)

				instance := v1alpha1.RpaasInstance{}
				err = m.cli.Get(context.Background(), types.NamespacedName{Name: "noscale-instance", Namespace: getServiceName()}, &instance)
				require.NoError(t, err)

				assert.NotEqual(t, nil, instance.Spec.Autoscale)
				expectedAutoscale := &v1alpha1.RpaasInstanceAutoscaleSpec{
					MaxReplicas: 10,
				}
				assert.Equal(t, expectedAutoscale, instance.Spec.Autoscale)
			},
		},
		{
			instance: "my-instance",
			autoscale: clientTypes.Autoscale{
				MaxReplicas: pointerToInt32(10),
				MinReplicas: pointerToInt32(5),
				CPU:         pointerToInt32(80),
				Memory:      pointerToInt32(512),
			},
			assertion: func(t *testing.T, err error, m *k8sRpaasManager) {
				assert.NoError(t, err)

				instance := v1alpha1.RpaasInstance{}
				err = m.cli.Get(context.Background(), types.NamespacedName{Name: "my-instance", Namespace: getServiceName()}, &instance)
				require.NoError(t, err)

				assert.NotEqual(t, nil, instance.Spec.Autoscale)
				expectedAutoscale := &v1alpha1.RpaasInstanceAutoscaleSpec{
					MaxReplicas:                       10,
					MinReplicas:                       pointerToInt32(5),
					TargetCPUUtilizationPercentage:    pointerToInt32(80),
					TargetMemoryUtilizationPercentage: pointerToInt32(512),
				}
				assert.Equal(t, expectedAutoscale, instance.Spec.Autoscale)
			},
		},
	}

	for _, tt := range testCases {
		t.Run("", func(t *testing.T) {
			manager := &k8sRpaasManager{cli: fake.NewClientBuilder().WithScheme(scheme).WithRuntimeObjects(resources...).Build()}
			err := manager.UpdateAutoscale(context.Background(), tt.instance, &tt.autoscale)
			tt.assertion(t, err, manager)
		})
	}
}

func Test_k8sRpaasManager_Scale(t *testing.T) {
	scheme := runtime.NewScheme()
	corev1.AddToScheme(scheme)
	v1alpha1.SchemeBuilder.AddToScheme(scheme)
	nginxv1alpha1.SchemeBuilder.AddToScheme(scheme)

	instance1 := newEmptyRpaasInstance()
	instance1.Spec.Autoscale = &v1alpha1.RpaasInstanceAutoscaleSpec{
		MaxReplicas:                    3,
		MinReplicas:                    pointerToInt32(1),
		TargetCPUUtilizationPercentage: pointerToInt32(70),
	}

	instance2 := newEmptyRpaasInstance()
	instance2.Name = "another-instance"
	instance2.Spec.Autoscale = nil

	resources := []runtime.Object{instance1, instance2}

	testCases := []struct {
		instance  string
		assertion func(*testing.T, error, *k8sRpaasManager)
	}{
		{
			instance: "my-invalid-instance",
			assertion: func(t *testing.T, err error, m *k8sRpaasManager) {
				assert.Error(t, NotFoundError{
					Msg: `instance not found`,
				}, err)
			},
		},
		{
			instance: "my-instance",
			assertion: func(t *testing.T, err error, m *k8sRpaasManager) {
				assert.Error(t, err)
				assert.Equal(t, "cannot scale manual with autoscaler configured, please update autoscale settings", err.Error())

				instance := v1alpha1.RpaasInstance{}
				err = m.cli.Get(context.Background(), types.NamespacedName{Name: "my-instance", Namespace: getServiceName()}, &instance)
				require.NoError(t, err)

				assert.NotNil(t, instance.Spec.Autoscale)
			},
		},
		{
			instance: "another-instance",
			assertion: func(t *testing.T, err error, m *k8sRpaasManager) {
				assert.NoError(t, err)

				instance := v1alpha1.RpaasInstance{}
				err = m.cli.Get(context.Background(), types.NamespacedName{Name: "another-instance", Namespace: getServiceName()}, &instance)
				require.NoError(t, err)

				assert.Equal(t, int32(30), *instance.Spec.Replicas)
			},
		},
	}

	for _, tt := range testCases {
		t.Run(tt.instance, func(t *testing.T) {
			manager := &k8sRpaasManager{cli: fake.NewClientBuilder().WithScheme(scheme).WithRuntimeObjects(resources...).Build()}
			err := manager.Scale(context.Background(), tt.instance, 30)
			tt.assertion(t, err, manager)
		})
	}

}

func Test_k8sRpaasManager_DeleteAutoscale(t *testing.T) {
	scheme := runtime.NewScheme()
	corev1.AddToScheme(scheme)
	v1alpha1.SchemeBuilder.AddToScheme(scheme)
	nginxv1alpha1.SchemeBuilder.AddToScheme(scheme)

	instance1 := newEmptyRpaasInstance()
	instance1.Spec.Autoscale = &v1alpha1.RpaasInstanceAutoscaleSpec{
		MaxReplicas:                       3,
		MinReplicas:                       pointerToInt32(1),
		TargetCPUUtilizationPercentage:    pointerToInt32(70),
		TargetMemoryUtilizationPercentage: pointerToInt32(1024),
	}

	resources := []runtime.Object{instance1}

	testCases := []struct {
		instance  string
		assertion func(*testing.T, error, *k8sRpaasManager)
	}{
		{
			instance: "my-invalid-instance",
			assertion: func(t *testing.T, err error, m *k8sRpaasManager) {
				assert.Error(t, NotFoundError{
					Msg: `instance not found`,
				}, err)
			},
		},
		{
			instance: "my-instance",
			assertion: func(t *testing.T, err error, m *k8sRpaasManager) {
				assert.NoError(t, err)

				instance := v1alpha1.RpaasInstance{}
				err = m.cli.Get(context.Background(), types.NamespacedName{Name: "my-instance", Namespace: getServiceName()}, &instance)
				require.NoError(t, err)

				assert.Nil(t, instance.Spec.Autoscale)
			},
		},
	}

	for _, tt := range testCases {
		t.Run("", func(t *testing.T) {
			manager := &k8sRpaasManager{cli: fake.NewClientBuilder().WithScheme(scheme).WithRuntimeObjects(resources...).Build()}
			err := manager.DeleteAutoscale(context.Background(), tt.instance)
			tt.assertion(t, err, manager)
		})
	}
}

func Test_k8sRpaasManager_GetInstanceInfo(t *testing.T) {
	cfg := config.Get()
	defer func() { config.Set(cfg) }()
	config.Set(config.RpaasConfig{
		DashboardTemplate: "http://grafana.example/?instance_name={{ .Name }}&service={{ .Service }}",
	})

	t0 := time.Date(2020, 4, 2, 16, 10, 0, 0, time.UTC)

	tests := map[string]struct {
		resources []runtime.Object
		manager   func(c client.Client) RpaasManager
		instance  func(i v1alpha1.RpaasInstance) v1alpha1.RpaasInstance
		expected  func(info clientTypes.InstanceInfo) clientTypes.InstanceInfo
	}{
		"base instance info": {
			instance: func(i v1alpha1.RpaasInstance) v1alpha1.RpaasInstance {
				return i
			},
			expected: func(info clientTypes.InstanceInfo) clientTypes.InstanceInfo {
				return info
			},
		},

		"base instance info from multi-cluster service": {
			manager: func(c client.Client) RpaasManager {
				return &k8sRpaasManager{cli: c, clusterName: "my-cluster", poolName: "my-pool"}
			},
			instance: func(i v1alpha1.RpaasInstance) v1alpha1.RpaasInstance {
				return i
			},
			expected: func(info clientTypes.InstanceInfo) clientTypes.InstanceInfo {
				info.Cluster = "my-cluster"
				info.Pool = "my-pool"
				return info
			},
		},

		"w/ replicas set": {
			instance: func(i v1alpha1.RpaasInstance) v1alpha1.RpaasInstance {
				i.Spec.Replicas = pointerToInt32(7)
				return i
			},
			expected: func(info clientTypes.InstanceInfo) clientTypes.InstanceInfo {
				info.Replicas = pointerToInt32(7)
				return info
			},
		},

		"w/ autoscale config": {
			instance: func(i v1alpha1.RpaasInstance) v1alpha1.RpaasInstance {
				i.Spec.Autoscale = &v1alpha1.RpaasInstanceAutoscaleSpec{
					MaxReplicas:                    100,
					MinReplicas:                    pointerToInt32(1),
					TargetCPUUtilizationPercentage: pointerToInt32(90),
				}
				return i
			},
			expected: func(info clientTypes.InstanceInfo) clientTypes.InstanceInfo {
				info.Autoscale = &clientTypes.Autoscale{
					MaxReplicas: pointerToInt32(100),
					MinReplicas: pointerToInt32(1),
					CPU:         pointerToInt32(90),
				}
				return info
			},
		},

		"w/ blocks and routes": {
			instance: func(i v1alpha1.RpaasInstance) v1alpha1.RpaasInstance {
				i.Spec.Blocks = map[v1alpha1.BlockType]v1alpha1.Value{
					v1alpha1.BlockTypeHTTP:   {Value: "# some nginx config at http context"},
					v1alpha1.BlockTypeServer: {Value: "# some nginx config at server context"},
				}
				i.Spec.Locations = []v1alpha1.Location{
					{Path: "/custom/path/1", Destination: "app1.tsuru.example.com"},
					{Path: "/custom/path/2", Content: &v1alpha1.Value{Value: "# some nginx configuration"}},
					{Path: "/custom/path/3", Destination: "app3.tsuru.example.com", ForceHTTPS: true},
				}
				return i
			},
			expected: func(info clientTypes.InstanceInfo) clientTypes.InstanceInfo {
				info.Blocks = []clientTypes.Block{
					{Name: "http", Content: "# some nginx config at http context"},
					{Name: "server", Content: "# some nginx config at server context"},
				}
				info.Routes = []clientTypes.Route{
					{Path: "/custom/path/1", Destination: "app1.tsuru.example.com"},
					{Path: "/custom/path/2", Content: "# some nginx configuration"},
					{Path: "/custom/path/3", Destination: "app3.tsuru.example.com", HTTPSOnly: true},
				}
				return info
			},
		},

		"w/ TLS certs": {
			resources: []runtime.Object{
				&corev1.Secret{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "my-instance-certs-01",
						Namespace: "rpaasv2",
						Labels: map[string]string{
							"rpaas.extensions.tsuru.io/certificate-name": "default",
							"rpaas.extensions.tsuru.io/instance-name":    "my-instance",
						},
					},
					Data: map[string][]byte{
						"tls.crt": []byte(rsaCertificateInPEM),
						"tls.key": []byte(rsaPrivateKeyInPEM),
					},
				},
				&corev1.Secret{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "my-instance-certs-02",
						Namespace: "rpaasv2",
						Labels: map[string]string{
							"rpaas.extensions.tsuru.io/certificate-name": "my-instance.example.com",
							"rpaas.extensions.tsuru.io/instance-name":    "my-instance",
						},
					},
					Data: map[string][]byte{
						"tls.crt": []byte(ecdsaCertificateInPEM),
						"tls.key": []byte(ecdsaPrivateKeyInPEM),
					},
				},
			},
			instance: func(i v1alpha1.RpaasInstance) v1alpha1.RpaasInstance {
				i.Spec.TLS = []nginxv1alpha1.NginxTLS{
					{SecretName: "my-instance-certs-01"},
					{SecretName: "my-instance-certs-02"},
				}
				return i
			},
			expected: func(info clientTypes.InstanceInfo) clientTypes.InstanceInfo {
				info.Certificates = []clientTypes.CertificateInfo{
					{
						Name:               "default",
						ValidFrom:          time.Date(2020, time.August, 12, 20, 27, 46, 0, time.UTC),
						ValidUntil:         time.Date(2021, time.August, 12, 20, 27, 46, 0, time.UTC),
						DNSNames:           []string{"localhost", "example.com", "another-name.test"},
						PublicKeyAlgorithm: "RSA",
						PublicKeyBitSize:   512,
					},
					{
						Name:               "my-instance.example.com",
						ValidFrom:          time.Date(2017, time.October, 20, 19, 43, 6, 0, time.UTC),
						ValidUntil:         time.Date(2018, time.October, 20, 19, 43, 6, 0, time.UTC),
						DNSNames:           []string{"localhost:5453", "127.0.0.1:5453"},
						PublicKeyAlgorithm: "ECDSA",
						PublicKeyBitSize:   256,
					},
				}
				return info
			},
		},

		"w/ service load balancer failed": {
			resources: []runtime.Object{
				&nginxv1alpha1.Nginx{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "my-instance",
						Namespace: "rpaasv2",
					},
					Status: nginxv1alpha1.NginxStatus{
						Services: []nginxv1alpha1.ServiceStatus{{Name: "my-instance-service"}},
					},
				},
				&corev1.Service{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "my-instance-service",
						Namespace: "rpaasv2",
						UID:       types.UID("my-instance-service-uid"),
					},
					Spec: corev1.ServiceSpec{
						Type:      corev1.ServiceTypeLoadBalancer,
						ClusterIP: "10.10.10.100",
					},
				},
				&corev1.Event{
					ObjectMeta: metav1.ObjectMeta{
						Name:              "my-instance-service.313",
						Namespace:         "rpaasv2",
						CreationTimestamp: metav1.NewTime(t0),
					},
					InvolvedObject: corev1.ObjectReference{
						Kind:      "Service",
						Name:      "my-instance-service",
						Namespace: "rpaasv2",
						UID:       types.UID("my-instance-service-uid"),
					},
					FirstTimestamp: metav1.NewTime(t0.Add(-time.Hour)),
					LastTimestamp:  metav1.NewTime(t0.Add(-time.Second)),
					Count:          16,
					Type:           corev1.EventTypeWarning,
					Reason:         "EnsuringLoadBalancer",
					Message:        "Some error to set up loadbalancer",
				},
			},
			instance: func(i v1alpha1.RpaasInstance) v1alpha1.RpaasInstance {
				return i
			},
			expected: func(info clientTypes.InstanceInfo) clientTypes.InstanceInfo {
				info.Addresses = []clientTypes.InstanceAddress{
					{
						Type:        clientTypes.InstanceAddressTypeClusterExternal,
						ServiceName: "my-instance-service",
						Status:      "pending: 2020-04-02T16:09:59Z - Warning - Some error to set up loadbalancer\n",
					},
					{
						Type:        clientTypes.InstanceAddressTypeClusterInternal,
						ServiceName: "my-instance-service",
						Hostname:    "my-instance-service.rpaasv2.svc.cluster.local",
						IP:          "10.10.10.100",
						Status:      "ready",
					},
				}
				return info
			},
		},

		"w/ ingress failed": {
			resources: []runtime.Object{
				&nginxv1alpha1.Nginx{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "my-instance",
						Namespace: "rpaasv2",
					},
					Status: nginxv1alpha1.NginxStatus{
						Ingresses: []nginxv1alpha1.IngressStatus{{Name: "my-instance"}},
					},
				},
				&networkingv1.Ingress{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "my-instance",
						Namespace: "rpaasv2",
						UID:       types.UID("my-instance-uid"),
					},
				},
				&corev1.Event{
					ObjectMeta: metav1.ObjectMeta{
						Name:              "my-instance.123",
						Namespace:         "rpaasv2",
						CreationTimestamp: metav1.NewTime(t0),
					},
					InvolvedObject: corev1.ObjectReference{
						Kind:      "Ingress",
						Name:      "my-instance",
						Namespace: "rpaasv2",
						UID:       types.UID("my-instance-uid"),
					},
					FirstTimestamp: metav1.NewTime(t0.Add(-time.Hour)),
					LastTimestamp:  metav1.NewTime(t0.Add(-time.Second)),
					Count:          42,
					Type:           corev1.EventTypeWarning,
					Reason:         "InvalidIngressRule",
					Message:        "Failed to valid some rule in the Ingress",
				},
			},
			instance: func(i v1alpha1.RpaasInstance) v1alpha1.RpaasInstance {
				return i
			},
			expected: func(info clientTypes.InstanceInfo) clientTypes.InstanceInfo {
				info.Addresses = []clientTypes.InstanceAddress{
					{
						Type:        clientTypes.InstanceAddressTypeClusterExternal,
						IngressName: "my-instance",
						Status:      "pending: 2020-04-02T16:09:59Z - Warning - Failed to valid some rule in the Ingress\n",
					},
				}
				return info
			},
		},

		"w/ load balancer from Service and Ingress + DNS names": {
			resources: []runtime.Object{
				&nginxv1alpha1.Nginx{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "my-instance",
						Namespace: "rpaasv2",
					},
					Status: nginxv1alpha1.NginxStatus{
						Services:  []nginxv1alpha1.ServiceStatus{{Name: "my-instance-service"}},
						Ingresses: []nginxv1alpha1.IngressStatus{{Name: "my-instance"}},
					},
				},
				&corev1.Service{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "my-instance-service",
						Namespace: "rpaasv2",
						Annotations: map[string]string{
							"external-dns.alpha.kubernetes.io/hostname": "my-instance.zone1.tld"},
					},
					Spec: corev1.ServiceSpec{
						Type: corev1.ServiceTypeLoadBalancer,
					},
					Status: corev1.ServiceStatus{
						LoadBalancer: corev1.LoadBalancerStatus{
							Ingress: []corev1.LoadBalancerIngress{{IP: "192.168.10.10"}},
						},
					},
				},
				&networkingv1.Ingress{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "my-instance",
						Namespace: "rpaasv2",
						Annotations: map[string]string{
							"external-dns.alpha.kubernetes.io/hostname": "example.com,www.example.com",
						},
					},
					Status: networkingv1.IngressStatus{
						LoadBalancer: corev1.LoadBalancerStatus{
							Ingress: []corev1.LoadBalancerIngress{
								{IP: "192.168.200.200"},
								{Hostname: "abc.lb.example.com"},
							},
						},
					},
				},
			},
			instance: func(i v1alpha1.RpaasInstance) v1alpha1.RpaasInstance {
				return i
			},
			expected: func(info clientTypes.InstanceInfo) clientTypes.InstanceInfo {
				info.Addresses = []clientTypes.InstanceAddress{
					{
						Type:        clientTypes.InstanceAddressTypeClusterExternal,
						IngressName: "my-instance",
						Hostname:    "abc.lb.example.com,example.com,www.example.com",
						Status:      "ready",
					},
					{
						Type:        clientTypes.InstanceAddressTypeClusterExternal,
						ServiceName: "my-instance-service",
						IP:          "192.168.10.10",
						Hostname:    "my-instance.zone1.tld",
						Status:      "ready",
					},
					{
						Type:        clientTypes.InstanceAddressTypeClusterExternal,
						IngressName: "my-instance",
						Hostname:    "example.com,www.example.com",
						IP:          "192.168.200.200",
						Status:      "ready",
					},
				}
				return info
			},
		},

		"w/ pod status and pod errors": {
			resources: []runtime.Object{
				&nginxv1alpha1.Nginx{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "my-instance",
						Namespace: "rpaasv2",
					},
					Status: nginxv1alpha1.NginxStatus{
						PodSelector: "nginx.tsuru.io/app=nginx,nginx.tsuru.io/resource-name=my-instance",
					},
				},
				&corev1.Pod{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "my-instance-6f86f957b7-abcde",
						Namespace: "rpaasv2",
						Labels: map[string]string{
							"nginx.tsuru.io/app":           "nginx",
							"nginx.tsuru.io/resource-name": "my-instance",
						},
						CreationTimestamp: metav1.NewTime(t0),
						UID:               types.UID("my-instance-6f86f957b7-abcde"),
					},
					Spec: corev1.PodSpec{
						Containers: []corev1.Container{
							{
								Name: "nginx",
								Ports: []corev1.ContainerPort{
									{Name: "http", HostPort: int32(30000)},
									{Name: "https", HostPort: int32(30001)},
									{Name: "nginx-metrics", HostPort: int32(30002)},
								},
							},
						},
					},
					Status: corev1.PodStatus{
						Phase:  corev1.PodRunning,
						PodIP:  "172.16.100.21",
						HostIP: "10.10.10.11",
						ContainerStatuses: []corev1.ContainerStatus{
							{Name: "nginx", Ready: true, RestartCount: int32(10)},
						},
					},
				},
				&corev1.Pod{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "my-instance-6f86f957b7-fghij",
						Namespace: "rpaasv2",
						Labels: map[string]string{
							"nginx.tsuru.io/app":           "nginx",
							"nginx.tsuru.io/resource-name": "my-instance",
						},
						CreationTimestamp: metav1.NewTime(t0.Add(time.Hour)),
						UID:               types.UID("my-instance-6f86f957b7-fghij"),
					},
					Spec: corev1.PodSpec{
						Containers: []corev1.Container{
							{
								Name: "nginx",
								Ports: []corev1.ContainerPort{
									{Name: "http", HostPort: int32(30000)},
									{Name: "https", HostPort: int32(30001)},
									{Name: "nginx-metrics", HostPort: int32(30002)},
								},
							},
						},
					},
					Status: corev1.PodStatus{
						Phase: corev1.PodRunning,
						ContainerStatuses: []corev1.ContainerStatus{
							{
								Name: "nginx",
								State: corev1.ContainerState{
									Waiting: &corev1.ContainerStateWaiting{
										Reason:  "CrashLoopBackOff",
										Message: "Back-off 5m0s restarting failed container=nginx pod=my-instance-6f86f957b7-fghij_default(pod uuid)",
									},
								},
								LastTerminationState: corev1.ContainerState{
									Terminated: &corev1.ContainerStateTerminated{
										ExitCode: int32(137),
										Reason:   "Error",
									},
								},
							},
						},
					},
				},
				&corev1.Pod{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "my-instance-6f86f957b7-klmno",
						Namespace: "rpaasv2",
						Labels: map[string]string{
							"nginx.tsuru.io/app":           "nginx",
							"nginx.tsuru.io/resource-name": "my-instance",
						},
						CreationTimestamp: metav1.NewTime(t0),
						DeletionTimestamp: &metav1.Time{Time: t0.Add(time.Minute)},
						UID:               types.UID("my-instance-6f86f957b7-klmno"),
					},
					Spec: corev1.PodSpec{
						Containers: []corev1.Container{
							{
								Name: "nginx",
								Ports: []corev1.ContainerPort{
									{Name: "http", HostPort: int32(30000)},
									{Name: "https", HostPort: int32(30001)},
									{Name: "nginx-metrics", HostPort: int32(30002)},
								},
							},
						},
					},
					Status: corev1.PodStatus{
						Phase:  corev1.PodRunning,
						PodIP:  "172.16.100.20",
						HostIP: "10.10.10.10",
						ContainerStatuses: []corev1.ContainerStatus{
							{Name: "nginx", Ready: true},
						},
					},
				},
				&metricsv1beta1.PodMetrics{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "my-instance-6f86f957b7-abcde",
						Namespace: "rpaasv2",
						Labels: map[string]string{
							"nginx.tsuru.io/app":           "nginx",
							"nginx.tsuru.io/resource-name": "my-instance",
						},
						UID: types.UID("my-instance-6f86f957b7-abcde"),
					},
					Containers: []metricsv1beta1.ContainerMetrics{
						{
							Name: "nginx",
							Usage: corev1.ResourceList{
								"cpu":    resource.MustParse("100m"),
								"memory": resource.MustParse("100Mi"),
							},
						},
					},
				},
				&metricsv1beta1.PodMetrics{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "my-instance-6f86f957b7-klmno",
						Namespace: "rpaasv2",
						Labels: map[string]string{
							"nginx.tsuru.io/app":           "nginx",
							"nginx.tsuru.io/resource-name": "my-instance",
						},
						UID: types.UID("my-instance-6f86f957b7-klmno"),
					},
					Containers: []metricsv1beta1.ContainerMetrics{
						{
							Name: "nginx",
							Usage: corev1.ResourceList{
								"cpu":    resource.MustParse("30m"),
								"memory": resource.MustParse("10Mi"),
							},
						},
					},
				},
				&corev1.Event{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "my-instance-6f86f957b7-fghij.1",
						Namespace: "rpaasv2",
					},
					InvolvedObject: corev1.ObjectReference{
						Kind:      "Pod",
						Name:      "my-instance-6f86f957b7-fghij",
						Namespace: "rpaasv2",
						UID:       types.UID("my-instance-6f86f957b7-fghij"),
					},
					FirstTimestamp: metav1.NewTime(t0.Add(-time.Hour)),
					LastTimestamp:  metav1.NewTime(t0.Add(-time.Hour)),
					Type:           corev1.EventTypeNormal,
					Reason:         "Pulled",
					Message:        `Container image "nginx:1.16.1" already present on machine`,
				},
				&corev1.Event{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "my-instance-6f86f957b7-fghij.2",
						Namespace: "rpaasv2",
					},
					InvolvedObject: corev1.ObjectReference{
						Kind:      "Pod",
						Name:      "my-instance-6f86f957b7-fghij",
						Namespace: "rpaasv2",
						UID:       types.UID("my-instance-6f86f957b7-fghij"),
					},
					FirstTimestamp: metav1.NewTime(t0.Add(-time.Hour)),
					LastTimestamp:  metav1.NewTime(t0.Add(-time.Minute)),
					Count:          15,
					Type:           corev1.EventTypeWarning,
					Reason:         "FailedPostStartHook",
					Message:        "Exec lifecycle hook ([/bin/sh -c nginx -t && touch /tmp/done]) for Container \"nginx\" in Pod \"my-instance-6f86f957b7-fghij_rpaasv2(pod uuid)\" failed - error: command '/bin/sh -c nginx -t && touch /tmp/done' exited with 1: 2020/04/07 16:54:18 [emerg] 18#18: \"location\" directive is not allowed here in /etc/nginx/nginx.conf:118\nnginx: [emerg] \"location\" directive is not allowed here in /etc/nginx/nginx.conf:118\nnginx: configuration file /etc/nginx/nginx.conf test failed\n, message: \"2020/04/07 16:54:18 [emerg] 18#18: \\\"location\\\" directive is not allowed here in /etc/nginx/nginx.conf:118\\nnginx: [emerg] \\\"location\\\" directive is not allowed here in /etc/nginx/nginx.conf:118\\nnginx: configuration file /etc/nginx/nginx.conf test failed\\n\"",
				},
				&corev1.Event{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "my-instance-6f86f957b7-fghij.3",
						Namespace: "rpaasv2",
					},
					InvolvedObject: corev1.ObjectReference{
						Kind:      "Pod",
						Name:      "my-instance-6f86f957b7-fghij",
						Namespace: "rpaasv2",
						UID:       types.UID("my-instance-6f86f957b7-fghij"),
					},
					FirstTimestamp: metav1.NewTime(t0.Add(-time.Hour)),
					LastTimestamp:  metav1.NewTime(t0.Add(-time.Second)),
					Count:          16,
					Type:           corev1.EventTypeWarning,
					Reason:         "BackOff",
					Message:        "Back-off restarting failed container",
				},
			},
			instance: func(i v1alpha1.RpaasInstance) v1alpha1.RpaasInstance {
				return i
			},
			expected: func(info clientTypes.InstanceInfo) clientTypes.InstanceInfo {
				info.Pods = []clientTypes.Pod{
					{
						Name:      "my-instance-6f86f957b7-abcde",
						IP:        "172.16.100.21",
						HostIP:    "10.10.10.11",
						Status:    "Running",
						CreatedAt: time.Date(2020, 4, 2, 16, 10, 0, 0, time.UTC),
						Restarts:  int32(10),
						Ready:     true,
						Ports: []clientTypes.PodPort{
							{Name: "http", HostPort: int32(30000)},
							{Name: "https", HostPort: int32(30001)},
							{Name: "nginx-metrics", HostPort: int32(30002)},
						},
						Metrics: &clientTypes.PodMetrics{
							CPU:    "100m",
							Memory: "100Mi",
						},
					},
					{
						Name:      "my-instance-6f86f957b7-fghij",
						Status:    "Errored",
						CreatedAt: time.Date(2020, 4, 2, 17, 10, 0, 0, time.UTC),
						Ports: []clientTypes.PodPort{
							{Name: "http", HostPort: int32(30000)},
							{Name: "https", HostPort: int32(30001)},
							{Name: "nginx-metrics", HostPort: int32(30002)},
						},
						Ready: false,
						Errors: []clientTypes.PodError{
							{
								First:   t0.Add(-time.Hour).In(time.UTC),
								Last:    t0.Add(-time.Minute).In(time.UTC),
								Count:   int32(15),
								Message: "Exec lifecycle hook ([/bin/sh -c nginx -t && touch /tmp/done]) for Container \"nginx\" in Pod \"my-instance-6f86f957b7-fghij_rpaasv2(pod uuid)\" failed - error: command '/bin/sh -c nginx -t && touch /tmp/done' exited with 1: 2020/04/07 16:54:18 [emerg] 18#18: \"location\" directive is not allowed here in /etc/nginx/nginx.conf:118\nnginx: [emerg] \"location\" directive is not allowed here in /etc/nginx/nginx.conf:118\nnginx: configuration file /etc/nginx/nginx.conf test failed\n, message: \"2020/04/07 16:54:18 [emerg] 18#18: \\\"location\\\" directive is not allowed here in /etc/nginx/nginx.conf:118\\nnginx: [emerg] \\\"location\\\" directive is not allowed here in /etc/nginx/nginx.conf:118\\nnginx: configuration file /etc/nginx/nginx.conf test failed\\n\"",
							},
							{
								First:   t0.Add(-time.Hour).In(time.UTC),
								Last:    t0.Add(-time.Second).In(time.UTC),
								Count:   int32(16),
								Message: "Back-off restarting failed container",
							},
						},
					},
					{
						Name:         "my-instance-6f86f957b7-klmno",
						IP:           "172.16.100.20",
						HostIP:       "10.10.10.10",
						Status:       "Terminating",
						CreatedAt:    time.Date(2020, 4, 2, 16, 10, 0, 0, time.UTC),
						TerminatedAt: time.Date(2020, 4, 2, 16, 11, 0, 0, time.UTC),
						Ready:        true,
						Ports: []clientTypes.PodPort{
							{Name: "http", HostPort: int32(30000)},
							{Name: "https", HostPort: int32(30001)},
							{Name: "nginx-metrics", HostPort: int32(30002)},
						},
						Metrics: &clientTypes.PodMetrics{
							CPU:    "30m",
							Memory: "10Mi",
						},
					},
				}
				return info
			},
		},

		"w/ events": {
			resources: []runtime.Object{
				&nginxv1alpha1.Nginx{
					ObjectMeta: metav1.ObjectMeta{
						Name:              "my-instance",
						Namespace:         "rpaasv2",
						UID:               types.UID("my-instance"),
						CreationTimestamp: metav1.NewTime(t0.Add(-time.Hour)),
					},
				},
				&corev1.Event{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "my-instance.1",
						Namespace: "rpaasv2",
					},
					InvolvedObject: corev1.ObjectReference{
						Kind:      "Nginx",
						Name:      "my-instance",
						Namespace: "rpaasv2",
						UID:       types.UID("my-instance"),
					},
					FirstTimestamp: metav1.NewTime(t0.Add(-time.Hour)),
					LastTimestamp:  metav1.NewTime(t0.Add(-5 * time.Minute)),
					Count:          777,
					Type:           corev1.EventTypeWarning,
					Reason:         "ServiceQuotaExceeded",
					Message:        `failed to create Service: services "my-instance-service" is forbidden: exceeded quota: my-custom-quota, requested: services.loadbalancers=1, used: services.loadbalancers=1, limited: services.loadbalancers=1`,
				},
				&corev1.Event{
					ObjectMeta: metav1.ObjectMeta{
						Name:              "my-instance.2",
						Namespace:         "rpaasv2",
						CreationTimestamp: metav1.NewTime(t0.Add(-30 * time.Second)),
					},
					InvolvedObject: corev1.ObjectReference{
						Kind:      "Nginx",
						Name:      "my-instance",
						Namespace: "rpaasv2",
						UID:       types.UID("my-instance"),
					},
					FirstTimestamp: metav1.NewTime(t0.Add(-30 * time.Second)),
					LastTimestamp:  metav1.NewTime(t0.Add(-30 * time.Second)),
					Count:          1,
					Type:           corev1.EventTypeNormal,
					Reason:         "ServiceCreated",
					Message:        `service created successfully`,
				},
			},
			instance: func(i v1alpha1.RpaasInstance) v1alpha1.RpaasInstance {
				return i
			},
			expected: func(info clientTypes.InstanceInfo) clientTypes.InstanceInfo {
				info.Events = []clientTypes.Event{
					{
						First:   t0.Add(-time.Hour),
						Last:    t0.Add(-5 * time.Minute),
						Count:   777,
						Type:    "Warning",
						Reason:  "ServiceQuotaExceeded",
						Message: `failed to create Service: services "my-instance-service" is forbidden: exceeded quota: my-custom-quota, requested: services.loadbalancers=1, used: services.loadbalancers=1, limited: services.loadbalancers=1`,
					},
					{
						First:   t0.Add(-30 * time.Second),
						Last:    t0.Add(-30 * time.Second),
						Count:   1,
						Type:    corev1.EventTypeNormal,
						Reason:  "ServiceCreated",
						Message: `service created successfully`,
					},
				}
				return info
			},
		},

		"w/ ACLs": {
			instance: func(i v1alpha1.RpaasInstance) v1alpha1.RpaasInstance {
				i.Spec.AllowedUpstreams = []v1alpha1.AllowedUpstream{
					{Host: "169.196.254.254"},
					{Host: "my-app.apps.tsuru.io", Port: 80},
					{Host: "my-app.apps.tsuru.io", Port: 443},
				}
				return i
			},
			expected: func(info clientTypes.InstanceInfo) clientTypes.InstanceInfo {
				info.ACLs = []clientTypes.AllowedUpstream{
					{Host: "169.196.254.254"},
					{Host: "my-app.apps.tsuru.io", Port: 80},
					{Host: "my-app.apps.tsuru.io", Port: 443},
				}
				return info
			},
		},

		"w/ extra files": {
			resources: []runtime.Object{
				&corev1.ConfigMap{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "my-instance-extra-files-1",
						Namespace: "rpaasv2",
						Labels: map[string]string{
							"rpaas.extensions.tsuru.io/instance-name": "my-instance",
							"rpaas.extensions.tsuru.io/service-name":  "rpaasv2",
							"rpaas_instance":                          "my-instance",
							"rpaas_service":                           "rpaasv2",
							"rpaas.extensions.tsuru.io/is-file":       "true",
							"rpaas.extensions.tsuru.io/file-name":     "waf.cfg",
						},
					},
					BinaryData: map[string][]byte{
						"waf.cfg": []byte("My WAF rules :P"),
					},
				},
				&corev1.ConfigMap{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "my-instance-extra-files-2",
						Namespace: "rpaasv2",
						Labels: map[string]string{
							"rpaas.extensions.tsuru.io/instance-name": "my-instance",
							"rpaas.extensions.tsuru.io/service-name":  "rpaasv2",
							"rpaas_instance":                          "my-instance",
							"rpaas_service":                           "rpaasv2",
							"rpaas.extensions.tsuru.io/is-file":       "true",
							"rpaas.extensions.tsuru.io/file-name":     "binary.exe",
						},
					},
					BinaryData: map[string][]byte{
						"binary.exe": {66, 55, 10, 0},
					},
				},
			},
			instance: func(i v1alpha1.RpaasInstance) v1alpha1.RpaasInstance {
				i.Spec.Files = []v1alpha1.File{{Name: "waf.cfg"}, {Name: "binary.exe"}}
				return i
			},
			expected: func(info clientTypes.InstanceInfo) clientTypes.InstanceInfo {
				info.ExtraFiles = []clientTypes.RpaasFile{
					{Name: "binary.exe", Content: []byte{66, 55, 10, 0}},
					{Name: "waf.cfg", Content: []byte("My WAF rules :P")},
				}
				return info
			},
		},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			require.NotNil(t, tt.instance)
			instance := tt.instance(v1alpha1.RpaasInstance{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "my-instance",
					Namespace: "rpaasv2",
					Annotations: map[string]string{
						"rpaas.extensions.tsuru.io/description": "Some description about this instance",
						"rpaas.extensions.tsuru.io/tags":        "tag1,tag2,tag3",
						"rpaas.extensions.tsuru.io/team-owner":  "tsuru",
					},
					Labels: map[string]string{
						"rpaas_instance": "my-instance",
						"rpaas_service":  "rpaasv2-devel",
						"rpaas.extensions.tsuru.io/instance-name": "my-instance",
						"rpaas.extensions.tsuru.io/service-name":  "rpaasv2-devel",
					},
				},
				Spec: v1alpha1.RpaasInstanceSpec{
					PlanName: "huge",
					Flavors:  []string{"mango", "milk"},
				},
			})

			resources := append(tt.resources, &instance)

			client := fake.NewClientBuilder().
				WithScheme(newScheme()).
				WithRuntimeObjects(resources...).
				Build()

			var manager RpaasManager = &k8sRpaasManager{cli: client}
			if tt.manager != nil {
				manager = tt.manager(client)
			}

			got, err := manager.GetInstanceInfo(context.Background(), instance.Name)
			require.NoError(t, err)

			expected := tt.expected(clientTypes.InstanceInfo{
				Name:        "my-instance",
				Service:     "rpaasv2-devel",
				Dashboard:   "http://grafana.example/?instance_name=my-instance&service=rpaasv2-devel",
				Description: "Some description about this instance",
				Team:        "tsuru",
				Tags:        []string{"tag1", "tag2", "tag3"},
				Plan:        "huge",
				Flavors:     []string{"mango", "milk"},
			})
			assert.Equal(t, expected, *got)
		})
	}
}

func Test_k8sRpaasManager_GetPlans(t *testing.T) {
	cfg := config.Get()
	defer func() { config.Set(cfg) }()
	config.Set(config.RpaasConfig{
		LoadBalancerNameLabelKey: "cloudprovider.example/loadbalancer-name",
	})

	resources := []runtime.Object{
		&v1alpha1.RpaasPlan{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "default",
				Namespace: defaultNamespace,
			},
			Spec: v1alpha1.RpaasPlanSpec{
				Default:     true,
				Description: "Some description about \"default\"",
			},
		},
		&v1alpha1.RpaasPlan{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "plan-b",
				Namespace: defaultNamespace,
			},
			Spec: v1alpha1.RpaasPlanSpec{
				Description: "awesome description about plan-b",
			},
		},
		&v1alpha1.RpaasFlavor{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "system",
				Namespace: defaultNamespace,
			},
			Spec: v1alpha1.RpaasFlavorSpec{
				Default: true,
			},
		},
		&v1alpha1.RpaasFlavor{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "flavor-a",
				Namespace: defaultNamespace,
			},
			Spec: v1alpha1.RpaasFlavorSpec{
				Description: "description about flavor-a",
			},
		},
	}

	manager := &k8sRpaasManager{
		cli: fake.NewClientBuilder().WithScheme(newScheme()).WithRuntimeObjects(resources...).Build(),
	}
	plans, err := manager.GetPlans(context.TODO())
	require.NoError(t, err)

	p := map[string]interface{}{
		"$id":     "https://example.com/schema.json",
		"$schema": "https://json-schema.org/draft-07/schema#",
		"type":    "object",
		"properties": map[string]interface{}{
			"flavors": map[string]interface{}{
				"type": "array",
				"items": map[string]interface{}{
					"$ref": "#/definitions/flavor",
				},
				"description": `Provides a self-contained set of features that can be enabled on this plan. Example: flavors=flavor-a,flavor-b.
  supported flavors:
    - name: flavor-a
      description: description about flavor-a
`,
				"enum": []string{"flavor-a"},
			},
			"ip": map[string]interface{}{
				"type":        "string",
				"description": "IP address that will be assigned to load balancer. Example: ip=192.168.15.10.\n",
			},
			"plan-override": map[string]interface{}{
				"type":        "object",
				"description": "Allows an instance to change its plan parameters to specific ones. Examples: plan-override={\"config\": {\"cacheEnabled\": false}}; plan-override={\"image\": \"tsuru/nginx:latest\"}.\n",
			},
			"lb-name": map[string]interface{}{
				"type":        "string",
				"description": "Custom domain address (e.g. following RFC 1035) assigned to instance's load balancer. Example: lb-name=my-instance.internal.subdomain.example.\n",
			},
		},
		"definitions": map[string]interface{}{
			"flavor": map[string]interface{}{
				"type": "string",
			},
		},
	}
	schemas := &osb.Schemas{
		ServiceInstance: &osb.ServiceInstanceSchema{
			Create: &osb.InputParametersSchema{Parameters: p},
			Update: &osb.InputParametersSchema{Parameters: p},
		},
	}

	expected := []Plan{
		{
			Name:        "default",
			Description: "Some description about \"default\"",
			Schemas:     schemas,
		},
		{
			Name:        "plan-b",
			Description: "awesome description about plan-b",
			Schemas:     schemas,
		},
	}
	assert.Equal(t, expected, plans)
}

func Test_certificateName(t *testing.T) {
	tests := []struct {
		name     string
		expected string
	}{
		{expected: "default"},
		{name: "default", expected: "default"},
		{name: "*.example.com", expected: "example.com"},
		{name: ".example.com", expected: "example.com"},
		{name: "MY-dom4in.EXAMPLE.COM", expected: "my-dom4in.example.com"},
	}

	for _, tt := range tests {
		t.Run("", func(t *testing.T) {
			assert.Equal(t, tt.expected, certificateName(tt.name))
		})
	}
}

func Test_AddUpstream(t *testing.T) {
	instance1 := newEmptyRpaasInstance()
	instance1.ObjectMeta.Name = "instance1"
	instance1.Spec.AllowedUpstreams = []v1alpha1.AllowedUpstream{{Host: "host-1", Port: 8889}}
	instance2 := newEmptyRpaasInstance()
	instance2.ObjectMeta.Name = "instance2"

	resources := []runtime.Object{instance1, instance2}

	testCases := []struct {
		name      string
		instance  string
		upstream  v1alpha1.AllowedUpstream
		assertion func(*testing.T, error, *k8sRpaasManager)
	}{
		{
			name:     "updates an instance",
			instance: "instance1",
			upstream: v1alpha1.AllowedUpstream{
				Host: "host-2", Port: 8888,
			},
			assertion: func(t *testing.T, err error, m *k8sRpaasManager) {
				assert.NoError(t, err)

				instance := v1alpha1.RpaasInstance{}
				err = m.cli.Get(context.Background(), types.NamespacedName{Name: "instance1", Namespace: getServiceName()}, &instance)
				require.NoError(t, err)

				expectedItems := []v1alpha1.AllowedUpstream{
					{Host: "host-1", Port: 8889},
					{Host: "host-2", Port: 8888},
				}
				assert.Equal(t, expectedItems, instance.Spec.AllowedUpstreams)
			},
		},
		{
			name:     "creates an instance",
			instance: "instance2",
			upstream: v1alpha1.AllowedUpstream{
				Host: "host-3", Port: 8888,
			},
			assertion: func(t *testing.T, err error, m *k8sRpaasManager) {
				assert.NoError(t, err)

				instance := v1alpha1.RpaasInstance{}
				err = m.cli.Get(context.Background(), types.NamespacedName{Name: "instance2", Namespace: getServiceName()}, &instance)
				require.NoError(t, err)

				expectedItems := []v1alpha1.AllowedUpstream{
					{Host: "host-3", Port: 8888},
				}
				assert.Equal(t, expectedItems, instance.Spec.AllowedUpstreams)
			},
		},
		{
			name:     "conflict error",
			instance: "instance1",
			upstream: v1alpha1.AllowedUpstream{
				Host: "host-1", Port: 8889,
			},
			assertion: func(t *testing.T, err error, m *k8sRpaasManager) {
				assert.EqualError(t, err, "upstream already present in instance: instance1")
			},
		},

		{
			name:     "upstream without host",
			instance: "instance1",
			upstream: v1alpha1.AllowedUpstream{},
			assertion: func(t *testing.T, err error, m *k8sRpaasManager) {
				assert.Error(t, err)
				assert.Equal(t, &ValidationError{Msg: "cannot add an upstream with empty host"}, err)
			},
		},
	}

	for _, tt := range testCases {
		t.Run(tt.name, func(t *testing.T) {
			manager := &k8sRpaasManager{cli: fake.NewClientBuilder().WithScheme(newScheme()).WithRuntimeObjects(resources...).Build()}
			err := manager.AddUpstream(context.Background(), tt.instance, tt.upstream)
			tt.assertion(t, err, manager)
		})
	}
}

func Test_k8sRpaasManager_DeleteUpstream(t *testing.T) {
	scheme := runtime.NewScheme()
	corev1.AddToScheme(scheme)
	v1alpha1.SchemeBuilder.AddToScheme(scheme)

	instance1 := newEmptyRpaasInstance()
	instance1.ObjectMeta.Name = "instance1"
	instance1.Spec.AllowedUpstreams = []v1alpha1.AllowedUpstream{
		{Host: "host-1", Port: 8889},
		{Host: "host-2", Port: 8888},
	}

	resources := []runtime.Object{instance1}

	testCases := []struct {
		name      string
		instance  string
		upstream  v1alpha1.AllowedUpstream
		assertion func(*testing.T, error, *k8sRpaasManager)
	}{
		{
			name:     "updates an instance",
			instance: "instance1",
			upstream: v1alpha1.AllowedUpstream{
				Host: "host-2", Port: 8888,
			},
			assertion: func(t *testing.T, err error, m *k8sRpaasManager) {
				assert.NoError(t, err)

				instance := v1alpha1.RpaasInstance{}
				err = m.cli.Get(context.Background(), types.NamespacedName{Name: "instance1", Namespace: getServiceName()}, &instance)
				require.NoError(t, err)

				expectedItems := []v1alpha1.AllowedUpstream{
					{Host: "host-1", Port: 8889},
				}
				assert.Equal(t, expectedItems, instance.Spec.AllowedUpstreams)
			},
		},
		{
			name:     "error removing nonexistent",
			instance: "instance1",
			upstream: v1alpha1.AllowedUpstream{
				Host: "host-3", Port: 8888,
			},
			assertion: func(t *testing.T, err error, m *k8sRpaasManager) {
				assert.EqualError(t, err, "upstream not found inside list of allowed upstreams of instance1")
			},
		},
	}

	for _, tt := range testCases {
		t.Run(tt.name, func(t *testing.T) {
			manager := &k8sRpaasManager{cli: fake.NewClientBuilder().WithScheme(scheme).WithRuntimeObjects(resources...).Build()}
			err := manager.DeleteUpstream(context.Background(), tt.instance, tt.upstream)
			tt.assertion(t, err, manager)
		})
	}
}

func Test_k8sRpaasManager_GetAccessControlList(t *testing.T) {
	scheme := runtime.NewScheme()
	corev1.AddToScheme(scheme)
	v1alpha1.SchemeBuilder.AddToScheme(scheme)

	instance1 := newEmptyRpaasInstance()
	instance1.ObjectMeta.Name = "instance1"
	instance1.Spec.AllowedUpstreams = []v1alpha1.AllowedUpstream{
		{Host: "host-1", Port: 8889},
		{Host: "host-2", Port: 8888},
	}

	instance2 := newEmptyRpaasInstance()
	instance2.ObjectMeta.Name = "instance2"

	resources := []runtime.Object{instance1, instance2}

	testCases := []struct {
		name      string
		instance  string
		assertion func(*testing.T, error, []v1alpha1.AllowedUpstream, *k8sRpaasManager)
	}{
		{
			name:     "get an instance",
			instance: "instance1",
			assertion: func(t *testing.T, err error, upstreams []v1alpha1.AllowedUpstream, m *k8sRpaasManager) {
				assert.NoError(t, err)
				expectedUpstreams := []v1alpha1.AllowedUpstream{
					{Host: "host-1", Port: 8889},
					{Host: "host-2", Port: 8888},
				}
				assert.Equal(t, expectedUpstreams, instance1.Spec.AllowedUpstreams)
			},
		},
		{
			name:     "cannot get nonexistent instance",
			instance: "instance2",
			assertion: func(t *testing.T, err error, upstreams []v1alpha1.AllowedUpstream, m *k8sRpaasManager) {
				assert.NoError(t, err)
				assert.Empty(t, upstreams)
			},
		},
	}

	for _, tt := range testCases {
		t.Run(tt.name, func(t *testing.T) {
			manager := &k8sRpaasManager{cli: fake.NewClientBuilder().WithScheme(scheme).WithRuntimeObjects(resources...).Build()}
			instance, err := manager.GetUpstreams(context.Background(), tt.instance)
			tt.assertion(t, err, instance, manager)
		})
	}
}
