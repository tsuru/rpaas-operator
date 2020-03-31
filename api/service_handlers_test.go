// Copyright 2019 tsuru authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	nginxv1alpha1 "github.com/tsuru/nginx-operator/pkg/apis/nginx/v1alpha1"
	"github.com/tsuru/rpaas-operator/internal/pkg/rpaas"
	"github.com/tsuru/rpaas-operator/internal/pkg/rpaas/fake"
	"github.com/tsuru/rpaas-operator/pkg/apis/extensions/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func Test_serviceCreate(t *testing.T) {
	testCases := []struct {
		name         string
		requestBody  string
		expectedCode int
		expectedBody string
		manager      rpaas.RpaasManager
	}{
		{
			name:         "when some error is returned",
			requestBody:  "foo=bar",
			expectedCode: http.StatusBadRequest,
			expectedBody: "some error message",
			manager: &fake.RpaasManager{
				FakeCreateInstance: func(args rpaas.CreateArgs) error {
					assert.Equal(t, rpaas.CreateArgs{}, args)
					return rpaas.ValidationError{Msg: "some error message"}
				},
			},
		},
		{
			name:         "passing all create parameters on body",
			requestBody:  "name=my-instance&description=some%20description&plan=my-plan&team=my-team&tags.0=tsuru&tags.1=rpaas&parameters.flavors.0=orange&parameters.flavors.1=strawberry&parameters.flavors.2=blueberry",
			expectedCode: http.StatusCreated,
			manager: &fake.RpaasManager{
				FakeCreateInstance: func(args rpaas.CreateArgs) error {
					expected := rpaas.CreateArgs{
						Name:        "my-instance",
						Description: "some description",
						Plan:        "my-plan",
						Team:        "my-team",
						Tags:        []string{"tsuru", "rpaas"},
						Parameters: map[string]interface{}{
							"flavors": map[string]interface{}{
								"0": "orange",
								"1": "strawberry",
								"2": "blueberry",
							},
						},
					}
					assert.Equal(t, expected, args)
					assert.Equal(t, []string{"orange", "strawberry", "blueberry"}, args.Flavors())
					return nil
				},
			},
		},
	}

	for _, tt := range testCases {
		t.Run(tt.name, func(t *testing.T) {
			srv := newTestingServer(t, tt.manager)
			defer srv.Close()
			path := fmt.Sprintf("%s/resources", srv.URL)
			request, err := http.NewRequest(http.MethodPost, path, strings.NewReader(tt.requestBody))
			require.NoError(t, err)
			request.Header.Set(echo.HeaderContentType, echo.MIMEApplicationForm)
			rsp, err := srv.Client().Do(request)
			require.NoError(t, err)
			assert.Equal(t, tt.expectedCode, rsp.StatusCode)
			assert.Regexp(t, tt.expectedBody, bodyContent(rsp))
		})
	}
}

func Test_serviceDelete(t *testing.T) {
	testCases := []struct {
		instanceName string
		expectedCode int
		expectedBody string
		manager      rpaas.RpaasManager
	}{
		{
			instanceName: "unkwnown",
			expectedCode: http.StatusNotFound,
			expectedBody: "",
			manager: &fake.RpaasManager{
				FakeDeleteInstance: func(instance string) error {
					return rpaas.NotFoundError{Msg: "rpaas instance \"unkwnown\" not found"}
				},
			},
		},
		{
			instanceName: "my-instance",
			expectedCode: http.StatusOK,
			expectedBody: "",
			manager:      &fake.RpaasManager{},
		},
	}

	for _, tt := range testCases {
		t.Run("", func(t *testing.T) {
			srv := newTestingServer(t, tt.manager)
			defer srv.Close()
			path := fmt.Sprintf("%s/resources/%s", srv.URL, tt.instanceName)
			request, err := http.NewRequest(http.MethodDelete, path, nil)
			require.NoError(t, err)
			rsp, err := srv.Client().Do(request)
			require.NoError(t, err)
			assert.Equal(t, tt.expectedCode, rsp.StatusCode)
			assert.Regexp(t, tt.expectedBody, bodyContent(rsp))
		})
	}
}

