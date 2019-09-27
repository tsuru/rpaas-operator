package cmd

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/tsuru/rpaas-operator/build/cli/proxy"
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

func TestGetInfo(t *testing.T) {
	testCases := []struct {
		name      string
		info      infoArgs
		handler   http.HandlerFunc
		assertion func(t *testing.T, err error)
	}{
		{
			name: "when invalid flags are passed",
			info: infoArgs{service: "", instance: "", prox: &proxy.Proxy{ServiceName: "", InstanceName: "", Method: "GET"}},
			assertion: func(t *testing.T, err error) {
				assert.Error(t, err, "404 Not Found")
			},
			handler: func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusNotFound)
			},
		},
		{
			name: "when valid flags are passed",
			info: infoArgs{service: "rpaas-service-test", instance: "rpaas-instance-test",
				prox: &proxy.Proxy{ServiceName: "rpaas-service-test", InstanceName: "rpaas-instance-test", Method: "GET"}},

			assertion: func(t *testing.T, err error) {
				assert.NilError(t, err)
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
			name: "when invalid body is returned",
			info: infoArgs{service: "rpaas-service-test", instance: "rpaas-instance-test",
				prox: &proxy.Proxy{ServiceName: "rpaas-service-test", InstanceName: "rpaas-instance-test", Method: "GET"}},

			assertion: func(t *testing.T, err error) {
				assert.Error(t, err, "unexpected end of JSON input")
			},
			handler: func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				w.Write(nil)
				w.WriteHeader(0)
			},
		},
	}
	for _, tt := range testCases {
		t.Run(tt.name, func(t *testing.T) {
			ts := httptest.NewServer(tt.handler)
			tt.info.prox.Server = &mockServer{ts: ts}
			defer ts.Close()
			err := runInfo(tt.info)
			tt.assertion(t, err)
		})
	}
}
