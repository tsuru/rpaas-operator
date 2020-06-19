// Copyright 2019 tsuru authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"testing"

	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tsuru/rpaas-operator/internal/pkg/rpaas"
	"github.com/tsuru/rpaas-operator/internal/pkg/rpaas/fake"
)

func Test_deleteRoute(t *testing.T) {
	tests := []struct {
		name         string
		instance     string
		requestBody  string
		expectedCode int
		expectedBody string
		manager      rpaas.RpaasManager
	}{
		{
			name:         "when delete route is successful",
			instance:     "my-instance",
			requestBody:  "path=/my/custom/path",
			expectedCode: http.StatusOK,
			manager: &fake.RpaasManager{
				FakeDeleteRoute: func(instanceName, path string) error {
					assert.Equal(t, "my-instance", instanceName)
					assert.Equal(t, "/my/custom/path", path)
					return nil
				},
			},
		},
		{
			name:         "when delete route method returns some error",
			instance:     "my-instance",
			requestBody:  "path=/",
			expectedCode: http.StatusBadRequest,
			expectedBody: "some error",
			manager: &fake.RpaasManager{
				FakeDeleteRoute: func(instanceName, path string) error {
					assert.Equal(t, "my-instance", instanceName)
					assert.Equal(t, "/", path)
					return &rpaas.ValidationError{Msg: "some error"}
				},
			},
		},
		{
			name:         "when path is url encoded",
			instance:     "my-instance",
			requestBody:  "path=%2Fmy%2Fpath",
			expectedCode: http.StatusOK,
			manager: &fake.RpaasManager{
				FakeDeleteRoute: func(instanceName, path string) error {
					assert.Equal(t, "my-instance", instanceName)
					assert.Equal(t, "/my/path", path)
					return nil
				},
			},
		},
		{
			name:         "when request has no body message",
			instance:     "my-instance",
			expectedCode: http.StatusBadRequest,
			manager:      &fake.RpaasManager{},
			expectedBody: "missing body message",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			srv := newTestingServer(t, tt.manager)
			defer srv.Close()
			path := fmt.Sprintf("%s/resources/%s/route", srv.URL, tt.instance)
			request, err := http.NewRequest(http.MethodDelete, path, strings.NewReader(tt.requestBody))
			require.NoError(t, err)
			request.Header.Set(echo.HeaderContentType, echo.MIMEApplicationForm)
			rsp, err := srv.Client().Do(request)
			require.NoError(t, err)
			assert.Equal(t, tt.expectedCode, rsp.StatusCode)
			assert.Regexp(t, tt.expectedBody, bodyContent(rsp))
		})
	}
}

func Test_getRoutes(t *testing.T) {
	tests := []struct {
		name           string
		instance       string
		expectedCode   int
		expectedRoutes []rpaas.Route
		manager        rpaas.RpaasManager
	}{
		{
			name:           "when instance has no routes",
			instance:       "my-instance",
			expectedCode:   http.StatusOK,
			expectedRoutes: []rpaas.Route{},
			manager: &fake.RpaasManager{
				FakeGetRoutes: func(instanceName string) ([]rpaas.Route, error) {
					assert.Equal(t, "my-instance", instanceName)
					return nil, nil
				},
			},
		},
		{
			name:         "when instance has many routes",
			instance:     "my-instance",
			expectedCode: http.StatusOK,
			expectedRoutes: []rpaas.Route{
				{
					Path:    "/path1",
					Content: "# My custom NGINX config",
				},
				{
					Path:        "/path2",
					Destination: "app2.tsuru.example.com",
					HTTPSOnly:   true,
				},
				{
					Path:        "/path3",
					Destination: "app3.tsuru.example.com",
				},
			},
			manager: &fake.RpaasManager{
				FakeGetRoutes: func(instanceName string) ([]rpaas.Route, error) {
					assert.Equal(t, "my-instance", instanceName)
					return []rpaas.Route{
						{
							Path:    "/path1",
							Content: "# My custom NGINX config",
						},
						{
							Path:        "/path2",
							Destination: "app2.tsuru.example.com",
							HTTPSOnly:   true,
						},
						{
							Path:        "/path3",
							Destination: "app3.tsuru.example.com",
						},
					}, nil
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			srv := newTestingServer(t, tt.manager)
			defer srv.Close()
			path := fmt.Sprintf("%s/resources/%s/route", srv.URL, tt.instance)
			request, err := http.NewRequest(http.MethodGet, path, nil)
			require.NoError(t, err)
			rsp, err := srv.Client().Do(request)
			require.NoError(t, err)
			assert.Equal(t, tt.expectedCode, rsp.StatusCode)
			if tt.expectedRoutes != nil {
				var result map[string][]rpaas.Route
				body := bodyContent(rsp)
				err = json.Unmarshal([]byte(body), &result)
				require.NoError(t, err)
				require.Contains(t, result, "paths")
				assert.Equal(t, tt.expectedRoutes, result["paths"])
			}
		})
	}
}

func Test_updateRoute(t *testing.T) {
	tests := []struct {
		name         string
		instance     string
		requestBody  string
		expectedCode int
		expectedBody string
		manager      rpaas.RpaasManager
	}{
		{
			name:         "when update route retunrs no error",
			instance:     "my-instance",
			requestBody:  "path=/path1&destination=app1.tsuru.example.com&https_only=true",
			expectedCode: http.StatusCreated,
			manager: &fake.RpaasManager{
				FakeUpdateRoute: func(instanceName string, route rpaas.Route) error {
					assert.Equal(t, "my-instance", instanceName)
					assert.Equal(t, rpaas.Route{
						Path:        "/path1",
						Destination: "app1.tsuru.example.com",
						HTTPSOnly:   true,
					}, route)
					return nil
				},
			},
		},
		{
			name:         "when update route returns some error",
			instance:     "my-instance",
			requestBody:  "path=/path1&content=%23%20My%20NGINX%20configurations!",
			expectedCode: http.StatusBadRequest,
			expectedBody: "some error",
			manager: &fake.RpaasManager{
				FakeUpdateRoute: func(instanceName string, route rpaas.Route) error {
					assert.Equal(t, "my-instance", instanceName)
					assert.Equal(t, rpaas.Route{
						Path:    "/path1",
						Content: "# My NGINX configurations!",
					}, route)
					return &rpaas.ValidationError{Msg: "some error"}
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			srv := newTestingServer(t, tt.manager)
			defer srv.Close()
			path := fmt.Sprintf("%s/resources/%s/route", srv.URL, tt.instance)
			request, err := http.NewRequest(http.MethodPost, path, strings.NewReader(tt.requestBody))
			require.NoError(t, err)
			request.Header.Set(echo.HeaderContentType, echo.MIMEApplicationForm)
			rsp, err := srv.Client().Do(request)
			require.NoError(t, err)
			request.Header.Set(echo.HeaderContentType, echo.MIMEApplicationForm)
			assert.Equal(t, tt.expectedCode, rsp.StatusCode)
			assert.Regexp(t, tt.expectedBody, bodyContent(rsp))
		})
	}
}