func Test_serviceUpdate(t *testing.T) {
	tests := []struct {
		name         string
		instance     string
		requestBody  string
		expectedCode int
		expectedBody string
		manager      rpaas.RpaasManager
	}{
		{
			name:         "when some error is returned",
			instance:     "my-instance",
			requestBody:  "foo=bar",
			expectedCode: http.StatusBadRequest,
			expectedBody: "some error",
			manager: &fake.RpaasManager{
				FakeUpdateInstance: func(instanceName string, args rpaas.UpdateInstanceArgs) error {
					assert.Equal(t, "my-instance", instanceName)
					assert.Equal(t, rpaas.UpdateInstanceArgs{}, args)
					return rpaas.ValidationError{Msg: "some error"}
				},
			},
		},
		{
			name:         "passing all update parameters on body",
			instance:     "other-instance",
			requestBody:  "description=some%20description&plan=huge&team=team-one&tags.0=tag1&tags.1=tag2&parameters.flavors.0=orange&parameters.flavors.1=mango",
			expectedCode: http.StatusOK,
			manager: &fake.RpaasManager{
				FakeUpdateInstance: func(instanceName string, args rpaas.UpdateInstanceArgs) error {
					assert.Equal(t, "other-instance", instanceName)
					assert.Equal(t, rpaas.UpdateInstanceArgs{
						Description: "some description",
						Plan:        "huge",
						Tags:        []string{"tag1", "tag2"},
						Team:        "team-one",
						Parameters: map[string]interface{}{
							"flavors": map[string]interface{}{
								"0": "orange",
								"1": "mango",
							},
						},
					}, args)
					assert.Equal(t, []string{"orange", "mango"}, args.Flavors())
					return nil
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			srv := newTestingServer(t, tt.manager)
			defer srv.Close()
			path := fmt.Sprintf("%s/resources/%s", srv.URL, tt.instance)
			request, err := http.NewRequest(http.MethodPut, path, strings.NewReader(tt.requestBody))
			require.NoError(t, err)
			request.Header.Set(echo.HeaderContentType, echo.MIMEApplicationForm)
			rsp, err := srv.Client().Do(request)
			require.NoError(t, err)
			assert.Equal(t, tt.expectedCode, rsp.StatusCode)
			assert.Regexp(t, tt.expectedBody, bodyContent(rsp))
		})
	}
}

func Test_servicePlans(t *testing.T) {
	testCases := []struct {
		name          string
		expectedCode  int
		expectedError string
		expectedPlans []rpaas.Plan
		manager       rpaas.RpaasManager
	}{
		{
			name:          "when returns some error",
			expectedCode:  http.StatusConflict,
			expectedError: "some error",
			manager: &fake.RpaasManager{
				FakeGetPlans: func() ([]rpaas.Plan, error) {
					return nil, rpaas.ConflictError{Msg: "some error"}
				},
			},
		},
		{
			name:          "when has no plans",
			expectedCode:  http.StatusOK,
			expectedPlans: []rpaas.Plan{},
			manager: &fake.RpaasManager{
				FakeGetPlans: func() ([]rpaas.Plan, error) {
					return nil, nil
				},
			},
		},
		{
			name:         "when returns several plans",
			expectedCode: http.StatusOK,
			expectedPlans: []rpaas.Plan{
				{
					Name: "my-plan",
				},
				{
					Name:        "my-default-plan",
					Description: "Some description about my-default-plan.",
				},
			},
			manager: &fake.RpaasManager{
				FakeGetPlans: func() ([]rpaas.Plan, error) {
					return []rpaas.Plan{
						{
							Name: "my-plan",
						},
						{
							Name:        "my-default-plan",
							Description: "Some description about my-default-plan.",
						},
					}, nil
				},
			},
		},
	}

	for _, tt := range testCases {
		t.Run(tt.name, func(t *testing.T) {
			srv := newTestingServer(t, tt.manager)
			defer srv.Close()
			path := fmt.Sprintf("%s/resources/plans", srv.URL)
			request, err := http.NewRequest(http.MethodGet, path, nil)
			require.NoError(t, err)
			rsp, err := srv.Client().Do(request)
			require.NoError(t, err)
			assert.Equal(t, tt.expectedCode, rsp.StatusCode)
			if tt.expectedError != "" {
				assert.Regexp(t, tt.expectedError, bodyContent(rsp))
				return
			}
			var result []rpaas.Plan
			require.NoError(t, json.Unmarshal([]byte(bodyContent(rsp)), &result))
			assert.Equal(t, tt.expectedPlans, result)
		})
	}
}

func Test_serviceInfo(t *testing.T) {
	getAddressOfInt32 := func(n int32) *int32 {
		return &n
	}

	testCases := []struct {
		instanceName string
		expectedCode int
		expectedInfo []map[string]string
		manager      rpaas.RpaasManager
	}{
		{
			instanceName: "my-instance",
			expectedCode: http.StatusOK,
			expectedInfo: []map[string]string{
				{
					"label": "Address",
					"value": "pending",
				},
				{
					"label": "Instances",
					"value": "0",
				},
				{
					"label": "Routes",
					"value": "",
				},
			},
			manager: &fake.RpaasManager{
				FakeGetInstance: func(string) (*v1alpha1.RpaasInstance, error) {
					return &v1alpha1.RpaasInstance{
						TypeMeta: metav1.TypeMeta{
							APIVersion: "extensions.tsuru.io/v1alpha1",
							Kind:       "RpaasInstance",
						},
						ObjectMeta: metav1.ObjectMeta{
							Name: "my-instance",
						},
						Spec: v1alpha1.RpaasInstanceSpec{},
					}, nil
				},
				FakeInstanceAddress: func(string) (string, error) {
					return "", nil
				},
			},
		},
		{
			instanceName: "my-instance",
			expectedCode: http.StatusOK,
			expectedInfo: []map[string]string{
				{
					"label": "Address",
					"value": "127.0.0.1",
				},
				{
					"label": "Instances",
					"value": "5",
				},
				{
					"label": "Routes",
					"value": "/status\n/admin",
				},
			},
			manager: &fake.RpaasManager{
				FakeGetInstance: func(string) (*v1alpha1.RpaasInstance, error) {
					return &v1alpha1.RpaasInstance{
						TypeMeta: metav1.TypeMeta{
							APIVersion: "extensions.tsuru.io/v1alpha1",
							Kind:       "RpaasInstance",
						},
						ObjectMeta: metav1.ObjectMeta{
							Name: "my-instance",
						},
						Spec: v1alpha1.RpaasInstanceSpec{
							Replicas: getAddressOfInt32(5),
							Service: &nginxv1alpha1.NginxService{
								LoadBalancerIP: "127.0.0.1",
							},
							Locations: []v1alpha1.Location{
								{Path: "/status"},
								{Path: "/admin"},
							},
						},
					}, nil
				},
				FakeInstanceAddress: func(string) (string, error) {
					return "127.0.0.1", nil
				},
			},
		},
	}

	for _, tt := range testCases {
		t.Run("", func(t *testing.T) {
			srv := newTestingServer(t, tt.manager)
			defer srv.Close()
			path := fmt.Sprintf("%s/resources/%s", srv.URL, tt.instanceName)
			request, err := http.NewRequest(http.MethodGet, path, nil)
			require.NoError(t, err)
			rsp, err := srv.Client().Do(request)
			require.NoError(t, err)
			assert.Equal(t, tt.expectedCode, rsp.StatusCode)
			var info []map[string]string
			require.NoError(t, json.Unmarshal([]byte(bodyContent(rsp)), &info))
			assert.Equal(t, tt.expectedInfo, info)
		})
	}
}

func Test_serviceBindApp(t *testing.T) {
	tests := []struct {
		name         string
		requestBody  string
		expectedCode int
		manager      rpaas.RpaasManager
	}{
		{
			name:         "when no request body is sent",
			expectedCode: http.StatusBadRequest,
			manager:      &fake.RpaasManager{},
		},
		{
			name:         "when bind with application is successful",
			requestBody:  "app-host=app1.tsuru.example.com&app-name=app1&user=admin@tsuru.example.com&eventid=123456",
			expectedCode: http.StatusCreated,
			manager: &fake.RpaasManager{
				FakeBindApp: func(instanceName string, args rpaas.BindAppArgs) error {
					assert.Equal(t, "my-instance", instanceName)
					expected := rpaas.BindAppArgs{
						AppName: "app1",
						AppHost: "app1.tsuru.example.com",
						User:    "admin@tsuru.example.com",
						EventID: "123456",
					}
					assert.Equal(t, expected, args)
					return nil
				},
			},
		},
		{
			name:         "when BindApp method returns an error",
			expectedCode: http.StatusBadRequest,
			manager: &fake.RpaasManager{
				FakeBindApp: func(instanceName string, args rpaas.BindAppArgs) error {
					return &rpaas.ValidationError{Msg: "some error"}
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			srv := newTestingServer(t, tt.manager)
			defer srv.Close()
			path := fmt.Sprintf("%s/resources/my-instance/bind-app", srv.URL)
			request, err := http.NewRequest(http.MethodPost, path, strings.NewReader(tt.requestBody))
			require.NoError(t, err)
			request.Header.Set(echo.HeaderContentType, echo.MIMEApplicationForm)
			rsp, err := srv.Client().Do(request)
			require.NoError(t, err)
			assert.Equal(t, tt.expectedCode, rsp.StatusCode)
		})
	}
}

func Test_serviceUnbindApp(t *testing.T) {
	tests := []struct {
		name         string
		instance     string
		requestBody  string
		appName      string
		expectedCode int
		manager      rpaas.RpaasManager
	}{
		{
			name:         "when unbind method returns no error",
			instance:     "my-instance",
			appName:      "some-app",
			requestBody:  "app-name=some-app",
			expectedCode: http.StatusOK,
			manager: &fake.RpaasManager{
				FakeUnbindApp: func(instanceName, appName string) error {
					assert.Equal(t, "my-instance", instanceName)
					assert.Equal(t, "some-app", appName)
					return nil
				},
			},
		},
		{
			name:         "when UnbindApp returns an error",
			instance:     "my-instance",
			requestBody:  "app-name=some-app",
			expectedCode: http.StatusBadRequest,
			manager: &fake.RpaasManager{
				FakeUnbindApp: func(instanceName, appName string) error {
					return &rpaas.ValidationError{Msg: "some error"}
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			webApi, err := New(nil)
			require.NoError(t, err)
			webApi.rpaasManager = tt.manager
			srv := httptest.NewServer(webApi.Handler())
			defer srv.Close()
			path := fmt.Sprintf("%s/resources/%s/bind-app", srv.URL, tt.instance)
			request, err := http.NewRequest(http.MethodDelete, path, strings.NewReader(tt.requestBody))
			require.NoError(t, err)
			request.Header.Set(echo.HeaderContentType, echo.MIMEApplicationForm)
			rsp, err := srv.Client().Do(request)
			require.NoError(t, err)
			assert.Equal(t, tt.expectedCode, rsp.StatusCode)
		})
	}
}

func Test_serviceBindUnit(t *testing.T) {
	t.Run("ensure bind unit route exists", func(t *testing.T) {
		instance := "my-instance"
		requestBody := "app-name=app1&app-hosts=app1.tsuru.example.com&unit-host=127.0.0.1:32123"
		srv := newTestingServer(t, &fake.RpaasManager{})
		defer srv.Close()
		path := fmt.Sprintf("%s/resources/%s/bind", srv.URL, instance)
		request, err := http.NewRequest(http.MethodPost, path, strings.NewReader(requestBody))
		require.NoError(t, err)
		request.Header.Set(echo.HeaderContentType, echo.MIMEApplicationForm)
		rsp, err := srv.Client().Do(request)
		require.NoError(t, err)
		assert.Equal(t, http.StatusCreated, rsp.StatusCode)
	})
}

func Test_serviceUnbindUnit(t *testing.T) {
	t.Run("ensure unbind unit route exists", func(t *testing.T) {
		instance := "my-instance"
		requestBody := "app-hosts=app1.tsuru.example.com&unit-host=127.0.0.1:32123"
		srv := newTestingServer(t, &fake.RpaasManager{})
		defer srv.Close()
		path := fmt.Sprintf("%s/resources/%s/bind", srv.URL, instance)
		request, err := http.NewRequest(http.MethodDelete, path, strings.NewReader(requestBody))
		require.NoError(t, err)
		request.Header.Set(echo.HeaderContentType, echo.MIMEApplicationForm)
		rsp, err := srv.Client().Do(request)
		require.NoError(t, err)
		assert.Equal(t, http.StatusOK, rsp.StatusCode)
	})
}

func Test_serviceStatus(t *testing.T) {
	tests := []struct {
		name         string
		instance     string
		expectedCode int
		manager      rpaas.RpaasManager
	}{
		{
			name:         "when manager returns an error, should return it",
			instance:     "not-found-instance",
			expectedCode: http.StatusNotFound,
			manager: &fake.RpaasManager{
				FakeInstanceAddress: func(name string) (string, error) {
					assert.Equal(t, "not-found-instance", name)
					return "", rpaas.NotFoundError{Msg: "instance not found"}
				},
			},
		},
		{
			name:         "when returned address empty, should return 202",
			instance:     "my-instance",
			expectedCode: http.StatusAccepted,
			manager: &fake.RpaasManager{
				FakeInstanceAddress: func(name string) (string, error) {
					assert.Equal(t, "my-instance", name)
					return "", nil
				},
			},
		},
		{
			name:         "when address is not empty, should return 204",
			instance:     "another-instance",
			expectedCode: http.StatusNoContent,
			manager: &fake.RpaasManager{
				FakeInstanceAddress: func(name string) (string, error) {
					assert.Equal(t, "another-instance", name)
					return "169.254.10.10", nil
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			srv := newTestingServer(t, tt.manager)
			defer srv.Close()
			path := fmt.Sprintf("%s/resources/%s/status", srv.URL, tt.instance)
			request, err := http.NewRequest(http.MethodGet, path, nil)
			require.NoError(t, err)
			rsp, err := srv.Client().Do(request)
			require.NoError(t, err)
			assert.Equal(t, tt.expectedCode, rsp.StatusCode)
		})
	}
}
