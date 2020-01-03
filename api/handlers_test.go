// Copyright 2019 tsuru authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package api

import (
	"fmt"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tsuru/rpaas-operator/config"
)

func Test_healthcheck(t *testing.T) {
	testCases := []struct {
		name  string
		setup func(*testing.T)
	}{
		{
			name: "without auth",
		},
		{
			name: "with auth",
			setup: func(t *testing.T) {
				config.Set(config.RpaasConfig{APIUsername: "u1", APIPassword: "p1"})
			},
		},
	}

	for _, tt := range testCases {
		t.Run(tt.name, func(t *testing.T) {
			if tt.setup != nil {
				tt.setup(t)
			}
			defer config.Set(config.RpaasConfig{})
			srv := newTestingServer(t, nil)
			defer srv.Close()
			path := fmt.Sprintf("%s/healthcheck", srv.URL)
			request, err := http.NewRequest(http.MethodGet, path, nil)
			require.NoError(t, err)
			rsp, err := srv.Client().Do(request)
			require.NoError(t, err)
			assert.Equal(t, 200, rsp.StatusCode)
			assert.Regexp(t, "OK", bodyContent(rsp))
		})
	}
}

func Test_MiddlewareBasicAuth(t *testing.T) {
	testCases := []struct {
		name         string
		setup        func(*testing.T, *http.Request)
		expectedCode int
	}{
		{
			name:         "without auth",
			expectedCode: 404,
		},
		{
			name: "with auth enabled",
			setup: func(t *testing.T, r *http.Request) {
				config.Set(config.RpaasConfig{APIUsername: "u1", APIPassword: "p1"})
			},
			expectedCode: 401,
		},
		{
			name: "with auth enabled and credentials",
			setup: func(t *testing.T, r *http.Request) {
				config.Set(config.RpaasConfig{APIUsername: "u1", APIPassword: "p1"})
				r.SetBasicAuth("u1", "p1")
			},
			expectedCode: 404,
		},
		{
			name: "with auth enabled and invalid username",
			setup: func(t *testing.T, r *http.Request) {
				config.Set(config.RpaasConfig{APIUsername: "u1", APIPassword: "p1"})
				r.SetBasicAuth("u9", "p1")
			},
			expectedCode: 401,
		},
		{
			name: "with auth enabled and invalid password",
			setup: func(t *testing.T, r *http.Request) {
				config.Set(config.RpaasConfig{APIUsername: "u1", APIPassword: "p1"})
				r.SetBasicAuth("u1", "p9")
			},
			expectedCode: 401,
		},
	}

	for _, tt := range testCases {
		t.Run(tt.name, func(t *testing.T) {
			defer config.Set(config.RpaasConfig{})
			srv := newTestingServer(t, nil)
			defer srv.Close()
			path := fmt.Sprintf("%s/", srv.URL)
			request, err := http.NewRequest(http.MethodGet, path, nil)
			require.NoError(t, err)
			if tt.setup != nil {
				tt.setup(t, request)
			}
			rsp, err := srv.Client().Do(request)
			require.NoError(t, err)
			assert.Equal(t, tt.expectedCode, rsp.StatusCode)
		})
	}
}
