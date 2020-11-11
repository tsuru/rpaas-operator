// Copyright 2019 tsuru authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package test

import (
	"context"
	"fmt"
	"io/ioutil"
	"math/rand"
	"net/http"
	"os"
	"os/exec"
	"strconv"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	nginxv1alpha1 "github.com/tsuru/nginx-operator/api/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/tsuru/rpaas-operator/api/v1alpha1"
	"github.com/tsuru/rpaas-operator/internal/pkg/rpaas"
)

func TestMain(m *testing.M) {
	runningIntegration := os.Getenv("RPAAS_OPERATOR_INTEGRATION") != ""
	if !runningIntegration {
		fmt.Println("Skipping the integration tests since RPAAS_OPERATOR_INTEGRATION env var isn't defined")
		os.Exit(0)
	}
	rand.Seed(time.Now().Unix())

	os.Exit(m.Run())
}

func assertInstanceContains(t *testing.T, localPort int, expectedStatus int, bodyPart string) {
	rsp, iErr := http.Get(fmt.Sprintf("http://127.0.0.1:%d/", localPort))
	require.NoError(t, iErr)
	assert.Equal(t, expectedStatus, rsp.StatusCode)
	defer rsp.Body.Close()
	rawBody, iErr := ioutil.ReadAll(rsp.Body)
	require.NoError(t, iErr)
	assert.Contains(t, string(rawBody), bodyPart)
}

