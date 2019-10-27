// Copyright 2019 tsuru authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package api

import (
	"fmt"
	"net/http"
	"strings"
	"testing"

	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/assert"
	"github.com/tsuru/rpaas-operator/internal/pkg/rpaas"
	"github.com/tsuru/rpaas-operator/internal/pkg/rpaas/fake"
)

func pointerToInt(x int32) *int32 {
	return &x
}

func Test_getAutoscale(t *testing.T) {
	tests := []struct {
		name         string
		instance     string
		expectedCode int
		expectedBody string
		manager      rpaas.RpaasManager
	}{
		{
			name:         "when instance does not exists",
			instance:     "invalid-instance",
			expectedCode: http.StatusNotFound,
			expectedBody: `{"Msg":"rpaas instance \\"invalid-instance\\" not found"}`,
			manager: &fake.RpaasManager{
				FakeGetAutoscale: func(instance string) (*rpaas.Autoscale, error) {
					assert.Equal(t, "invalid-instance", instance)
					return nil, rpaas.NotFoundError{Msg: fmt.Sprintf("rpaas instance %q not found", instance)}
				},
			},
		},
		{
			name:         "when instance exists and has no autoscale",
			instance:     "my-instance",
			expectedCode: http.StatusOK,
			expectedBody: `{"Autoscale":{}}`,
			manager: &fake.RpaasManager{
				FakeGetAutoscale: func(instance string) (*rpaas.Autoscale, error) {
					assert.Equal(t, "my-instance", instance)
					return nil, nil
				},
			},
		},
		{
			name:         "when successfully getting autoscale settings",
			instance:     "my-instance",
			expectedCode: http.StatusOK,
			expectedBody: `{"Autoscale":{"minReplicas":3,"maxReplicas":10,"cpu":60,"memory":512}}`,
			manager: &fake.RpaasManager{
				FakeGetAutoscale: func(instance string) (*rpaas.Autoscale, error) {
					assert.Equal(t, "my-instance", instance)
					s := &rpaas.Autoscale{
						MaxReplicas: 10,
						MinReplicas: pointerToInt(3),
						CPU:         pointerToInt(60),
						Memory:      pointerToInt(512),
					}
					return s, nil
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			srv := newTestingServer(t, tt.manager)
			defer srv.Close()
			path := fmt.Sprintf("%s/resources/%s/autoscale", srv.URL, tt.instance)
			request, err := http.NewRequest(http.MethodGet, path, nil)
			assert.NoError(t, err)
			rsp, err := srv.Client().Do(request)
			assert.NoError(t, err)
			assert.Equal(t, tt.expectedCode, rsp.StatusCode)
			assert.Regexp(t, tt.expectedBody, bodyContent(rsp))
		})
	}
}

func Test_createAutoscale(t *testing.T) {
	tests := []struct {
		name         string
		instance     string
		requestBody  string
		expectedCode int
		expectedBody string
		manager      rpaas.RpaasManager
	}{
		{
			name:         "when instance does not exists",
			instance:     "invalid-instance",
			requestBody:  "max=10",
			expectedCode: http.StatusNotFound,
			expectedBody: `{"Msg":"rpaas instance \\"invalid-instance\\" not found"}`,
			manager: &fake.RpaasManager{
				FakeCreateAutoscale: func(instance string, autoscale *rpaas.Autoscale) error {
					assert.Equal(t, "invalid-instance", instance)
					assert.Equal(t, int32(10), autoscale.MaxReplicas)
					return rpaas.NotFoundError{Msg: fmt.Sprintf("rpaas instance %q not found", instance)}
				},
			},
		},
		{
			name:         "when instance exists and has no autoscale",
			instance:     "my-instance",
			requestBody:  "min=10",
			expectedCode: http.StatusBadRequest,
			expectedBody: `{"Msg":"max replicas is required"}`,
			manager: &fake.RpaasManager{
				FakeCreateAutoscale: func(instance string, autoscale *rpaas.Autoscale) error {
					assert.Equal(t, "my-instance", instance)
					assert.Equal(t, pointerToInt(10), autoscale.MinReplicas)
					return rpaas.ValidationError{Msg: "max replicas is required"}
				},
			},
		},
		{
			name:         "when successfully creating autoscale settings",
			instance:     "my-instance",
			requestBody:  "max=10&min=3&cpu=60&memory=512",
			expectedCode: http.StatusOK,
			expectedBody: ``,
			manager: &fake.RpaasManager{
				FakeCreateAutoscale: func(instance string, autoscale *rpaas.Autoscale) error {
					assert.Equal(t, "my-instance", instance)
					expectedAutoscale := &rpaas.Autoscale{
						MaxReplicas: 10,
						MinReplicas: pointerToInt(3),
						CPU:         pointerToInt(60),
						Memory:      pointerToInt(512),
					}
					assert.Equal(t, autoscale, expectedAutoscale)
					return nil
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			srv := newTestingServer(t, tt.manager)
			defer srv.Close()
			path := fmt.Sprintf("%s/resources/%s/autoscale", srv.URL, tt.instance)
			request, err := http.NewRequest(http.MethodPost, path, strings.NewReader(tt.requestBody))
			assert.NoError(t, err)
			request.Header.Set(echo.HeaderContentType, echo.MIMEApplicationForm)
			rsp, err := srv.Client().Do(request)
			assert.NoError(t, err)
			assert.Equal(t, tt.expectedCode, rsp.StatusCode)
			assert.Regexp(t, tt.expectedBody, bodyContent(rsp))
		})
	}
}

func Test_updateAutoscale(t *testing.T) {
	tests := []struct {
		name         string
		instance     string
		requestBody  string
		expectedCode int
		expectedBody string
		manager      rpaas.RpaasManager
	}{
		{
			name:         "when instance does not exists",
			instance:     "invalid-instance",
			requestBody:  "max=10",
			expectedCode: http.StatusNotFound,
			expectedBody: `{"Msg":"rpaas instance \\"invalid-instance\\" not found"}`,
			manager: &fake.RpaasManager{
				FakeUpdateAutoscale: func(instance string, autoscale *rpaas.Autoscale) error {
					assert.Equal(t, "invalid-instance", instance)
					assert.Equal(t, int32(10), autoscale.MaxReplicas)
					return rpaas.NotFoundError{Msg: fmt.Sprintf("rpaas instance %q not found", instance)}
				},
			},
		},
		{
			name:         "when instance exists and has no autoscale",
			instance:     "my-instance",
			requestBody:  "min=10",
			expectedCode: http.StatusBadRequest,
			expectedBody: `{"Msg":"max replicas is required"}`,
			manager: &fake.RpaasManager{
				FakeUpdateAutoscale: func(instance string, autoscale *rpaas.Autoscale) error {
					assert.Equal(t, "my-instance", instance)
					assert.Equal(t, pointerToInt(10), autoscale.MinReplicas)
					return rpaas.ValidationError{Msg: "max replicas is required"}
				},
			},
		},
		{
			name:         "when successfully creating autoscale settings",
			instance:     "my-instance",
			requestBody:  "max=10&min=3&cpu=60&memory=512",
			expectedCode: http.StatusOK,
			expectedBody: ``,
			manager: &fake.RpaasManager{
				FakeUpdateAutoscale: func(instance string, autoscale *rpaas.Autoscale) error {
					assert.Equal(t, "my-instance", instance)
					expectedAutoscale := &rpaas.Autoscale{
						MaxReplicas: 10,
						MinReplicas: pointerToInt(3),
						CPU:         pointerToInt(60),
						Memory:      pointerToInt(512),
					}
					assert.Equal(t, autoscale, expectedAutoscale)
					return nil
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			srv := newTestingServer(t, tt.manager)
			defer srv.Close()
			path := fmt.Sprintf("%s/resources/%s/autoscale", srv.URL, tt.instance)
			request, err := http.NewRequest(http.MethodPatch, path, strings.NewReader(tt.requestBody))
			assert.NoError(t, err)
			request.Header.Set(echo.HeaderContentType, echo.MIMEApplicationForm)
			rsp, err := srv.Client().Do(request)
			assert.NoError(t, err)
			assert.Equal(t, tt.expectedCode, rsp.StatusCode)
			assert.Regexp(t, tt.expectedBody, bodyContent(rsp))
		})
	}
}

func Test_deleteAutoscale(t *testing.T) {
	tests := []struct {
		name         string
		instance     string
		expectedCode int
		expectedBody string
		manager      rpaas.RpaasManager
	}{
		{
			name:         "when instance does not exists",
			instance:     "invalid-instance",
			expectedCode: http.StatusNotFound,
			expectedBody: `{"Msg":"rpaas instance \\"invalid-instance\\" not found"}`,
			manager: &fake.RpaasManager{
				FakeDeleteAutoscale: func(instance string) error {
					assert.Equal(t, "invalid-instance", instance)
					return rpaas.NotFoundError{Msg: fmt.Sprintf("rpaas instance %q not found", instance)}
				},
			},
		},
		{
			name:         "when successfully getting autoscale settings",
			instance:     "my-instance",
			expectedCode: http.StatusOK,
			expectedBody: ``,
			manager: &fake.RpaasManager{
				FakeDeleteAutoscale: func(instance string) error {
					assert.Equal(t, "my-instance", instance)
					return nil
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			srv := newTestingServer(t, tt.manager)
			defer srv.Close()
			path := fmt.Sprintf("%s/resources/%s/autoscale", srv.URL, tt.instance)
			request, err := http.NewRequest(http.MethodDelete, path, nil)
			assert.NoError(t, err)
			rsp, err := srv.Client().Do(request)
			assert.NoError(t, err)
			assert.Equal(t, tt.expectedCode, rsp.StatusCode)
			assert.Regexp(t, tt.expectedBody, bodyContent(rsp))
		})
	}
}
