package rpaas

import (
	"crypto/tls"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	nginxv1alpha1 "github.com/tsuru/nginx-operator/pkg/apis/nginx/v1alpha1"
	"github.com/tsuru/rpaas-operator/pkg/apis/extensions/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	logf "sigs.k8s.io/controller-runtime/pkg/runtime/log"
)

func init() {
	logf.SetLogger(logf.ZapLogger(true))
}

func Test_k8sRpaasManager_DeleteBlock(t *testing.T) {
	scheme := runtime.NewScheme()
	corev1.AddToScheme(scheme)
	v1alpha1.SchemeBuilder.AddToScheme(scheme)

	instance1 := newEmptyRpaasInstance()

	instance2 := newEmptyRpaasInstance()
	instance2.ObjectMeta.Name = "another-instance"
	instance2.Spec.Blocks = map[v1alpha1.BlockType]v1alpha1.ConfigRef{
		v1alpha1.BlockTypeHTTP: v1alpha1.ConfigRef{
			Kind: v1alpha1.ConfigKindConfigMap,
			Name: "another-instance-blocks",
		},
	}

	cb := newEmptyConfigurationBlocks()
	cb.ObjectMeta.Name = "another-instance-blocks"
	cb.Data = map[string]string{
		"http": "# just a user configuration on http context",
	}

	resources := []runtime.Object{instance1, instance2, cb}

	testCases := []struct {
		instance  string
		block     string
		assertion func(*testing.T, error, *k8sRpaasManager)
	}{
		{
			"my-instance",
			"unknown-block",
			func(t *testing.T, err error, m *k8sRpaasManager) {
				assert.Error(t, err)
				assert.Equal(t, ErrBlockInvalid, err)
			},
		},
		{
			"another-instance",
			"http",
			func(t *testing.T, err error, m *k8sRpaasManager) {
				assert.NoError(t, err)

				cm := &corev1.ConfigMap{}
				err = m.cli.Get(nil, types.NamespacedName{Name: "another-instance-blocks", Namespace: "default"}, cm)
				require.NoError(t, err)
				_, ok := cm.Data["http"]
				assert.False(t, ok)

				ri := &v1alpha1.RpaasInstance{}
				err = m.cli.Get(nil, types.NamespacedName{Name: "another-instance", Namespace: "default"}, ri)
				require.NoError(t, err)
				_, ok = ri.Spec.Blocks[v1alpha1.BlockType("http")]
				assert.False(t, ok)
			},
		},
		{
			"my-instance",
			"server",
			func(t *testing.T, err error, m *k8sRpaasManager) {
				assert.Error(t, err)
				assert.Equal(t, ErrBlockIsNotDefined, err)
			},
		},
		{
			"another-instance",
			"server",
			func(t *testing.T, err error, m *k8sRpaasManager) {
				assert.Error(t, err)
				assert.Equal(t, ErrBlockIsNotDefined, err)
			},
		},
	}

	for _, testCase := range testCases {
		t.Run("", func(t *testing.T) {
			manager := &k8sRpaasManager{
				cli: fake.NewFakeClientWithScheme(scheme, resources...),
			}
			err := manager.DeleteBlock(nil, testCase.instance, testCase.block)
			testCase.assertion(t, err, manager)
		})
	}
}

func Test_k8sRpaasManager_ListBlocks(t *testing.T) {
	scheme := runtime.NewScheme()
	corev1.AddToScheme(scheme)
	v1alpha1.SchemeBuilder.AddToScheme(scheme)

	instance1 := newEmptyRpaasInstance()

	instance2 := newEmptyRpaasInstance()
	instance2.ObjectMeta.Name = "another-instance"
	instance2.Spec.Blocks = map[v1alpha1.BlockType]v1alpha1.ConfigRef{
		v1alpha1.BlockTypeHTTP: v1alpha1.ConfigRef{
			Kind: v1alpha1.ConfigKindConfigMap,
			Name: "another-instance-blocks",
		},
	}

	cb := newEmptyConfigurationBlocks()
	cb.ObjectMeta.Name = "another-instance-blocks"
	cb.Data = map[string]string{
		"http": "# just a user configuration on http context",
	}

	resources := []runtime.Object{instance1, instance2, cb}

	testCases := []struct {
		instance  string
		assertion func(*testing.T, []ConfigurationBlock, error)
	}{
		{
			"unknown-instance",
			func(t *testing.T, blocks []ConfigurationBlock, err error) {
				assert.Error(t, err)
			},
		},
		{
			"my-instance",
			func(t *testing.T, blocks []ConfigurationBlock, err error) {
				assert.NoError(t, err)
				assert.NotNil(t, blocks)
				assert.Len(t, blocks, 0)
			},
		},
		{
			"another-instance",
			func(t *testing.T, blocks []ConfigurationBlock, err error) {
				assert.NoError(t, err)
				expected := []ConfigurationBlock{
					{Name: "http", Content: "# just a user configuration on http context"},
				}
				assert.Equal(t, expected, blocks)
			},
		},
	}

	for _, testCase := range testCases {
		t.Run("", func(t *testing.T) {
			manager := &k8sRpaasManager{
				cli: fake.NewFakeClientWithScheme(scheme, resources...),
			}
			blocks, err := manager.ListBlocks(nil, testCase.instance)
			testCase.assertion(t, blocks, err)
		})
	}
}

