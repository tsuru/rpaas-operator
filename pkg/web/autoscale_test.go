// Copyright 2019 tsuru authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package web

import (
	"fmt"
	"net/http"
	"strings"
	"testing"

	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/assert"

	"github.com/tsuru/rpaas-operator/internal/pkg/rpaas"
	"github.com/tsuru/rpaas-operator/internal/pkg/rpaas/fake"
	clientTypes "github.com/tsuru/rpaas-operator/pkg/rpaas/client/types"
)

func pointerToInt32(x int32) *int32 {
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
			expectedBody: `{"message":"rpaas instance \\"invalid-instance\\" not found"}`,
			manager: &fake.RpaasManager{
				FakeGetAutoscale: func(instance string) (*clientTypes.Autoscale, error) {
					assert.Equal(t, "invalid-instance", instance)
					return nil, rpaas.NotFoundError{Msg: fmt.Sprintf("rpaas instance %q not found", instance)}
				},
			},
		},
		{
			name:         "when instance exists and has no autoscale",
			instance:     "my-instance",
			expectedCode: http.StatusOK,
			expectedBody: `{}`,
			manager: &fake.RpaasManager{
				FakeGetAutoscale: func(instance string) (*clientTypes.Autoscale, error) {
					assert.Equal(t, "my-instance", instance)
					return nil, nil
				},
			},
		},
		{
			name:         "when successfully getting autoscale settings",
			instance:     "my-instance",
			expectedCode: http.StatusOK,
			expectedBody: `{"minReplicas":3,"maxReplicas":10,"cpu":60,"memory":512,"rps":500}`,
			manager: &fake.RpaasManager{
				FakeGetAutoscale: func(instance string) (*clientTypes.Autoscale, error) {
					assert.Equal(t, "my-instance", instance)
					s := &clientTypes.Autoscale{
						MaxReplicas: pointerToInt32(10),
						MinReplicas: pointerToInt32(3),
						CPU:         pointerToInt32(60),
						Memory:      pointerToInt32(512),
						RPS:         pointerToInt32(500),
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
			expectedBody: `{"message":"rpaas instance \\"invalid-instance\\" not found"}`,
			manager: &fake.RpaasManager{
				FakeCreateAutoscale: func(instance string, autoscale *clientTypes.Autoscale) error {
					assert.Equal(t, "invalid-instance", instance)
					assert.Equal(t, pointerToInt32(10), autoscale.MaxReplicas)
					return rpaas.NotFoundError{Msg: fmt.Sprintf("rpaas instance %q not found", instance)}
				},
			},
		},
		{
			name:         "when instance exists and has no autoscale",
			instance:     "my-instance",
			requestBody:  "min=10",
			expectedCode: http.StatusBadRequest,
			expectedBody: `{"message":"max replicas is required"}`,
			manager: &fake.RpaasManager{
				FakeCreateAutoscale: func(instance string, autoscale *clientTypes.Autoscale) error {
					assert.Equal(t, "my-instance", instance)
					assert.Equal(t, pointerToInt32(10), autoscale.MinReplicas)
					return rpaas.ValidationError{Msg: "max replicas is required"}
				},
			},
		},
		{
			name:         "when successfully creating autoscale settings",
			instance:     "my-instance",
			requestBody:  "max=10&min=3&cpu=60&memory=512&rps=500",
			expectedCode: http.StatusOK,
			manager: &fake.RpaasManager{
				FakeCreateAutoscale: func(instance string, autoscale *clientTypes.Autoscale) error {
					assert.Equal(t, "my-instance", instance)
					expectedAutoscale := &clientTypes.Autoscale{
						MaxReplicas: pointerToInt32(10),
						MinReplicas: pointerToInt32(3),
						CPU:         pointerToInt32(60),
						Memory:      pointerToInt32(512),
						RPS:         pointerToInt32(500),
					}
					assert.Equal(t, expectedAutoscale, autoscale)
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
			expectedBody: `{"message":"rpaas instance \\"invalid-instance\\" not found"}`,
			manager: &fake.RpaasManager{
				FakeGetAutoscale: func(instance string) (*clientTypes.Autoscale, error) {
					assert.Equal(t, "invalid-instance", instance)
					return nil, rpaas.NotFoundError{Msg: fmt.Sprintf("rpaas instance %q not found", instance)}
				},
				FakeUpdateAutoscale: func(instance string, autoscale *clientTypes.Autoscale) error {
					assert.Fail(t, "Autoscale update should not be called")
					return nil
				},
			},
		},
		{
			name:         "when instance exists and has no autoscale",
			instance:     "my-instance",
			requestBody:  "min=10",
			expectedCode: http.StatusBadRequest,
			expectedBody: `{"message":"max replicas is required"}`,
			manager: &fake.RpaasManager{
				FakeGetAutoscale: func(instance string) (*clientTypes.Autoscale, error) {
					assert.Equal(t, "my-instance", instance)
					return nil, nil
				},
				FakeUpdateAutoscale: func(instance string, autoscale *clientTypes.Autoscale) error {
					assert.Equal(t, "my-instance", instance)
					assert.Equal(t, pointerToInt32(10), autoscale.MinReplicas)
					return rpaas.ValidationError{Msg: "max replicas is required"}
				},
			},
		},
		{
			name:         "when successfully updating autoscale settings",
			instance:     "my-instance",
			requestBody:  "min=5&memory=512",
			expectedCode: http.StatusCreated,
			manager: &fake.RpaasManager{
				FakeGetAutoscale: func(instance string) (*clientTypes.Autoscale, error) {
					assert.Equal(t, "my-instance", instance)
					currentAutoscale := &clientTypes.Autoscale{
						MaxReplicas: pointerToInt32(10),
						MinReplicas: pointerToInt32(3),
						CPU:         pointerToInt32(80),
						Memory:      pointerToInt32(1024),
						RPS:         pointerToInt32(100),
					}
					return currentAutoscale, nil
				},
				FakeUpdateAutoscale: func(instance string, autoscale *clientTypes.Autoscale) error {
					assert.Equal(t, "my-instance", instance)
					expectedAutoscale := &clientTypes.Autoscale{
						MaxReplicas: pointerToInt32(10),
						MinReplicas: pointerToInt32(5),
						CPU:         pointerToInt32(80),
						Memory:      pointerToInt32(512),
						RPS:         pointerToInt32(100),
					}
					assert.Equal(t, expectedAutoscale, autoscale)
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
			expectedBody: `{"message":"rpaas instance \\"invalid-instance\\" not found"}`,
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
