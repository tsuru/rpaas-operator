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
			err := manager.DeleteBlock(testCase.instance, testCase.block)
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
			blocks, err := manager.ListBlocks(testCase.instance)
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
			err := manager.UpdateBlock(testCase.instance, testCase.block)
			testCase.assertion(t, err, manager)
		})
	}
}

func Test_k8sRpaasManager_UpdateCertificate(t *testing.T) {
	scheme := runtime.NewScheme()
	corev1.AddToScheme(scheme)
	v1alpha1.SchemeBuilder.AddToScheme(scheme)

	instance := "my-instance"
	namespace := "custom-namespace"

	rpaasInstance := &v1alpha1.RpaasInstance{
		TypeMeta: metav1.TypeMeta{
			Kind:       "RpaasInstance",
			APIVersion: "extensions.tsuru.io/v1alpha1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      instance,
			Namespace: namespace,
		},
	}

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

	assertSecretData := func(t *testing.T, m *k8sRpaasManager, expectedCertName string, expectedCertData, expectedKeyData []byte) {
		ri, err := m.GetInstance(instance)
		require.NoError(t, err)
		secret, err := m.getCertificateSecret(*ri, expectedCertName)
		require.NoError(t, err)
		gotCertData, ok := secret.Data["certificate"]
		assert.True(t, ok)
		assert.Equal(t, expectedCertData, gotCertData)
		gotKeyData, ok := secret.Data["key"]
		assert.True(t, ok)
		assert.Equal(t, expectedKeyData, gotKeyData)
	}

	assertTLSCertificate := func(t *testing.T, m *k8sRpaasManager, expectedName string) {
		ri, err := m.GetInstance(instance)
		require.NoError(t, err)
		secret, err := m.getCertificateSecret(*ri, expectedName)
		require.NoError(t, err)
		expectesTLSSecret := nginxv1alpha1.TLSSecret{
			SecretName:       secret.ObjectMeta.Name,
			CertificateField: "certificate",
			CertificatePath:  expectedName + ".crt.pem",
			KeyField:         "key",
			KeyPath:          expectedName + ".key.pem",
		}
		gotCertificate, ok := ri.Spec.Certificates[expectedName]
		assert.True(t, ok)
		assert.Equal(t, expectesTLSSecret, gotCertificate)
	}

	testCases := []struct {
		name        string
		instance    string
		certificate tls.Certificate
		setup       func(*testing.T, *k8sRpaasManager)
		assertion   func(*testing.T, error, *k8sRpaasManager)
	}{
		{
			instance:    "instance-not-found",
			certificate: ecdsaCertificate,
			assertion: func(t *testing.T, err error, m *k8sRpaasManager) {
				assert.Error(t, err)
				assert.True(t, IsNotFoundError(err))
			},
		},
		{
			instance:    instance,
			certificate: ecdsaCertificate,
			assertion: func(t *testing.T, err error, m *k8sRpaasManager) {
				require.NoError(t, err)
				assertSecretData(t, m, "default", []byte(ecdsaCertPem), []byte(ecdsaKeyPem))
				assertTLSCertificate(t, m, "default")
			},
		},
		{
			name:        "mycert",
			instance:    instance,
			certificate: ecdsaCertificate,
			assertion: func(t *testing.T, err error, m *k8sRpaasManager) {
				require.NoError(t, err)
				assertSecretData(t, m, "mycert", []byte(ecdsaCertPem), []byte(ecdsaKeyPem))
				assertTLSCertificate(t, m, "mycert")
			},
		},
		{
			instance:    instance,
			certificate: rsaCertificate,
			setup: func(t *testing.T, m *k8sRpaasManager) {
				err := m.UpdateCertificate(instance, "", ecdsaCertificate)
				require.NoError(t, err)
				assertSecretData(t, m, "default", []byte(ecdsaCertPem), []byte(ecdsaKeyPem))
				assertTLSCertificate(t, m, "default")
			},
			assertion: func(t *testing.T, err error, m *k8sRpaasManager) {
				require.NoError(t, err)
				assertSecretData(t, m, "default", []byte(rsaCertPem), []byte(rsaKeyPem))
				assertTLSCertificate(t, m, "default")
			},
		},
	}

	for _, tt := range testCases {
		t.Run("", func(t *testing.T) {
			manager := &k8sRpaasManager{
				cli: fake.NewFakeClientWithScheme(scheme, rpaasInstance),
			}
			if tt.setup != nil {
				tt.setup(t, manager)
			}
			err := manager.UpdateCertificate(tt.instance, tt.name, tt.certificate)
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
			address, err := manager.GetInstanceAddress(testCase.instance)
			testCase.assertion(t, address, err)
		})
	}
}
