package controllerapi_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tsuru/nginx-operator/api/v1alpha1"
	"github.com/tsuru/rpaas-operator/pkg/controllerapi"
	extensionsruntime "github.com/tsuru/rpaas-operator/pkg/runtime"
	coreV1 "k8s.io/api/core/v1"
	metaV1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func TestPrometheusDiscover(t *testing.T) {

	nginx1 := &v1alpha1.Nginx{
		ObjectMeta: metaV1.ObjectMeta{
			Name:      "test",
			Namespace: "default",
		},
		Spec: v1alpha1.NginxSpec{
			TLS: []v1alpha1.NginxTLS{
				{
					Hosts: []string{"test.internal"},
				},
				{
					Hosts: []string{"hello.globo"},
				},
			},
		},
		Status: v1alpha1.NginxStatus{
			Services: []v1alpha1.ServiceStatus{
				{
					Name: "test",
				},
			},
		},
	}

	svc1 := &coreV1.Service{
		ObjectMeta: metaV1.ObjectMeta{
			Name:      "test",
			Namespace: "default",
			Labels: map[string]string{
				"nginx.tsuru.io/app":                      "nginx",
				"rpaas.extensions.tsuru.io/instance-name": "test",
				"rpaas.extensions.tsuru.io/service-name":  "rpaasv2",
				"rpaas.extensions.tsuru.io/team-owner":    "team01",
			},
		},
		Status: coreV1.ServiceStatus{
			LoadBalancer: coreV1.LoadBalancerStatus{
				Ingress: []coreV1.LoadBalancerIngress{
					{
						IP: "1.1.1.1",
					},
				},
			},
		},
	}

	scheme := extensionsruntime.NewScheme()
	client := fake.NewClientBuilder().WithScheme(scheme).WithRuntimeObjects(nginx1, svc1).Build()
	api := controllerapi.New(client)

	w := httptest.NewRecorder()
	r, _ := http.NewRequest(http.MethodGet, "/v1/prometheus/discover", nil)

	api.ServeHTTP(w, r)

	assert.Equal(t, http.StatusOK, w.Code)
	l := []controllerapi.TargetGroup{}
	err := json.Unmarshal(w.Body.Bytes(), &l)
	require.NoError(t, err)

	assert.Equal(t, []controllerapi.TargetGroup{
		{
			Targets: []string{"http://1.1.1.1/_nginx_healthcheck"},
			Labels: map[string]string{
				"service":          "rpaasv2",
				"service_instance": "test",
				"team_owner":       "team01",
			},
		},
		{
			Targets: []string{"https://1.1.1.1/_nginx_healthcheck"},
			Labels: map[string]string{
				"servername":       "test.internal",
				"service":          "rpaasv2",
				"service_instance": "test",
				"team_owner":       "team01",
			},
		},
		{
			Targets: []string{"https://1.1.1.1/_nginx_healthcheck"},
			Labels: map[string]string{
				"servername":       "hello.globo",
				"service":          "rpaasv2",
				"service_instance": "test",
				"team_owner":       "team01",
			},
		},
	}, l)
}
