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

func TestProxyInvalidTarget(t *testing.T) {
	tt := struct {
		name      string
		prox      Proxy
		handler   http.HandlerFunc
		assertion func(t *testing.T, err error)
	}{
		name: "testing invalid target",
		prox: Proxy{ServiceName: "", InstanceName: "", Method: "GET"},
		assertion: func(t *testing.T, err error) {
			assert.Error(t, err, err.Error())
			assert.Equal(t, err.Error(), "Error while aquiring target")
		},
		handler: func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		},
	}
	t.Run(tt.name, func(t *testing.T) {
		ts := httptest.NewServer(tt.handler)
		tt.prox.Server = &mockServer{ts: ts, getTargetFunc: func() (string, error) {
			return "", errors.New("Error while aquiring target")
		}}
		defer ts.Close()
		_, err := tt.prox.ProxyRequest()
		tt.assertion(t, err)
	})
}

func TestGetInfoInvalidURL(t *testing.T) {
	tt := struct {
		name      string
		prox      Proxy
		handler   http.HandlerFunc
		assertion func(t *testing.T, err error)
	}{
		name: "testing invalid URL",
		prox: Proxy{
			ServiceName:  "rpaas-service-test",
			InstanceName: "rpaas-instance-test",
			Method:       "GET"},
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
	}
	t.Run(tt.name, func(t *testing.T) {
		ts := httptest.NewServer(tt.handler)
		tt.prox.Server = &mockServer{ts: ts, getURLfunc: func(path string) (string, error) {
			return "", errors.New("Error while parsing URL")
		}}
		defer ts.Close()
		_, err := tt.prox.ProxyRequest()
		tt.assertion(t, err)
	})
}

func TestGetInfoInvalidToken(t *testing.T) {
	tt := struct {
		name      string
		prox      Proxy
		handler   http.HandlerFunc
		assertion func(t *testing.T, err error)
	}{
		name: "testing invalid token",
		prox: Proxy{
			ServiceName:  "rpaas-service-test",
			InstanceName: "rpaas-instance-test",
			Method:       "GET"},
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
	}
	t.Run(tt.name, func(t *testing.T) {
		ts := httptest.NewServer(tt.handler)
		tt.prox.Server = &mockServer{ts: ts, readTokenFunc: func() (string, error) {
			return "", errors.New("Error while parsing token")
		}}
		defer ts.Close()
		_, err := tt.prox.ProxyRequest()
		tt.assertion(t, err)
	})
}
