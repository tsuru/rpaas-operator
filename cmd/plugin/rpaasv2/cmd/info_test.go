// Copyright 2019 tsuru authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package cmd

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/tsuru/rpaas-operator/build/cli/proxy"
	"gotest.tools/assert"
)

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
				assert.Error(t, err, "Status Code: 404 Not Found\nResponse Body:\n")
			},
			handler: func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusNotFound)
			},
		},
		{
			name: "when valid flags are passed",
			info: infoArgs{service: "rpaas-service-test", instance: "rpaas-instance-test",
				prox: proxy.New("rpaas-service-test", "rpaas-instance-test", "GET", nil)},

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
				prox: proxy.New("rpaas-service-test", "rpaas-instance-test", "GET", nil)},

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
