package proxy

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"gotest.tools/assert"
)

type mockServer struct {
	ts            *httptest.Server
	getURLfunc    func(string) (string, error)
	getTargetFunc func() (string, error)
	readTokenFunc func() (string, error)
}

func (ms *mockServer) GetURL(path string) (string, error) {
	if ms.getURLfunc == nil {
		return ms.ts.URL, nil
	}
	return ms.getURLfunc(path)
}

func (ms *mockServer) GetTarget() (string, error) {
	if ms.getTargetFunc == nil {
		return "", nil
	}
	return ms.getTargetFunc()
}

func (ms *mockServer) ReadToken() (string, error) {
	if ms.readTokenFunc == nil {
		return "", nil
	}
	return ms.readTokenFunc()
}

func TestProxy(t *testing.T) {
	testCases := []struct {
		name       string
		prox       Proxy
		handler    http.HandlerFunc
		assertion  func(t *testing.T, err error)
		testServer *httptest.Server
	}{
		{
			name: "testing invalid target",
			prox: Proxy{ServiceName: "",
				InstanceName: "",
				Method:       "GET",
				Server: &mockServer{getTargetFunc: func() (string, error) {
					return "", errors.New("Error while aquiring target")
				}},
			},
			assertion: func(t *testing.T, err error) {
				assert.Error(t, err, err.Error())
				assert.Equal(t, err.Error(), "Error while aquiring target")
			},
			handler: func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusOK)
			},
		},
		{
			name: "testing invalid URL",
			prox: Proxy{
				ServiceName:  "rpaas-service-test",
				InstanceName: "rpaas-instance-test",
				Method:       "GET",
				Server: &mockServer{getTargetFunc: func() (string, error) {
					return "", errors.New("Error while parsing URL")
				}},
			},
			assertion: func(t *testing.T, err error) {
				assert.Error(t, err, err.Error())
				assert.Equal(t, err.Error(), "Error while parsing URL")
			},
			handler: func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				helper := []struct {
					instanceName string `json:"name"`
					serviceName  string `json:"service"`
				}{
					{
						instanceName: "rpaas-instance-test",
						serviceName:  "rpaas-service-test",
					},
				}
				body, _ := json.Marshal(helper)
				w.Write(body)
				w.WriteHeader(http.StatusOK)
			},
		},
		{
			name: "testing invalid token",
			prox: Proxy{
				ServiceName:  "rpaas-service-test",
				InstanceName: "rpaas-instance-test",
				Method:       "GET",
				Server: &mockServer{getTargetFunc: func() (string, error) {
					return "", errors.New("Error while parsing token")
				}},
			},
			assertion: func(t *testing.T, err error) {
				assert.Error(t, err, err.Error())
				assert.Equal(t, err.Error(), "Error while parsing token")
			},
			handler: func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				helper := []struct {
					instanceName string `json:"name"`
					serviceName  string `json:"service"`
				}{
					{
						instanceName: "rpaas-instance-test",
						serviceName:  "rpaas-service-test",
					},
				}
				body, _ := json.Marshal(helper)
				w.Write(body)
				w.WriteHeader(http.StatusOK)
			},
		},
	}
	for _, tt := range testCases {
		t.Run(tt.name, func(t *testing.T) {
			tt.testServer = httptest.NewServer(tt.handler)
			_, err := tt.prox.ProxyRequest()
			tt.assertion(t, err)
		})
	}
}