func Test_k8sRpaasManager_UpdateBlock(t *testing.T) {
	scheme := runtime.NewScheme()
	corev1.AddToScheme(scheme)
	v1alpha1.SchemeBuilder.AddToScheme(scheme)

	instance1 := newEmptyRpaasInstance()

	instance2 := newEmptyRpaasInstance()
	instance2.ObjectMeta.Name = "another-instance"
	instance2.Spec.Blocks = map[v1alpha1.BlockType]v1alpha1.ConfigRef{
		v1alpha1.BlockTypeHTTP: v1alpha1.ConfigRef{
			Kind: v1alpha1.ConfigKindConfigMap,
			Name: "another-instance-blocks",
		},
	}

	cb := newEmptyConfigurationBlocks()
	cb.ObjectMeta.Name = "another-instance-blocks"
	cb.Data = map[string]string{
		"http": "# just a user configuration on http context",
	}

	resources := []runtime.Object{instance1, instance2, cb}

	testCases := []struct {
		instance  string
		block     ConfigurationBlock
		assertion func(*testing.T, error, *k8sRpaasManager)
	}{
		{
			"my-instance",
			ConfigurationBlock{Name: "unknown block"},
			func(t *testing.T, err error, m *k8sRpaasManager) {
				assert.Error(t, err)
				assert.Equal(t, ErrBlockInvalid, err)
			},
		},
		{
			"instance-not-found",
			ConfigurationBlock{Name: "root", Content: "# My root configuration"},
			func(t *testing.T, err error, m *k8sRpaasManager) {
				assert.Error(t, err)
				assert.True(t, IsNotFoundError(err))
			},
		},
		{
			"my-instance",
			ConfigurationBlock{Name: "http", Content: "# my custom http configuration"},
			func(t *testing.T, err error, m *k8sRpaasManager) {
				assert.NoError(t, err)

				ri := &v1alpha1.RpaasInstance{}
				err = m.cli.Get(nil, types.NamespacedName{Name: "my-instance", Namespace: "default"}, ri)
				require.NoError(t, err)
				expectedBlocks := map[v1alpha1.BlockType]v1alpha1.ConfigRef{
					v1alpha1.BlockTypeHTTP: v1alpha1.ConfigRef{
						Name: "my-instance-blocks",
						Kind: v1alpha1.ConfigKindConfigMap,
					},
				}
				assert.Equal(t, expectedBlocks, ri.Spec.Blocks)

				cm := &corev1.ConfigMap{}
				err = m.cli.Get(nil, types.NamespacedName{Name: "my-instance-blocks", Namespace: "default"}, cm)
				assert.NoError(t, err)
				expectedConfigMapData := map[string]string{"http": "# my custom http configuration"}
				assert.Equal(t, expectedConfigMapData, cm.Data)
			},
		},
		{
			"another-instance",
			ConfigurationBlock{Name: "server", Content: "# my custom server configuration"},
			func(t *testing.T, err error, m *k8sRpaasManager) {
				assert.NoError(t, err)

				ri := &v1alpha1.RpaasInstance{}
				err = m.cli.Get(nil, types.NamespacedName{Name: "another-instance", Namespace: "default"}, ri)
				require.NoError(t, err)
				expectedBlocks := map[v1alpha1.BlockType]v1alpha1.ConfigRef{
					v1alpha1.BlockTypeHTTP: v1alpha1.ConfigRef{
						Name: "another-instance-blocks",
						Kind: v1alpha1.ConfigKindConfigMap,
					},
					v1alpha1.BlockTypeServer: v1alpha1.ConfigRef{
						Name: "another-instance-blocks",
						Kind: v1alpha1.ConfigKindConfigMap,
					},
				}
				assert.Equal(t, expectedBlocks, ri.Spec.Blocks)

				cm := &corev1.ConfigMap{}
				err = m.cli.Get(nil, types.NamespacedName{Name: "another-instance-blocks", Namespace: "default"}, cm)
				require.NoError(t, err)

				expectedConfigMapData := map[string]string{
					"http":   "# just a user configuration on http context",
					"server": "# my custom server configuration",
				}
				assert.Equal(t, expectedConfigMapData, cm.Data)
			},
		},
	}

	for _, testCase := range testCases {
		t.Run("", func(t *testing.T) {
			manager := &k8sRpaasManager{
				cli: fake.NewFakeClientWithScheme(scheme, resources...),
			}
			err := manager.UpdateBlock(nil, testCase.instance, testCase.block)
			testCase.assertion(t, err, manager)
		})
	}
}

