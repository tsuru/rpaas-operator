package purge

import (
	"fmt"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	apiv1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	nginxManager "github.com/tsuru/rpaas-operator/internal/pkg/rpaas/nginx"
	extensionsruntime "github.com/tsuru/rpaas-operator/pkg/runtime"
)

func bodyContent(rsp *httptest.ResponseRecorder) string {
	data, _ := ioutil.ReadAll(rsp.Body)
	return strings.TrimSuffix(string(data), "\n")
}

type fakeCacheManager struct {
	purgeCacheFunc func(host, path string, port int32, preservePath bool) (bool, error)
}

func (f fakeCacheManager) PurgeCache(host, path string, port int32, preservePath bool) (bool, error) {
	if f.purgeCacheFunc != nil {
		return f.purgeCacheFunc(host, path, port, preservePath)
	}
	return false, nil
}

func TestCachePurge(t *testing.T) {
	tests := []struct {
		name           string
		instance       string
		requestBody    string
		expectedStatus int
		expectedBody   string
		cacheManager   fakeCacheManager
	}{
		{
			name:           "success",
			instance:       "sample-rpaasv2",
			requestBody:    `{"path":"/index.html","preserve_path":true}`,
			expectedStatus: http.StatusOK,
			expectedBody:   `{"path":"/index.html","instances_purged":2}`,
			cacheManager: fakeCacheManager{
				purgeCacheFunc: func(host, path string, port int32, preservePath bool) (bool, error) {
					return true, nil
				},
			},
		},
		{
			name:           "no cache key found",
			instance:       "sample-rpaasv2",
			requestBody:    `{"path":"/index.html","preserve_path":true}`,
			expectedStatus: http.StatusOK,
			expectedBody:   `{"path":"/index.html"}`,
			cacheManager:   fakeCacheManager{},
		},
		{
			name:           "fails on some servers",
			instance:       "sample-rpaasv2",
			requestBody:    `{"path":"/index.html","preserve_path":true}`,
			expectedStatus: http.StatusOK,
			expectedBody:   `{"path":"/index.html","instances_purged":1,"error":"1 error occurred:\n\t* pod 172.0.2.2:8889 failed: some nginx error\n\n"}`,
			cacheManager: fakeCacheManager{
				purgeCacheFunc: func(host, path string, port int32, preservePath bool) (bool, error) {
					if host == "172.0.2.2" {
						return false, nginxManager.NginxError{Msg: "some nginx error"}
					}
					return true, nil
				},
			},
		},
		{
			name:           "returns bad request if body is empty",
			instance:       "some-instance",
			requestBody:    "",
			expectedStatus: http.StatusBadRequest,
			expectedBody:   `{"message":"Request body can't be empty"}`,
			cacheManager:   fakeCacheManager{},
		},
		{
			name:           "returns bad request if instance is not sent",
			instance:       "",
			requestBody:    `{"path":"/index.html","preserve_path":true}`,
			expectedStatus: http.StatusBadRequest,
			expectedBody:   "instance is required",
			cacheManager:   fakeCacheManager{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			watcher, err := NewWatcher(fake.NewClientBuilder().WithScheme(extensionsruntime.NewScheme()).WithRuntimeObjects(getFakePods()...).Build())
			assert.NoError(t, err)

			api, err := NewAPI(watcher, tt.cacheManager)
			assert.NoError(t, err)

			w := httptest.NewRecorder()
			r, err := http.NewRequest(http.MethodPost, fmt.Sprintf("http://%s/resources/%s/purge", api.Address, tt.instance), strings.NewReader(tt.requestBody))
			assert.NoError(t, err)

			r.Header.Add("Content-Type", "application/json")

			api.e.ServeHTTP(w, r)

			assert.Equal(t, tt.expectedStatus, w.Code)
			assert.Equal(t, tt.expectedBody, bodyContent(w))
		})
	}
}

