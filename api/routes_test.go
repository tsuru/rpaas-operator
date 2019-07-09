package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"testing"

	"github.com/labstack/echo"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tsuru/rpaas-operator/rpaas"
	"github.com/tsuru/rpaas-operator/rpaas/fake"
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
			name:         "when manager is not set",
			instance:     "my-instance",
			expectedCode: http.StatusInternalServerError,
			manager:      nil,
		},
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
		expectedRoutes routes
		manager        rpaas.RpaasManager
	}{
		{
			name:         "when manager is not set",
			instance:     "my-instance",
			expectedCode: http.StatusInternalServerError,
			manager:      nil,
		},
		{
			name:           "when instance has no routes",
			instance:       "my-instance",
			expectedCode:   http.StatusOK,
			expectedRoutes: routes{Paths: []rpaas.Route{}},
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
			expectedRoutes: route{
				Paths: []rpaas.Route{
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
			},
			manager: &fake.RpaasManager{
				FakeGetRoutes: func(instanceName string) ([]rpaas.Route, error) {
					assert.Equal(t, "my-instance", instanceName)
					return route{
						Paths: []rpaas.Route{
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
				var routes []rpaas.Route
				body := bodyContent(rsp)
				err = json.Unmarshal([]byte(body), &routes)
				require.NoError(t, err)
				assert.Equal(t, tt.expectedRoutes, routes)
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
			name:         "when manager is not set",
			instance:     "my-instance",
			expectedCode: http.StatusInternalServerError,
			manager:      nil,
		},
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