func Test_k8sRpaasManager_UpdateCertificate(t *testing.T) {
	scheme := runtime.NewScheme()
	corev1.AddToScheme(scheme)
	v1alpha1.SchemeBuilder.AddToScheme(scheme)

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
-----END CERTIFICATE-----
`

	ecdsaKeyPem := `-----BEGIN EC PRIVATE KEY-----
MHcCAQEEIIrYSSNQFaA2Hwf1duRSxKtLYX5CB04fSeQ6tF1aY/PuoAoGCCqGSM49
AwEHoUQDQgAEPR3tU2Fta9ktY+6P9G0cWO+0kETA6SFs38GecTyudlHz6xvCdz8q
EKTcWGekdmdDPsHloRNtsiCa697B2O9IFA==
-----END EC PRIVATE KEY-----
`

	ecdsaCertificate, err := tls.X509KeyPair([]byte(ecdsaCertPem), []byte(ecdsaKeyPem))
	require.NoError(t, err)

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
-----END CERTIFICATE-----
`

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
-----END RSA PRIVATE KEY-----
`

	rsaCertificate, err := tls.X509KeyPair([]byte(rsaCertPem), []byte(rsaKeyPem))
	require.NoError(t, err)

	instance1 := newEmptyRpaasInstance()

	instance2 := newEmptyRpaasInstance()
	instance2.Name = "another-instance"
	instance2.Spec.Certificates = &nginxv1alpha1.TLSSecret{
		SecretName: "another-instance-certificates",
		Items: []nginxv1alpha1.TLSSecretItem{
			{CertificateField: "default.crt", KeyField: "default.key"},
		},
	}

	secret := newEmptySecret()
	secret.Name = "another-instance-certificates"
	secret.Data = map[string][]byte{
		"default.crt": []byte(rsaCertPem),
		"default.key": []byte(rsaKeyPem),
	}

	resources := []runtime.Object{instance1, instance2, secret}

	testCases := []struct {
		name            string
		instanceName    string
		certificateName string
		certificate     tls.Certificate
		assertion       func(*testing.T, error, *k8sRpaasManager)
	}{
		{
			name:         "instance not found",
			instanceName: "instance-not-found",
			certificate:  ecdsaCertificate,
			assertion: func(t *testing.T, err error, m *k8sRpaasManager) {
				assert.Error(t, err)
				assert.True(t, IsNotFoundError(err))
			},
		},
		{
			name:         "adding a new certificate without name, should use default name \"default\"",
			instanceName: "my-instance",
			certificate:  rsaCertificate,
			assertion: func(t *testing.T, err error, m *k8sRpaasManager) {
				require.NoError(t, err)

				instance := v1alpha1.RpaasInstance{}
				err = m.cli.Get(nil, types.NamespacedName{
					Name:      "my-instance",
					Namespace: "default",
				}, &instance)
				require.NoError(t, err)

				assert.NotNil(t, instance.Spec.Certificates)
				assert.NotEmpty(t, instance.Spec.Certificates.SecretName)

				expectedCertificates := &nginxv1alpha1.TLSSecret{
					SecretName: instance.Spec.Certificates.SecretName,
					Items: []nginxv1alpha1.TLSSecretItem{
						{CertificateField: "default.crt", KeyField: "default.key"},
					},
				}
				assert.Equal(t, expectedCertificates, instance.Spec.Certificates)

				secret := corev1.Secret{}
				err = m.cli.Get(nil, types.NamespacedName{
					Name:      instance.Spec.Certificates.SecretName,
					Namespace: "default",
				}, &secret)
				require.NoError(t, err)

				expectedSecretData := map[string][]byte{
					"default.crt": []byte(rsaCertPem),
					"default.key": []byte(rsaKeyPem),
				}
				assert.Equal(t, expectedSecretData, secret.Data)
			},
		},
		{
			name:            "adding a new certificate with a custom name",
			instanceName:    "my-instance",
			certificateName: "custom-name",
			certificate:     ecdsaCertificate,
			assertion: func(t *testing.T, err error, m *k8sRpaasManager) {
				require.NoError(t, err)

				instance := v1alpha1.RpaasInstance{}
				err = m.cli.Get(nil, types.NamespacedName{
					Name:      "my-instance",
					Namespace: "default",
				}, &instance)
				require.NoError(t, err)

				assert.NotNil(t, instance.Spec.Certificates)
				assert.NotEmpty(t, instance.Spec.Certificates.SecretName)

				expectedCertificates := &nginxv1alpha1.TLSSecret{
					SecretName: instance.Spec.Certificates.SecretName,
					Items: []nginxv1alpha1.TLSSecretItem{
						{CertificateField: "custom-name.crt", KeyField: "custom-name.key"},
					},
				}
				assert.Equal(t, expectedCertificates, instance.Spec.Certificates)

				secret := corev1.Secret{}
				err = m.cli.Get(nil, types.NamespacedName{
					Name:      instance.Spec.Certificates.SecretName,
					Namespace: "default",
				}, &secret)
				require.NoError(t, err)

				expectedSecretData := map[string][]byte{
					"custom-name.crt": []byte(ecdsaCertPem),
					"custom-name.key": []byte(ecdsaKeyPem),
				}
				assert.Equal(t, expectedSecretData, secret.Data)

			},
		},
		{
			name:         "updating an existing certificate from RSA to ECDSA",
			instanceName: "another-instance",
			certificate:  ecdsaCertificate,
			assertion: func(t *testing.T, err error, m *k8sRpaasManager) {
				require.NoError(t, err)

				instance := v1alpha1.RpaasInstance{}
				err = m.cli.Get(nil, types.NamespacedName{
					Name:      "another-instance",
					Namespace: "default",
				}, &instance)
				require.NoError(t, err)

				assert.NotNil(t, instance.Spec.Certificates)
				assert.NotEmpty(t, instance.Spec.Certificates.SecretName)

				expectedCertificates := &nginxv1alpha1.TLSSecret{
					SecretName: instance.Spec.Certificates.SecretName,
					Items: []nginxv1alpha1.TLSSecretItem{
						{CertificateField: "default.crt", KeyField: "default.key"},
					},
				}
				assert.Equal(t, expectedCertificates, instance.Spec.Certificates)

				secret := corev1.Secret{}
				err = m.cli.Get(nil, types.NamespacedName{
					Name:      instance.Spec.Certificates.SecretName,
					Namespace: "default",
				}, &secret)
				require.NoError(t, err)

				expectedSecretData := map[string][]byte{
					"default.crt": []byte(ecdsaCertPem),
					"default.key": []byte(ecdsaKeyPem),
				}
				assert.Equal(t, expectedSecretData, secret.Data)
			},
		},
		{
			name:            "adding multiple certificates",
			instanceName:    "another-instance",
			certificateName: "custom-name",
			certificate:     ecdsaCertificate,
			assertion: func(t *testing.T, err error, m *k8sRpaasManager) {
				require.NoError(t, err)

				instance := v1alpha1.RpaasInstance{}
				err = m.cli.Get(nil, types.NamespacedName{
					Name:      "another-instance",
					Namespace: "default",
				}, &instance)
				require.NoError(t, err)
				assert.NotNil(t, instance.Spec.Certificates)
				assert.NotEmpty(t, instance.Spec.Certificates.SecretName)

				expectedCertificates := &nginxv1alpha1.TLSSecret{
					SecretName: instance.Spec.Certificates.SecretName,
					Items: []nginxv1alpha1.TLSSecretItem{
						{CertificateField: "default.crt", KeyField: "default.key"},
						{CertificateField: "custom-name.crt", KeyField: "custom-name.key"},
					},
				}
				assert.Equal(t, expectedCertificates, instance.Spec.Certificates)

				secret := corev1.Secret{}
				err = m.cli.Get(nil, types.NamespacedName{
					Name:      instance.Spec.Certificates.SecretName,
					Namespace: "default",
				}, &secret)
				require.NoError(t, err)

				expectedSecretData := map[string][]byte{
					"default.crt":     []byte(rsaCertPem),
					"default.key":     []byte(rsaKeyPem),
					"custom-name.crt": []byte(ecdsaCertPem),
					"custom-name.key": []byte(ecdsaKeyPem),
				}
				assert.Equal(t, expectedSecretData, secret.Data)
			},
		},
		{
			name:         "updating to the same certificate, should do nothing",
			instanceName: "another-instance",
			certificate:  rsaCertificate,
			assertion: func(t *testing.T, err error, m *k8sRpaasManager) {
				assert.Error(t, err)
				assert.Equal(t, &ConflictError{Msg: "certificate \"default\" already is deployed"}, err)
			},
		},
	}

	for _, tt := range testCases {
		t.Run(tt.name, func(t *testing.T) {
			manager := &k8sRpaasManager{cli: fake.NewFakeClientWithScheme(scheme, resources...)}
			err := manager.UpdateCertificate(nil, tt.instanceName, tt.certificateName, tt.certificate)
			tt.assertion(t, err, manager)
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
			Namespace: "default",
		},
		Spec: v1alpha1.RpaasInstanceSpec{},
	}
}

func newEmptyConfigurationBlocks() *corev1.ConfigMap {
	return &corev1.ConfigMap{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "v1",
			Kind:       "ConfigMap",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "my-instance-blocks",
			Namespace: "default",
		},
	}
}

func newEmptyExtraFiles() *corev1.ConfigMap {
	return &corev1.ConfigMap{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "v1",
			Kind:       "ConfigMap",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "my-instance-extra-files",
			Namespace: "default",
		},
	}
}

func newEmptySecret() *corev1.Secret {
	return &corev1.Secret{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "v1",
			Kind:       "Secret",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "my-secrets",
			Namespace: "default",
		},
	}
}

func Test_k8sRpaasManager_GetInstanceAddress(t *testing.T) {
	scheme := runtime.NewScheme()
	corev1.AddToScheme(scheme)
	v1alpha1.SchemeBuilder.AddToScheme(scheme)
	nginxv1alpha1.SchemeBuilder.AddToScheme(scheme)

	instance1 := newEmptyRpaasInstance()
	instance2 := newEmptyRpaasInstance()
	instance2.ObjectMeta.Name = "another-instance"
	instance3 := newEmptyRpaasInstance()
	instance3.ObjectMeta.Name = "instance3"
	instance4 := newEmptyRpaasInstance()
	instance4.ObjectMeta.Name = "instance4"
	nginx1 := &nginxv1alpha1.Nginx{
		ObjectMeta: instance1.ObjectMeta,
		Status: nginxv1alpha1.NginxStatus{
			Services: []nginxv1alpha1.ServiceStatus{
				{Name: "svc1"},
			},
		},
	}
	nginx2 := &nginxv1alpha1.Nginx{
		ObjectMeta: instance2.ObjectMeta,
		Status: nginxv1alpha1.NginxStatus{
			Services: []nginxv1alpha1.ServiceStatus{
				{Name: "svc2"},
			},
		},
	}
	nginx3 := &nginxv1alpha1.Nginx{
		ObjectMeta: instance3.ObjectMeta,
		Status: nginxv1alpha1.NginxStatus{
			Services: []nginxv1alpha1.ServiceStatus{},
		},
	}
	svc1 := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "svc1",
			Namespace: instance1.Namespace,
		},
		Spec: corev1.ServiceSpec{
			ClusterIP: "10.1.1.9",
		},
		Status: corev1.ServiceStatus{
			LoadBalancer: corev1.LoadBalancerStatus{
				Ingress: []corev1.LoadBalancerIngress{
					{IP: "10.1.2.3"},
				},
			},
		},
	}
	svc2 := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "svc2",
			Namespace: instance1.Namespace,
		},
		Spec: corev1.ServiceSpec{
			ClusterIP: "10.1.1.9",
		},
	}

	resources := []runtime.Object{instance1, instance2, instance3, instance4, nginx1, nginx2, nginx3, svc1, svc2}

	testCases := []struct {
		instance  string
		assertion func(*testing.T, string, error)
	}{
		{
			"my-instance",
			func(t *testing.T, address string, err error) {
				assert.NoError(t, err)
				assert.Equal(t, address, "10.1.2.3")
			},
		},
		{
			"another-instance",
			func(t *testing.T, address string, err error) {
				assert.NoError(t, err)
				assert.Equal(t, address, "10.1.1.9")
			},
		},
		{
			"instance3",
			func(t *testing.T, address string, err error) {
				assert.NoError(t, err)
				assert.Equal(t, address, "")
			},
		},
		{
			"instance4",
			func(t *testing.T, address string, err error) {
				assert.NoError(t, err)
				assert.Equal(t, address, "")
			},
		},
		{
			"not-found-instance",
			func(t *testing.T, address string, err error) {
				assert.Error(t, err)
				assert.True(t, IsNotFoundError(err))
			},
		},
	}

	for _, testCase := range testCases {
		t.Run("", func(t *testing.T) {
			manager := &k8sRpaasManager{
				cli: fake.NewFakeClientWithScheme(scheme, resources...),
			}
			address, err := manager.GetInstanceAddress(nil, testCase.instance)
			testCase.assertion(t, address, err)
		})
	}
}

func Test_k8sRpaasManager_GetInstanceStatus(t *testing.T) {
	scheme := runtime.NewScheme()
	corev1.AddToScheme(scheme)
	v1alpha1.SchemeBuilder.AddToScheme(scheme)
	nginxv1alpha1.SchemeBuilder.AddToScheme(scheme)

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
			Pods: []nginxv1alpha1.PodStatus{
				{Name: "pod1"},
				{Name: "pod2"},
			},
		},
	}
	nginx2 := &nginxv1alpha1.Nginx{
		ObjectMeta: instance2.ObjectMeta,
		Status: nginxv1alpha1.NginxStatus{
			Pods: []nginxv1alpha1.PodStatus{
				{Name: "pod3"},
			},
		},
	}
	nginx3 := &nginxv1alpha1.Nginx{
		ObjectMeta: instance3.ObjectMeta,
		Status: nginxv1alpha1.NginxStatus{
			Pods: []nginxv1alpha1.PodStatus{},
		},
	}
	nginx4 := &nginxv1alpha1.Nginx{
		ObjectMeta: instance5.ObjectMeta,
		Status: nginxv1alpha1.NginxStatus{
			Pods: []nginxv1alpha1.PodStatus{
				{Name: "pod4"},
			},
		},
	}
	pod1 := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "pod1",
			Namespace: instance1.Namespace,
		},
		Status: corev1.PodStatus{
			PodIP: "10.0.0.1",
		},
	}
	pod2 := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "pod2",
			Namespace: instance1.Namespace,
		},
		Status: corev1.PodStatus{
			PodIP: "10.0.0.2",
		},
	}
	pod4 := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "pod4",
			Namespace: instance1.Namespace,
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
			Name: "pod1",
			Kind: "Pod",
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
			Name: "pod1",
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
			"instance2",
			func(t *testing.T, podMap PodStatusMap, err error) {
				assert.NoError(t, err)
				assert.Equal(t, podMap, PodStatusMap{
					"pod3": PodStatus{
						Running: false,
						Status:  "pods \"pod3\" not found",
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
			fakeCli := fake.NewFakeClientWithScheme(scheme, resources...)
			manager := &k8sRpaasManager{
				nonCachedCli: fakeCli,
				cli:          fakeCli,
			}
			podMap, err := manager.GetInstanceStatus(nil, testCase.instance)
			testCase.assertion(t, podMap, err)
		})
	}
}

func Test_k8sRpaasManager_CreateExtraFiles(t *testing.T) {
	scheme := runtime.NewScheme()
	corev1.AddToScheme(scheme)
	v1alpha1.SchemeBuilder.AddToScheme(scheme)
	nginxv1alpha1.SchemeBuilder.AddToScheme(scheme)

	instance1 := newEmptyRpaasInstance()
	instance2 := newEmptyRpaasInstance()
	instance2.Name = "another-instance"
	instance2.Spec.ExtraFiles = &nginxv1alpha1.FilesRef{
		Name: "another-instance-extra-files",
		Files: map[string]string{
			"index.html": "index.html",
		},
	}

	configMap := newEmptyExtraFiles()
	configMap.Name = "another-instance-extra-files"
	configMap.BinaryData = map[string][]byte{
		"index.html": []byte("Hello world"),
	}

	resources := []runtime.Object{instance1, instance2, configMap}

	testCases := []struct {
		instance  string
		files     []File
		assertion func(*testing.T, error, *k8sRpaasManager)
	}{
		{
			instance: "my-instance",
			files: []File{
				{
					Name:    "/path/to/my/file",
					Content: []byte("My invalid filename"),
				},
			},
			assertion: func(t *testing.T, err error, m *k8sRpaasManager) {
				assert.Error(t, err)
				assert.True(t, IsValidationError(err))
			},
		},
		{
			instance: "my-instance",
			files: []File{
				{
					Name:    "www/index.html",
					Content: []byte("<h1>Hello world!</h1>"),
				},
				{
					Name:    "waf/sqli-rules.cnf",
					Content: []byte("# my awesome rules against SQLi :)..."),
				},
			},
			assertion: func(t *testing.T, err error, m *k8sRpaasManager) {
				assert.NoError(t, err)

				instance := v1alpha1.RpaasInstance{}
				err = m.cli.Get(nil, types.NamespacedName{Name: "my-instance", Namespace: "default"}, &instance)
				require.NoError(t, err)

				expectedFiles := map[string]string{
					"www_index.html":     "www/index.html",
					"waf_sqli-rules.cnf": "waf/sqli-rules.cnf",
				}
				assert.Equal(t, expectedFiles, instance.Spec.ExtraFiles.Files)

				cm, err := m.getExtraFiles(nil, instance)
				assert.NoError(t, err)
				expectedConfigMapData := map[string][]byte{
					"www_index.html":     []byte("<h1>Hello world!</h1>"),
					"waf_sqli-rules.cnf": []byte("# my awesome rules against SQLi :)..."),
				}
				assert.Equal(t, expectedConfigMapData, cm.BinaryData)
			},
		},
		{
			instance: "another-instance",
			files: []File{
				{
					Name:    "index.html",
					Content: []byte("My new hello world"),
				},
			},
			assertion: func(t *testing.T, err error, m *k8sRpaasManager) {
				assert.Error(t, err)
				assert.True(t, IsConflictError(err))
				assert.Equal(t, &ConflictError{Msg: `file "index.html" already exists`}, err)
			},
		},
		{
			instance: "another-instance",
			files: []File{
				{
					Name:    "www/index.html",
					Content: []byte("<h1>Hello world!</h1>"),
				},
			},
			assertion: func(t *testing.T, err error, m *k8sRpaasManager) {
				assert.NoError(t, err)

				instance := v1alpha1.RpaasInstance{}
				err = m.cli.Get(nil, types.NamespacedName{Name: "another-instance", Namespace: "default"}, &instance)
				require.NoError(t, err)

				assert.NotEqual(t, "another-instance-extra-files", instance.Spec.ExtraFiles.Name)
				expectedFiles := map[string]string{
					"index.html":     "index.html",
					"www_index.html": "www/index.html",
				}
				assert.Equal(t, expectedFiles, instance.Spec.ExtraFiles.Files)

				cm, err := m.getExtraFiles(nil, instance)
				require.NoError(t, err)

				expectedConfigMapData := map[string][]byte{
					"index.html":     []byte("Hello world"),
					"www_index.html": []byte("<h1>Hello world!</h1>"),
				}
				assert.Equal(t, expectedConfigMapData, cm.BinaryData)
			},
		},
	}

	for _, tt := range testCases {
		t.Run("", func(t *testing.T) {
			manager := &k8sRpaasManager{cli: fake.NewFakeClientWithScheme(scheme, resources...)}
			err := manager.CreateExtraFiles(nil, tt.instance, tt.files...)
			tt.assertion(t, err, manager)
		})
	}
}

func Test_k8sRpaasManager_GetExtraFiles(t *testing.T) {
	scheme := runtime.NewScheme()
	corev1.AddToScheme(scheme)
	v1alpha1.SchemeBuilder.AddToScheme(scheme)
	nginxv1alpha1.SchemeBuilder.AddToScheme(scheme)

	instance1 := newEmptyRpaasInstance()

	instance2 := newEmptyRpaasInstance()
	instance2.Name = "another-instance"
	instance2.Spec.ExtraFiles = &nginxv1alpha1.FilesRef{
		Name: "another-instance-extra-files",
		Files: map[string]string{
			"index.html": "index.html",
		},
	}

	configMap := newEmptyExtraFiles()
	configMap.Name = "another-instance-extra-files"
	configMap.BinaryData = map[string][]byte{
		"index.html": []byte("Hello world"),
	}

	resources := []runtime.Object{instance1, instance2, configMap}

	testCases := []struct {
		instance      string
		expectedFiles []File
		expectedError error
	}{
		{
			instance:      "my-instance",
			expectedFiles: []File{},
		},
		{
			instance: "another-instance",
			expectedFiles: []File{
				{
					Name:    "index.html",
					Content: []byte("Hello world"),
				},
			},
		},
	}

	for _, tt := range testCases {
		t.Run("", func(t *testing.T) {
			manager := &k8sRpaasManager{cli: fake.NewFakeClientWithScheme(scheme, resources...)}
			files, err := manager.GetExtraFiles(nil, tt.instance)
			assert.NoError(t, err)
			assert.Equal(t, tt.expectedFiles, files)
		})
	}
}

func Test_k8sRpaasManager_UpdateExtraFiles(t *testing.T) {
	scheme := runtime.NewScheme()
	corev1.AddToScheme(scheme)
	v1alpha1.SchemeBuilder.AddToScheme(scheme)
	nginxv1alpha1.SchemeBuilder.AddToScheme(scheme)

	instance1 := newEmptyRpaasInstance()

	instance2 := newEmptyRpaasInstance()
	instance2.Name = "another-instance"
	instance2.Spec.ExtraFiles = &nginxv1alpha1.FilesRef{
		Name: "another-instance-extra-files",
		Files: map[string]string{
			"index.html": "index.html",
		},
	}

	configMap := newEmptyExtraFiles()
	configMap.Name = "another-instance-extra-files"
	configMap.BinaryData = map[string][]byte{
		"index.html": []byte("Hello world"),
	}

	resources := []runtime.Object{instance1, instance2, configMap}

	testCases := []struct {
		instance  string
		files     []File
		assertion func(*testing.T, error, *k8sRpaasManager)
	}{
		{
			instance: "my-instance",
			files: []File{
				{
					Name:    "www/index.html",
					Content: []byte("<h1>Hello world!</h1>"),
				},
			},
			assertion: func(t *testing.T, err error, m *k8sRpaasManager) {
				assert.Error(t, err)
				assert.Equal(t, &NotFoundError{Msg: "there are no extra files"}, err)
			},
		},
		{
			instance: "another-instance",
			files: []File{
				{
					Name:    "www/index.html",
					Content: []byte("<h1>Hello world!</h1>"),
				},
			},
			assertion: func(t *testing.T, err error, m *k8sRpaasManager) {
				assert.Error(t, err)
				assert.Equal(t, &NotFoundError{Msg: `file "www/index.html" does not exist`}, err)
			},
		},
		{
			instance: "another-instance",
			files: []File{
				{
					Name:    "index.html",
					Content: []byte("<h1>Hello world!</h1>"),
				},
			},
			assertion: func(t *testing.T, err error, m *k8sRpaasManager) {
				assert.NoError(t, err)

				instance := v1alpha1.RpaasInstance{}
				err = m.cli.Get(nil, types.NamespacedName{Name: "another-instance", Namespace: "default"}, &instance)
				require.NoError(t, err)

				cm, err := m.getExtraFiles(nil, instance)
				require.NoError(t, err)

				expectedConfigMapData := map[string][]byte{
					"index.html": []byte("<h1>Hello world!</h1>"),
				}
				assert.Equal(t, expectedConfigMapData, cm.BinaryData)

			},
		},
	}

	for _, tt := range testCases {
		t.Run("", func(t *testing.T) {
			manager := &k8sRpaasManager{cli: fake.NewFakeClientWithScheme(scheme, resources...)}
			err := manager.UpdateExtraFiles(nil, tt.instance, tt.files...)
			tt.assertion(t, err, manager)
		})
	}
}

func Test_k8sRpaasManager_DeleteExtraFiles(t *testing.T) {
	scheme := runtime.NewScheme()
	corev1.AddToScheme(scheme)
	v1alpha1.SchemeBuilder.AddToScheme(scheme)
	nginxv1alpha1.SchemeBuilder.AddToScheme(scheme)

	instance1 := newEmptyRpaasInstance()
	instance1.Spec.ExtraFiles = &nginxv1alpha1.FilesRef{
		Name: "my-instance-extra-files",
		Files: map[string]string{
			"index.html":     "index.html",
			"waf_rules.conf": "waf/rules.conf",
		},
	}

	instance2 := newEmptyRpaasInstance()
	instance2.Name = "another-instance"

	configMap := newEmptyExtraFiles()
	configMap.Name = "my-instance-extra-files"
	configMap.BinaryData = map[string][]byte{
		"index.html":     []byte("Hello world"),
		"waf_rules.conf": []byte("# my awesome WAF rules"),
	}

	resources := []runtime.Object{instance1, instance2, configMap}

	testCases := []struct {
		instance  string
		filenames []string
		assertion func(*testing.T, error, *k8sRpaasManager)
	}{
		{
			instance:  "another-instance",
			filenames: []string{"whatever-file.txt"},
			assertion: func(t *testing.T, err error, m *k8sRpaasManager) {
				assert.Error(t, err)
				assert.Equal(t, &NotFoundError{Msg: `there are no extra files`}, err)
			},
		},
		{
			instance:  "my-instance",
			filenames: []string{"index.html", "waf_rules.conf"},
			assertion: func(t *testing.T, err error, m *k8sRpaasManager) {
				assert.NoError(t, err)

				instance := v1alpha1.RpaasInstance{}
				err = m.cli.Get(nil, types.NamespacedName{Name: "my-instance", Namespace: "default"}, &instance)
				require.NoError(t, err)
				assert.Nil(t, instance.Spec.ExtraFiles)
			},
		},
		{
			instance:  "my-instance",
			filenames: []string{"not-found.txt"},
			assertion: func(t *testing.T, err error, m *k8sRpaasManager) {
				assert.Error(t, err)
				assert.Equal(t, &NotFoundError{Msg: `file "not-found.txt" does not exist`}, err)
			},
		},
	}

	for _, tt := range testCases {
		t.Run("", func(t *testing.T) {
			manager := &k8sRpaasManager{cli: fake.NewFakeClientWithScheme(scheme, resources...)}
			err := manager.DeleteExtraFiles(nil, tt.instance, tt.filenames...)
			tt.assertion(t, err, manager)
		})
	}
}

func Test_k8sRpaasManager_BindApp(t *testing.T) {
	instance1 := newEmptyRpaasInstance()

	instance2 := newEmptyRpaasInstance()
	instance2.Name = "another-instance"
	instance2.Spec.Host = "app2.tsuru.example.com"

	scheme := newScheme()
	resources := []runtime.Object{instance1, instance2}

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
			name:     "when AppHost field is not defined",
			instance: "my-instance",
			args:     BindAppArgs{},
			assertion: func(t *testing.T, err error, _ v1alpha1.RpaasInstance) {
				assert.Error(t, err)
				expected := &ValidationError{Msg: "application host cannot be empty"}
				assert.Equal(t, expected, err)
			},
		},
		{
			name:     "when instance successfully bound with an application",
			instance: "my-instance",
			args: BindAppArgs{
				AppHost: "app1.tsuru.example.com",
			},
			assertion: func(t *testing.T, err error, ri v1alpha1.RpaasInstance) {
				assert.NoError(t, err)
				assert.Equal(t, "app1.tsuru.example.com", ri.Spec.Host)
			},
		},
		{
			name:     "when instance already bound with another application",
			instance: "another-instance",
			args: BindAppArgs{
				AppHost: "app1.tsuru.example.com",
			},
			assertion: func(t *testing.T, err error, _ v1alpha1.RpaasInstance) {
				assert.Error(t, err)
				expected := &ConflictError{Msg: "instance already bound with another application"}
				assert.Equal(t, expected, err)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			manager := &k8sRpaasManager{cli: fake.NewFakeClientWithScheme(scheme, resources...)}
			bindAppErr := manager.BindApp(nil, tt.instance, tt.args)

			var instance v1alpha1.RpaasInstance

			if bindAppErr == nil {
				require.NoError(t, manager.cli.Get(nil, types.NamespacedName{
					Name:      tt.instance,
					Namespace: "default",
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
	instance2.Spec.Host = "app2.tsuru.example.com"

	scheme := newScheme()
	resources := []runtime.Object{instance1, instance2}

	tests := []struct {
		name      string
		instance  string
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
			name:     "when instance bound with no application",
			instance: "my-instance",
			assertion: func(t *testing.T, err error, _ v1alpha1.RpaasInstance) {
				assert.Error(t, err)
				expected := &ValidationError{Msg: "instance not bound"}
				assert.Equal(t, expected, err)
			},
		},
		{
			name:     "when instance successfully unbound",
			instance: "another-instance",
			assertion: func(t *testing.T, err error, ri v1alpha1.RpaasInstance) {
				assert.NoError(t, err)
				assert.Equal(t, "", ri.Spec.Host)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			manager := &k8sRpaasManager{cli: fake.NewFakeClientWithScheme(scheme, resources...)}
			unbindAppErr := manager.UnbindApp(nil, tt.instance)

			var instance v1alpha1.RpaasInstance

			if unbindAppErr == nil {
				require.NoError(t, manager.cli.Get(nil, types.NamespacedName{
					Name:      tt.instance,
					Namespace: "default",
				}, &instance))
			}

			tt.assertion(t, unbindAppErr, instance)
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

func newScheme() *runtime.Scheme {
	scheme := runtime.NewScheme()
	corev1.AddToScheme(scheme)
	v1alpha1.SchemeBuilder.AddToScheme(scheme)
	nginxv1alpha1.SchemeBuilder.AddToScheme(scheme)
	return scheme
}
