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
	ts *httptest.Server
}

func (ms *mockServer) GetURL(path string) (string, error) {
	return ms.ts.URL, nil
}

func (ms *mockServer) GetTarget() (string, error) {
	return "", nil
}

func (ms *mockServer) ReadToken() (string, error) {
	return "", nil
}

func TestGetInfo(t *testing.T) {
	testCases := []struct {
		name         string
		serviceName  string
		instanceName string
		body         string
		expectedCode int
		handler      http.HandlerFunc
		assertion    func(t *testing.T, err error)
	}{
		{
			name:         "when no flags are passed",
			serviceName:  "",
			instanceName: "",
			expectedCode: http.StatusNotFound,
			assertion: func(t *testing.T, err error) {
				assert.Error(t, err, err.Error())
			},
			handler: func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusNotFound)
			},
		},
		{
			name:         "when valid flags are passed",
			serviceName:  "rpaas-service-test",
			instanceName: "rpaas-instance-test",
			expectedCode: http.StatusOK,
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
	}
	for _, tt := range testCases {
		t.Run(tt.name, func(t *testing.T) {
			ts := httptest.NewServer(tt.handler)
			defer ts.Close()
			info := infoArgs{}
			info.service = tt.serviceName
			info.instance = tt.instanceName
			info.prox = &proxy.Proxy{ServiceName: info.service, InstanceName: info.instance, Method: "GET"}
			info.prox.Server = &mockServer{ts: ts}
			err := runInfo(info)
			tt.assertion(t, err)
		})
	}

}