func TestCachePurgeBulk(t *testing.T) {
	tests := []struct {
		name           string
		instance       string
		requestBody    string
		expectedStatus int
		expectedBody   string
		cacheManager   fakeCacheManager
	}{
		{
			name:           "success",
			instance:       "sample-rpaasv2",
			requestBody:    `[{"path":"/index.html","preserve_path":true}]`,
			expectedStatus: http.StatusOK,
			expectedBody:   `[{"path":"/index.html","instances_purged":2}]`,
			cacheManager: fakeCacheManager{
				purgeCacheFunc: func(host, path string, port int32, preservePath bool) (bool, error) {
					return true, nil
				},
			},
		},
		{
			name:           "fails on some servers",
			instance:       "sample-rpaasv2",
			requestBody:    `[{"path":"/index.html","preserve_path":true},{"path":"/other.html","preserve_path":true}]`,
			expectedStatus: http.StatusInternalServerError,
			expectedBody:   `[{"path":"/index.html","instances_purged":2},{"path":"/other.html","instances_purged":1,"error":"1 error occurred:\n\t* pod 172.0.2.2:8889 failed: some nginx error\n\n"}]`,
			cacheManager: fakeCacheManager{
				purgeCacheFunc: func(host, path string, port int32, preservePath bool) (bool, error) {
					if host == "172.0.2.2" && path == "/other.html" {
						return false, nginxManager.NginxError{Msg: "some nginx error"}
					}
					return true, nil
				},
			},
		},
		{
			name:           "returns bad request if instance is not sent",
			instance:       "",
			requestBody:    `[{"path":"/index.html","preserve_path":true}]`,
			expectedStatus: http.StatusBadRequest,
			expectedBody:   "instance is required",
			cacheManager:   fakeCacheManager{},
		},
		{
			name:           "returns bad request if arguments are not a list",
			instance:       "",
			requestBody:    `{"path":"/index.html","preserve_path":true}`,
			expectedStatus: http.StatusBadRequest,
			expectedBody:   "instance is required",
			cacheManager:   fakeCacheManager{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			watcher, err := NewWatcher(fake.NewClientBuilder().WithScheme(extensionsruntime.NewScheme()).WithRuntimeObjects(getFakePods()...).Build())
			assert.NoError(t, err)

			api, err := NewAPI(watcher, tt.cacheManager)
			assert.NoError(t, err)

			w := httptest.NewRecorder()
			r, err := http.NewRequest(http.MethodPost, fmt.Sprintf("http://%s/resources/%s/purge/bulk", api.Address, tt.instance), strings.NewReader(tt.requestBody))
			assert.NoError(t, err)

			r.Header.Add("Content-Type", "application/json")

			api.e.ServeHTTP(w, r)

			assert.Equal(t, tt.expectedStatus, w.Code)
			assert.Equal(t, tt.expectedBody, bodyContent(w))
		})
	}
}

func getFakePods() []runtime.Object {
	pod1 := &apiv1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name: "pod0-sample-rpaasv2",
			Labels: map[string]string{
				defaultInstanceLabel: "sample-rpaasv2",
			},
			ResourceVersion: "0",
		},
		Spec: apiv1.PodSpec{
			Containers: []apiv1.Container{
				{
					Ports: []apiv1.ContainerPort{
						{Name: "nginx-metrics", ContainerPort: 8889},
						{Name: "http", ContainerPort: 8888},
						{Name: "https", ContainerPort: 8443},
					},
				},
			},
		},
		Status: apiv1.PodStatus{
			PodIP:             "172.0.2.1",
			ContainerStatuses: []apiv1.ContainerStatus{{Ready: true}},
		},
	}

	pod2 := &apiv1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name: "pod1-sample-rpaasv2",
			Labels: map[string]string{
				defaultInstanceLabel: "sample-rpaasv2",
			},
			ResourceVersion: "0",
		},
		Spec: apiv1.PodSpec{
			Containers: []apiv1.Container{
				{
					Ports: []apiv1.ContainerPort{
						{Name: "nginx-metrics", ContainerPort: 8889},
						{Name: "http", ContainerPort: 8888},
						{Name: "https", ContainerPort: 8443},
					},
				},
			},
		},
		Status: apiv1.PodStatus{
			PodIP:             "172.0.2.2",
			ContainerStatuses: []apiv1.ContainerStatus{{Ready: true}},
		},
	}

	return []runtime.Object{pod1, pod2}
}