func Test_RpaasOperator(t *testing.T) {
	t.Run("apply manifests at rpaas-full.yaml", func(t *testing.T) {
		namespaceName := "rpaasoperator-full" + strconv.Itoa(rand.Int())

		cleanNsFunc, err := createNamespace(namespaceName)
		require.NoError(t, err)
		defer cleanNsFunc()

		err = apply("./testdata/rpaas-full.yaml", namespaceName)
		assert.NoError(t, err)

		nginx, err := getReadyNginx("my-instance", namespaceName, 2, 1)
		require.NoError(t, err)
		assert.Equal(t, int32(2), *nginx.Spec.Replicas)
		assert.Equal(t, "tsuru/nginx-tsuru:1.16.1", nginx.Spec.Image)
		assert.Equal(t, "/_nginx_healthcheck", nginx.Spec.HealthcheckPath)
		assert.Len(t, nginx.Status.Pods, 2)
		for _, podStatus := range nginx.Status.Pods {
			pod := &corev1.Pod{
				TypeMeta: metav1.TypeMeta{
					APIVersion: "v1",
					Kind:       "Pod",
				},
			}
			err = get(pod, podStatus.Name, namespaceName)
			require.NoError(t, err)
			assert.Equal(t, "label-value", pod.Labels["pod-custom-label"])
		}

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

		tlsSecret := &nginxv1alpha1.TLSSecret{
			SecretName: "my-instance-certificates",
			Items: []nginxv1alpha1.TLSSecretItem{
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
		assert.Equal(t, "nginx", nginxService.Spec.Selector["nginx.tsuru.io/app"])
		assert.Equal(t, "custom-annotation-value", nginxService.Annotations["rpaas.extensions.tsuru.io/custom-annotation"])
		assert.Equal(t, "custom-label-value", nginxService.Labels["custom-label"])
	})

	t.Run("use plan to set resource limits on nginx container", func(t *testing.T) {
		namespaceName := "rpaasoperator-full" + strconv.Itoa(rand.Int())

		cleanNsFunc, err := createNamespace(namespaceName)
		require.NoError(t, err)
		defer cleanNsFunc()

		err = apply("./testdata/rpaas-full.yaml", namespaceName)
		assert.NoError(t, err)

		nginx, err := getReadyNginx("my-instance", namespaceName, 2, 1)
		require.NoError(t, err)

		expectedLimits := corev1.ResourceRequirements{
			Limits: corev1.ResourceList{
				corev1.ResourceMemory: resource.MustParse("128Mi"),
			},
		}

		assert.Equal(t, expectedLimits, nginx.Spec.Resources)
	})
}

func Test_RpaasApi(t *testing.T) {
	apiAddress := os.Getenv("RPAAS_API_ADDRESS")
	rpaasv2Bin := os.Getenv("RPAAS_PLUGIN_BIN")

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

	namespaceName := "rpaasv2"

	cleanNsFunc, err := createNamespace(namespaceName)
	require.NoError(t, err)
	defer cleanNsFunc()

	err = apply("./testdata/rpaasplan-basic.yaml", namespaceName)
	require.NoError(t, err)
	defer func() {
		err = delete("./testdata/rpaasplan-basic.yaml", namespaceName)
		require.NoError(t, err)
	}()

	t.Run("creating and deleting an instance", func(t *testing.T) {
		instanceName := generateRandomName("my-instance")
		teamName := generateRandomName("team-one")
		planName := "basic"

		cleanFunc, err := api.createInstance(instanceName, planName, teamName)
		require.NoError(t, err)
		defer func() {
			err = cleanFunc()
			assert.NoError(t, err)
		}()

		nginx, err := getReadyNginx(instanceName, namespaceName, 1, 1)
		require.NoError(t, err)
		require.NotNil(t, nginx)
		assert.Equal(t, int32(1), *nginx.Spec.Replicas)
		assert.Equal(t, "tsuru/nginx-tsuru:1.16.1", nginx.Spec.Image)
		assert.Equal(t, "/_nginx_healthcheck", nginx.Spec.HealthcheckPath)

		nginxService := &corev1.Service{
			TypeMeta: metav1.TypeMeta{
				APIVersion: "v1",
				Kind:       "Service",
			},
		}
		err = get(nginxService, fmt.Sprintf("%s-service", nginx.Name), namespaceName)
		assert.NoError(t, err)
		assert.Equal(t, int32(80), nginxService.Spec.Ports[0].Port)
		assert.Equal(t, corev1.ServiceType("LoadBalancer"), nginxService.Spec.Type)
	})

	t.Run("bind and unbind with a local application", func(t *testing.T) {
		instanceName := generateRandomName("my-instance")
		teamName := generateRandomName("team-one")
		planName := "basic"

		cleanFunc, err := api.createInstance(instanceName, planName, teamName)
		require.NoError(t, err)
		defer cleanFunc()

		_, err = getReadyNginx(instanceName, namespaceName, 1, 1)
		require.NoError(t, err)

		err = apply("testdata/hello-app.yaml", namespaceName)
		require.NoError(t, err)
		defer func() {
			err = delete("testdata/hello-app.yaml", namespaceName)
			require.NoError(t, err)
		}()

		_, err = kubectl("wait", "--for=condition=Ready", "-l", "app=hello", "pod", "--timeout", "5m", "-n", namespaceName)
		require.NoError(t, err)

		serviceName := fmt.Sprintf("svc/%s-service", instanceName)
		servicePort := "80"

		ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
		defer cancel()
		err = portForward(ctx, namespaceName, serviceName, servicePort, func(localPort int) {
			assertInstanceContains(t, localPort, http.StatusNotFound, "instance not bound")
		})
		require.NoError(t, err)

		helloServiceHost := fmt.Sprintf("hello.%s.svc", namespaceName)
		err = api.bind("hello-app", instanceName, helloServiceHost)
		require.NoError(t, err)

		time.Sleep(10 * time.Second)

		_, err = getReadyNginx(instanceName, namespaceName, 1, 1)
		require.NoError(t, err)

		ctx, cancel = context.WithTimeout(context.Background(), time.Minute)
		defer cancel()
		err = portForward(ctx, namespaceName, serviceName, servicePort, func(localPort int) {
			assertInstanceContains(t, localPort, http.StatusOK, "CLIENT VALUES")
		})
		require.NoError(t, err)

		err = api.unbind("hello-app", instanceName, helloServiceHost)
		require.NoError(t, err)

		time.Sleep(10 * time.Second)

		_, err = getReadyNginx(instanceName, namespaceName, 1, 1)
		require.NoError(t, err)

		ctx, cancel = context.WithTimeout(context.Background(), time.Minute)
		defer cancel()
		err = portForward(ctx, namespaceName, serviceName, servicePort, func(localPort int) {
			assertInstanceContains(t, localPort, http.StatusNotFound, "instance not bound")
		})
		require.NoError(t, err)
	})

	t.Run("multiple binds with a local application", func(t *testing.T) {
		instanceName := generateRandomName("my-instance")
		teamName := generateRandomName("team-one")
		planName := "basic"

		cleanFunc, err := api.createInstance(instanceName, planName, teamName)
		require.NoError(t, err)
		defer cleanFunc()

		_, err = getReadyNginx(instanceName, namespaceName, 1, 1)
		require.NoError(t, err)

		err = apply("testdata/hello-app.yaml", namespaceName)
		require.NoError(t, err)
		defer func() {
			err = delete("testdata/hello-app.yaml", namespaceName)
			require.NoError(t, err)
		}()
		err = apply("testdata/echo-server.yaml", namespaceName)
		require.NoError(t, err)
		defer func() {
			err = delete("testdata/echo-server.yaml", namespaceName)
			require.NoError(t, err)
		}()

		_, err = kubectl("wait", "--for=condition=Ready", "-l", "app=hello", "pod", "--timeout", "5m", "-n", namespaceName)
		require.NoError(t, err)
		_, err = kubectl("wait", "--for=condition=Ready", "-l", "app=echo-server", "pod", "--timeout", "5m", "-n", namespaceName)
		require.NoError(t, err)

		serviceName := fmt.Sprintf("svc/%s-service", instanceName)
		servicePort := "80"

		ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
		defer cancel()
		err = portForward(ctx, namespaceName, serviceName, servicePort, func(localPort int) {
			assertInstanceContains(t, localPort, http.StatusNotFound, "instance not bound")
		})
		require.NoError(t, err)

		helloServiceHost := fmt.Sprintf("hello.%s.svc", namespaceName)
		err = api.bind("hello-app", instanceName, helloServiceHost)
		require.NoError(t, err)
		echoServerServiceHost := fmt.Sprintf("echo-server.%s.svc", namespaceName)
		err = api.bind("echo-server", instanceName, echoServerServiceHost)
		require.NoError(t, err)

		time.Sleep(10 * time.Second)

		_, err = getReadyNginx(instanceName, namespaceName, 1, 1)
		require.NoError(t, err)

		ctx, cancel = context.WithTimeout(context.Background(), time.Minute)
		defer cancel()
		err = portForward(ctx, namespaceName, serviceName, servicePort, func(localPort int) {
			assertInstanceContains(t, localPort, http.StatusOK, "CLIENT VALUES")
		})
		require.NoError(t, err)

		err = api.unbind("hello-app", instanceName, helloServiceHost)
		require.NoError(t, err)

		time.Sleep(10 * time.Second)

		_, err = getReadyNginx(instanceName, namespaceName, 1, 1)
		require.NoError(t, err)

		ctx, cancel = context.WithTimeout(context.Background(), time.Minute)
		defer cancel()
		err = portForward(ctx, namespaceName, serviceName, servicePort, func(localPort int) {
			assertInstanceContains(t, localPort, http.StatusOK, "")
		})
		require.NoError(t, err)

		err = api.unbind("echo-server", instanceName, echoServerServiceHost)
		require.NoError(t, err)

		time.Sleep(10 * time.Second)

		_, err = getReadyNginx(instanceName, namespaceName, 1, 1)
		require.NoError(t, err)

		ctx, cancel = context.WithTimeout(context.Background(), time.Minute)
		defer cancel()
		err = portForward(ctx, namespaceName, serviceName, servicePort, func(localPort int) {
			assertInstanceContains(t, localPort, http.StatusNotFound, "instance not bound")
		})
		require.NoError(t, err)

	})

	t.Run("adding and deleting routes", func(t *testing.T) {
		instanceName := generateRandomName("my-instance")
		teamName := generateRandomName("team-one")
		planName := "basic"

		cleanFunc, err := api.createInstance(instanceName, planName, teamName)
		require.NoError(t, err)
		defer cleanFunc()

		_, err = getReadyNginx(instanceName, namespaceName, 1, 1)
		require.NoError(t, err)

		err = api.updateRoute(instanceName, rpaas.Route{
			Path:    "/path1",
			Content: `echo "My custom content at /path1";`,
		})
		require.NoError(t, err)
		time.Sleep(time.Second)

		err = api.updateRoute(instanceName, rpaas.Route{
			Path:        "/",
			Destination: "localhost:8080",
		})
		require.NoError(t, err)
		time.Sleep(time.Second)

		err = api.updateRoute(instanceName, rpaas.Route{
			Path:        "/secure",
			Destination: "localhost:8080",
			HTTPSOnly:   true,
		})
		require.NoError(t, err)

		_, err = getReadyNginx(instanceName, namespaceName, 1, 1)
		require.NoError(t, err)

		rpaasInstance, err := getRpaasInstance(instanceName, namespaceName)
		require.NoError(t, err)

		assert.Equal(t, []v1alpha1.Location{
			{
				Path: "/path1",
				Content: &v1alpha1.Value{
					Value: `echo "My custom content at /path1";`,
				},
			},
			{
				Path:        "/",
				Destination: "localhost:8080",
			},
			{
				Path:        "/secure",
				Destination: "localhost:8080",
				ForceHTTPS:  true,
			},
		}, rpaasInstance.Spec.Locations)

		err = api.deleteRoute(instanceName, "/secure")
		require.NoError(t, err)

		err = api.updateRoute(instanceName, rpaas.Route{
			Path:    "/",
			Content: `echo "My root path response";`,
		})
		require.NoError(t, err)

		_, err = getReadyNginx(instanceName, namespaceName, 1, 1)
		require.NoError(t, err)

		rpaasInstance, err = getRpaasInstance(instanceName, namespaceName)
		require.NoError(t, err)

		assert.Equal(t, []v1alpha1.Location{
			{
				Path: "/path1",
				Content: &v1alpha1.Value{
					Value: `echo "My custom content at /path1";`,
				},
			},
			{
				Path: "/",
				Content: &v1alpha1.Value{
					Value: `echo "My root path response";`,
				},
			},
		}, rpaasInstance.Spec.Locations)
	})

	t.Run("limits the number of configs to 10 by default", func(t *testing.T) {
		instanceName := generateRandomName("my-instance")
		teamName := generateRandomName("team-one")
		planName := "basic"
		blockName := "server"

		cleanFunc, err := api.createInstance(instanceName, planName, teamName)
		require.NoError(t, err)
		defer cleanFunc()

		configList, err := getConfigList(instanceName, namespaceName)
		require.NoError(t, err)
		assert.Equal(t, len(configList.Items), 1)

		for i := 0; i < 15; i++ {
			_, err = api.createBlock(instanceName, blockName, fmt.Sprintf("location = /test%d { return 204; }", i))
			require.NoError(t, err)
			time.Sleep(500 * time.Millisecond)
		}

		timeout := time.After(60 * time.Second)
		expectedConfigSize := 10
		expectedStableCount := 10
		count := 0
		for {
			select {
			case <-timeout:
				t.Fatalf("timeout waiting for configs to reach %v, last count: %v", expectedConfigSize, len(configList.Items))
			case <-time.After(100 * time.Millisecond):
			}
			configList, err = getConfigList(instanceName, namespaceName)
			require.NoError(t, err)
			if len(configList.Items) == expectedConfigSize {
				count++
				if count == expectedStableCount {
					break
				}
				continue
			}
			count = 0
		}
		configList, err = getConfigList(instanceName, namespaceName)
		require.NoError(t, err)
		assert.Equal(t, expectedConfigSize, len(configList.Items))
	})

	t.Run("exec an remote command in instance", func(t *testing.T) {
		instanceName := generateRandomName("my-instance")
		teamName := generateRandomName("team-one")
		planName := "basic"

		cleanFunc, err := api.createInstance(instanceName, planName, teamName)
		require.NoError(t, err)
		defer cleanFunc()

		_, err = getReadyNginx(instanceName, namespaceName, 1, 1)
		require.NoError(t, err)

		cmd := exec.CommandContext(context.Background(), rpaasv2Bin, "--rpaas-url", apiAddress, "exec", "-i", instanceName, "--", "echo", "WORKING")
		out, err := cmd.CombinedOutput()
		require.NoError(t, err, fmt.Sprintf("exec was not successful. Returned output: %s", string(out)))
		assert.Contains(t, string(out), "WORKING\n")
	})
}

func getRpaasInstance(name, namespace string) (*v1alpha1.RpaasInstance, error) {
	instance := &v1alpha1.RpaasInstance{
		TypeMeta: metav1.TypeMeta{
			Kind: "RpaasInstance",
		},
	}
	err := get(instance, name, namespace)
	return instance, err
}

func getReadyNginx(name, namespace string, expectedPods, expectedSvcs int) (*nginxv1alpha1.Nginx, error) {
	nginx := &nginxv1alpha1.Nginx{TypeMeta: metav1.TypeMeta{Kind: "Nginx"}}
	timeout := time.After(120 * time.Second)
	var err error
	for {
		select {
		case <-timeout:
			return nil, fmt.Errorf("Timeout waiting for nginx status. Last nginx object: %#v. Last error: %v", nginx, err)
		case <-time.After(time.Millisecond * 100):
		}

		_, err = kubectl("rollout", "status", "-n", namespace, "deploy", name, "--timeout=5s", "--watch")
		if err != nil {
			continue
		}

		err = get(nginx, name, namespace)
		if err != nil || len(nginx.Status.Pods) != expectedPods || len(nginx.Status.Services) != expectedSvcs {
			continue
		}

		if len(nginx.Status.Pods) == 0 {
			return nginx, nil
		}

		waitArgs := []string{"wait", "--for=condition=Ready", "-n", namespace, "--timeout=1s"}

		for _, pod := range nginx.Status.Pods {
			waitArgs = append(waitArgs, fmt.Sprintf("pod/%s", pod.Name))
		}

		if _, err = kubectl(waitArgs...); err == nil {
			return nginx, nil
		}
	}
}

func generateRandomName(prefix string) string {
	n := rand.Int() / 100000
	return fmt.Sprintf("%s-%d", prefix, n)
}
