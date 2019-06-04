package test

import (
	"fmt"
	"net/http"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tsuru/nginx-operator/pkg/apis/nginx/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestMain(m *testing.M) {
	runningIntegration := os.Getenv("RPAAS_OPERATOR_INTEGRATION") != ""
	if !runningIntegration {
		fmt.Println("Skipping the integration tests since RPAAS_OPERATOR_INTEGRATION env var isn't defined")
		os.Exit(0)
	}

	os.Exit(m.Run())
}

func Test_RpaasOperator(t *testing.T) {
	t.Run("apply manifests at rpaas-full.yaml", func(t *testing.T) {
		namespaceName := "rpaasoperator-full"

		cleanNsFunc, err := createNamespace(namespaceName)
		require.NoError(t, err)
		defer cleanNsFunc()

		err = apply("./testdata/rpaas-full.yaml", namespaceName)
		assert.NoError(t, err)

		nginx, err := getReadyNginx("my-instance", namespaceName, 2, 1)
		require.NoError(t, err)
		assert.Equal(t, int32(2), *nginx.Spec.Replicas)
		assert.Equal(t, "tsuru/nginx-tsuru:1.15.0", nginx.Spec.Image)
		assert.Equal(t, "/_nginx_healthcheck/", nginx.Spec.HealthcheckPath)

		nginxConf := &corev1.ConfigMap{
			TypeMeta: metav1.TypeMeta{
				APIVersion: "v1",
				Kind:       "ConfigMap",
			},
		}

		err = get(nginxConf, nginx.Spec.Config.Name, namespaceName)
		assert.NoError(t, err)
		assert.Contains(t, "# My custom root block", nginxConf.Data["nginx-conf"])
		assert.Contains(t, "# My custom HTTP block", nginxConf.Data["nginx-conf"])
		assert.Contains(t, "# My custom server block", nginxConf.Data["nginx-conf"])

		tlsSecret := &v1alpha1.TLSSecret{
			SecretName: "my-instance-certificates",
			Items: []v1alpha1.TLSSecretItem{
				{
					CertificateField: "default.crt",
					CertificatePath:  "my-custom-name.crt",
					KeyField:         "default.key",
					KeyPath:          "my-custom-name.key",
				},
			},
		}
		assert.Equal(t, tlsSecret, nginx.Spec.Certificates)

		nginxService := &corev1.Service{
			TypeMeta: metav1.TypeMeta{
				APIVersion: "v1",
				Kind:       "Service",
			},
		}
		err = get(nginxService, fmt.Sprintf("%s-service", nginx.Name), namespaceName)
		assert.NoError(t, err)
		assert.Equal(t, int32(80), nginxService.Spec.Ports[0].Port)
		assert.Equal(t, int32(443), nginxService.Spec.Ports[1].Port)
		assert.Equal(t, corev1.ServiceType("LoadBalancer"), nginxService.Spec.Type)
		assert.Equal(t, "127.0.0.1", nginxService.Spec.LoadBalancerIP)
		assert.Equal(t, "nginx", nginxService.Spec.Selector["app"])
		assert.Equal(t, "custom-annotation-value", nginxService.Annotations["rpaas.extensions.tsuru.io/custom-annotation"])
		assert.Equal(t, "custom-label-value", nginxService.Labels["custom-label"])
	})
}

func Test_RpaasApi(t *testing.T) {
	apiAddress := os.Getenv("RPAAS_API_ADDRESS")

	if apiAddress == "" {
		t.Skip("Skipping RPaaS API integration test due the RPAAS_API_ADDRESS env var isn't defined")
	}

	api := &rpaasApi{
		address: apiAddress,
		client:  &http.Client{Timeout: 10 * time.Second},
	}

	ok, err := api.health()
	require.NoError(t, err)
	assert.True(t, ok)

	err = apply("./testdata/rpaasplan-basic.yaml", "no-namespaced")
	require.NoError(t, err)
	defer func() {
		err = delete("./testdata/rpaasplan-basic.yaml", "no-namespaced")
		require.NoError(t, err)
	}()

	t.Run("Creating and deleting a instance", func(t *testing.T) {
		instanceName := "my-instance"
		teamName := "team-one"
		planName := "basic"

		namespaceName := fmt.Sprintf("rpaasv2-%s", teamName)

		cleanFunc, err := api.createInstance(instanceName, planName, teamName)
		require.NoError(t, err)
		defer func() {
			err = cleanFunc()
			assert.NoError(t, err)

			var isNginxRemoved bool
			for retries := 10; retries > 0; retries-- {
				_, err = getReadyNginx(instanceName, namespaceName, 1, 1)
				if err != nil {
					isNginxRemoved = true
					break
				}
				time.Sleep(time.Second)
			}
			assert.True(t, isNginxRemoved)

			err = deleteNamespace(namespaceName)
			assert.NoError(t, err)
		}()

		namespace := corev1.Namespace{
			TypeMeta: metav1.TypeMeta{
				APIVersion: "v1",
				Kind:       "Namespace",
			},
		}
		err = get(&namespace, namespaceName, "no-namespaced")
		assert.NoError(t, err)

		nginx, err := getReadyNginx(instanceName, namespace.Name, 1, 1)
		assert.NoError(t, err)
		assert.Equal(t, int32(1), *nginx.Spec.Replicas)
		assert.Equal(t, "tsuru/nginx-tsuru:1.15.0", nginx.Spec.Image)
		assert.Equal(t, "/_nginx_healthcheck/", nginx.Spec.HealthcheckPath)

		nginxService := &corev1.Service{
			TypeMeta: metav1.TypeMeta{
				APIVersion: "v1",
				Kind:       "Service",
			},
		}
		err = get(nginxService, fmt.Sprintf("%s-service", nginx.Name), namespace.Name)
		assert.NoError(t, err)
		assert.Equal(t, int32(80), nginxService.Spec.Ports[0].Port)
		assert.Equal(t, corev1.ServiceType("LoadBalancer"), nginxService.Spec.Type)
	})
}

func getReadyNginx(name, namespace string, expectedPods, expectedSvcs int) (*v1alpha1.Nginx, error) {
	nginx := &v1alpha1.Nginx{TypeMeta: metav1.TypeMeta{Kind: "Nginx"}}
	timeout := time.After(60 * time.Second)
	for {
		err := get(nginx, name, namespace)
		if err == nil && len(nginx.Status.Pods) == expectedPods && len(nginx.Status.Services) == expectedSvcs {
			return nginx, nil
		}
		select {
		case <-timeout:
			return nil, fmt.Errorf("Timeout waiting for nginx status. Last nginx object: %#v. Last error: %v", nginx, err)
		case <-time.After(time.Millisecond * 100):
		}
	}
}
