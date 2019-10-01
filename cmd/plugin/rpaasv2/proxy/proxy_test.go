// Copyright 2019 tsuru authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package proxy

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"gotest.tools/assert"
)

type MockServer struct {
	ts            *httptest.Server
	getURLfunc    func(string) (string, error)
	getTargetFunc func() (string, error)
	readTokenFunc func() (string, error)
}

func (ms *MockServer) GetURL(path string) (string, error) {
	if ms.getURLfunc == nil {
		return ms.ts.URL, nil
	}
	return ms.getURLfunc(path)
}

func (ms *MockServer) GetTarget() (string, error) {
	if ms.getTargetFunc == nil {
		return "", nil
	}
	return ms.getTargetFunc()
}

func (ms *MockServer) ReadToken() (string, error) {
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
				Server: &MockServer{getTargetFunc: func() (string, error) {
					return "", errors.New("Error while aquiring target")
				}},
			},
			assertion: func(t *testing.T, err error) {
				assert.Equal(t, err.Error(), "Error while aquiring target")
			},
			handler: func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusOK)
			},
		},
		{
			name: "testing all valid",
			prox: Proxy{ServiceName: "",
				InstanceName: "",
				Method:       "GET",
				Server:       &MockServer{},
			},
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
			name: "testing invalid URL",
			prox: Proxy{
				ServiceName:  "rpaas-service-test",
				InstanceName: "rpaas-instance-test",
				Method:       "GET",
				Server: &MockServer{getURLfunc: func(string) (string, error) {
					return "", errors.New("Error while parsing URL")
				}},
			},
			assertion: func(t *testing.T, err error) {
				assert.Equal(t, err.Error(), "Error while parsing URL")
			},
		},
		{
			name: "testing invalid token",
			prox: Proxy{
				ServiceName:  "rpaas-service-test",
				InstanceName: "rpaas-instance-test",
				Method:       "GET",
				Server: &MockServer{readTokenFunc: func() (string, error) {
					return "", errors.New("Error while parsing token")
				}},
			},
			assertion: func(t *testing.T, err error) {
				assert.Equal(t, err.Error(), "Error while parsing token")
			},
		},
	}
	for _, tt := range testCases {
		t.Run(tt.name, func(t *testing.T) {
			tt.testServer = httptest.NewServer(tt.handler)
			tt.prox.Server.(*MockServer).ts = tt.testServer
			_, err := tt.prox.ProxyRequest()
			tt.assertion(t, err)
		})
	}
}
