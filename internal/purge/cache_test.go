package purge

import (
	"fmt"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	apiv1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/watch"
	ktesting "k8s.io/client-go/testing"
)

func bodyContent(rsp *httptest.ResponseRecorder) string {
	data, _ := ioutil.ReadAll(rsp.Body)
	return strings.TrimSuffix(string(data), "\n")
}

type fakeCacheManager struct {
}

func (f fakeCacheManager) PurgeCache(host, path string, port int32, preservePath bool) error {
	return nil
}

func TestCachePurge(t *testing.T) {
	// setup
	watchFake := watch.NewFake()
	k8sClient.Fake.PrependWatchReactor("pods", ktesting.DefaultWatchReactor(watchFake, nil))

	watcher, err := NewWatcher(k8sClient)
	assert.NoError(t, err)
	defer watcher.Stop()
	watcher.Watch()

	managerFake := fakeCacheManager{}
	api, err := NewAPI(watcher, managerFake)
	assert.NoError(t, err)

	// adds pods to watcher to ensure correct behaviour for success test cases
	basePod := &apiv1.Pod{
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
					},
				},
			},
		},
		Status: apiv1.PodStatus{
			PodIP:             "172.0.2.1",
			ContainerStatuses: []apiv1.ContainerStatus{{Ready: true}},
		},
	}
	watchFake.Add(basePod.DeepCopy())
	// Let fake watch propagate the event
	time.Sleep(100 * time.Millisecond)

	tests := []struct {
		name           string
		instance       string
		requestBody    string
		expectedStatus int
		expectedBody   string
	}{
		{
			name:           "success",
			instance:       "sample-rpaasv2",
			requestBody:    `{"path":"/index.html","preserve_path":true}`,
			expectedStatus: http.StatusOK,
			expectedBody:   "Object purged on 1 servers",
		},
		{
			name:           "returns bad request if body is empty",
			instance:       "some-instance",
			requestBody:    "",
			expectedStatus: http.StatusBadRequest,
			expectedBody:   `{"message":"Request body can't be empty"}`,
		},
		{
			name:           "returns bad request if instance is not sent",
			instance:       "",
			requestBody:    `{"path":"/index.html","preserve_path":true}`,
			expectedStatus: http.StatusBadRequest,
			expectedBody:   "instance is required",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {

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
	// setup
	watchFake := watch.NewFake()
	k8sClient.Fake.PrependWatchReactor("pods", ktesting.DefaultWatchReactor(watchFake, nil))

	watcher, err := NewWatcher(k8sClient)
	assert.NoError(t, err)
	defer watcher.Stop()
	watcher.Watch()

	managerFake := fakeCacheManager{}
	api, err := NewAPI(watcher, managerFake)
	assert.NoError(t, err)

	// adds pods to watcher to ensure correct behaviour for success test cases
	basePod := &apiv1.Pod{
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
					},
				},
			},
		},
		Status: apiv1.PodStatus{
			PodIP:             "172.0.2.1",
			ContainerStatuses: []apiv1.ContainerStatus{{Ready: true}},
		},
	}
	watchFake.Add(basePod.DeepCopy())
	// Let fake watch propagate the event
	time.Sleep(100 * time.Millisecond)

	tests := []struct {
		name           string
		instance       string
		requestBody    string
		expectedStatus int
		expectedBody   string
	}{
		{
			name:           "success",
			instance:       "sample-rpaasv2",
			requestBody:    `[{"path":"/index.html","preserve_path":true}]`,
			expectedStatus: http.StatusOK,
			expectedBody:   `[{"path":"/index.html","instances_purged":1}]`,
		},
		{
			name:           "returns bad request if instance is not sent",
			instance:       "",
			requestBody:    `[{"path":"/index.html","preserve_path":true}]`,
			expectedStatus: http.StatusBadRequest,
			expectedBody:   "instance is required",
		},
		{
			name:           "returns bad request if arguments are not a list",
			instance:       "",
			requestBody:    `{"path":"/index.html","preserve_path":true}`,
			expectedStatus: http.StatusBadRequest,
			expectedBody:   "instance is required",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {

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
