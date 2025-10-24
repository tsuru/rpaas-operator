// Copyright 2025 tsuru authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package web

import (
	"fmt"
	"net/http"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/tsuru/rpaas-operator/api/v1alpha1"
	"github.com/tsuru/rpaas-operator/internal/pkg/rpaas"
	"github.com/tsuru/rpaas-operator/internal/pkg/rpaas/fake"
)

func TestGetUpstreamOptions(t *testing.T) {
	tests := []struct {
		name         string
		instance     string
		expectedCode int
		expectedBody string
		manager      rpaas.RpaasManager
	}{
		{
			name:         "get upstream options successfully",
			instance:     "valid",
			expectedCode: http.StatusOK,
			expectedBody: `\[\{"app":"bind1","canary":\["canary1"\],"trafficShapingPolicy":\{"weight":50,"weightTotal":100\},"loadBalance":"round_robin"\}\]`,
			manager: &fake.RpaasManager{
				FakeGetUpstreamOptions: func(instanceName string) ([]v1alpha1.UpstreamOptions, error) {
					assert.Equal(t, "valid", instanceName)
					return []v1alpha1.UpstreamOptions{
						{
							PrimaryBind: "bind1",
							CanaryBinds: []string{"canary1"},
							TrafficShapingPolicy: v1alpha1.TrafficShapingPolicy{
								Weight:      50,
								WeightTotal: 100,
							},
							LoadBalance: v1alpha1.LoadBalanceRoundRobin,
						},
					}, nil
				},
			},
		},
		{
			name:         "get upstream options empty list",
			instance:     "empty",
			expectedCode: http.StatusOK,
			expectedBody: `\[\]`,
			manager: &fake.RpaasManager{
				FakeGetUpstreamOptions: func(instanceName string) ([]v1alpha1.UpstreamOptions, error) {
					assert.Equal(t, "empty", instanceName)
					return []v1alpha1.UpstreamOptions{}, nil
				},
			},
		},
		{
			name:         "get upstream options manager error",
			instance:     "error",
			expectedCode: http.StatusInternalServerError,
			expectedBody: `{"message":"manager error"}`,
			manager: &fake.RpaasManager{
				FakeGetUpstreamOptions: func(instanceName string) ([]v1alpha1.UpstreamOptions, error) {
					return nil, fmt.Errorf("manager error")
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			srv := newTestingServer(t, tt.manager)
			defer srv.Close()

			path := fmt.Sprintf("%s/resources/%s/upstream-options", srv.URL, tt.instance)
			request, err := http.NewRequest(http.MethodGet, path, nil)
			assert.NoError(t, err)

			rsp, err := srv.Client().Do(request)
			assert.NoError(t, err)
			assert.Equal(t, tt.expectedCode, rsp.StatusCode)
			assert.Regexp(t, tt.expectedBody, bodyContent(rsp))
		})
	}
}

func TestAddUpstreamOptions(t *testing.T) {
	tests := []struct {
		name         string
		instance     string
		requestBody  string
		expectedCode int
		expectedBody string
		manager      rpaas.RpaasManager
	}{
		{
			name:         "add upstream options successfully",
			instance:     "valid",
			requestBody:  `{"app":"bind1","canary":["canary1"],"trafficShapingPolicy":{"weight":50,"weightTotal":100},"loadBalance":"round_robin"}`,
			expectedCode: http.StatusCreated,
			expectedBody: "",
			manager: &fake.RpaasManager{
				FakeEnsureUpstreamOptions: func(instanceName string, args rpaas.UpstreamOptionsArgs) error {
					assert.Equal(t, "valid", instanceName)
					assert.Equal(t, rpaas.UpstreamOptionsArgs{
						PrimaryBind: "bind1",
						CanaryBinds: []string{"canary1"},
						TrafficShapingPolicy: v1alpha1.TrafficShapingPolicy{
							Weight:      50,
							WeightTotal: 100,
						},
						LoadBalance: v1alpha1.LoadBalanceRoundRobin,
					}, args)
					return nil
				},
			},
		},
		{
			name:         "add upstream options with minimal fields",
			instance:     "minimal",
			requestBody:  `{"app":"bind1"}`,
			expectedCode: http.StatusCreated,
			expectedBody: "",
			manager: &fake.RpaasManager{
				FakeEnsureUpstreamOptions: func(instanceName string, args rpaas.UpstreamOptionsArgs) error {
					assert.Equal(t, "minimal", instanceName)
					assert.Equal(t, rpaas.UpstreamOptionsArgs{
						PrimaryBind: "bind1",
					}, args)
					return nil
				},
			},
		},
		{
			name:         "add upstream options empty body",
			instance:     "empty",
			requestBody:  "",
			expectedCode: http.StatusBadRequest,
			expectedBody: `{"message":"Request body can't be empty"}`,
			manager:      &fake.RpaasManager{},
		},
		{
			name:         "add upstream options invalid json",
			instance:     "invalid",
			requestBody:  `{"invalid": json}`,
			expectedCode: http.StatusBadRequest,
			expectedBody: "",
			manager:      &fake.RpaasManager{},
		},
		{
			name:         "add upstream options manager error",
			instance:     "error",
			requestBody:  `{"app":"bind1"}`,
			expectedCode: http.StatusInternalServerError,
			expectedBody: `{"message":"manager error"}`,
			manager: &fake.RpaasManager{
				FakeEnsureUpstreamOptions: func(instanceName string, args rpaas.UpstreamOptionsArgs) error {
					return fmt.Errorf("manager error")
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			srv := newTestingServer(t, tt.manager)
			defer srv.Close()

			path := fmt.Sprintf("%s/resources/%s/upstream-options", srv.URL, tt.instance)
			request, err := http.NewRequest(http.MethodPost, path, strings.NewReader(tt.requestBody))
			assert.NoError(t, err)
			if tt.requestBody != "" {
				request.Header.Add("Content-Type", "application/json")
			}

			rsp, err := srv.Client().Do(request)
			assert.NoError(t, err)
			assert.Equal(t, tt.expectedCode, rsp.StatusCode)
			if tt.expectedBody != "" {
				assert.Regexp(t, tt.expectedBody, bodyContent(rsp))
			}
		})
	}
}

func TestUpdateUpstreamOptions(t *testing.T) {
	tests := []struct {
		name         string
		instance     string
		bind         string
		requestBody  string
		expectedCode int
		expectedBody string
		manager      rpaas.RpaasManager
	}{
		{
			name:         "update upstream options successfully",
			instance:     "valid",
			bind:         "bind1",
			requestBody:  `{"canary":["canary1","canary2"],"trafficShapingPolicy":{"weight":75,"weightTotal":100},"loadBalance":"chash"}`,
			expectedCode: http.StatusOK,
			expectedBody: "",
			manager: &fake.RpaasManager{
				FakeEnsureUpstreamOptions: func(instanceName string, args rpaas.UpstreamOptionsArgs) error {
					assert.Equal(t, "valid", instanceName)
					assert.Equal(t, rpaas.UpstreamOptionsArgs{
						PrimaryBind: "bind1",
						CanaryBinds: []string{"canary1", "canary2"},
						TrafficShapingPolicy: v1alpha1.TrafficShapingPolicy{
							Weight:      75,
							WeightTotal: 100,
						},
						LoadBalance: v1alpha1.LoadBalanceConsistentHash,
					}, args)
					return nil
				},
			},
		},
		{
			name:         "update upstream options partial update",
			instance:     "partial",
			bind:         "bind2",
			requestBody:  `{"loadBalance":"ewma"}`,
			expectedCode: http.StatusOK,
			expectedBody: "",
			manager: &fake.RpaasManager{
				FakeEnsureUpstreamOptions: func(instanceName string, args rpaas.UpstreamOptionsArgs) error {
					assert.Equal(t, "partial", instanceName)
					assert.Equal(t, rpaas.UpstreamOptionsArgs{
						PrimaryBind: "bind2",
						LoadBalance: v1alpha1.LoadBalanceEWMA,
					}, args)
					return nil
				},
			},
		},
		{
			name:         "update upstream options empty body",
			instance:     "empty",
			bind:         "bind3",
			requestBody:  "",
			expectedCode: http.StatusBadRequest,
			expectedBody: `{"message":"Request body can't be empty"}`,
			manager:      &fake.RpaasManager{},
		},
		{
			name:         "update upstream options invalid json",
			instance:     "invalid",
			bind:         "bind4",
			requestBody:  `{"invalid": json}`,
			expectedCode: http.StatusBadRequest,
			expectedBody: "",
			manager:      &fake.RpaasManager{},
		},
		{
			name:         "update upstream options manager error",
			instance:     "error",
			bind:         "bind5",
			requestBody:  `{}`,
			expectedCode: http.StatusInternalServerError,
			expectedBody: `{"message":"manager error"}`,
			manager: &fake.RpaasManager{
				FakeEnsureUpstreamOptions: func(instanceName string, args rpaas.UpstreamOptionsArgs) error {
					return fmt.Errorf("manager error")
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			srv := newTestingServer(t, tt.manager)
			defer srv.Close()

			path := fmt.Sprintf("%s/resources/%s/upstream-options/%s", srv.URL, tt.instance, tt.bind)
			request, err := http.NewRequest(http.MethodPut, path, strings.NewReader(tt.requestBody))
			assert.NoError(t, err)
			if tt.requestBody != "" {
				request.Header.Add("Content-Type", "application/json")
			}

			rsp, err := srv.Client().Do(request)
			assert.NoError(t, err)
			assert.Equal(t, tt.expectedCode, rsp.StatusCode)
			if tt.expectedBody != "" {
				assert.Regexp(t, tt.expectedBody, bodyContent(rsp))
			}
		})
	}
}

func TestDeleteUpstreamOptions(t *testing.T) {
	tests := []struct {
		name         string
		instance     string
		bind         string
		expectedCode int
		expectedBody string
		manager      rpaas.RpaasManager
	}{
		{
			name:         "delete upstream options successfully",
			instance:     "valid",
			bind:         "bind1",
			expectedCode: http.StatusOK,
			expectedBody: "",
			manager: &fake.RpaasManager{
				FakeDeleteUpstreamOptions: func(instanceName, primaryBind string) error {
					assert.Equal(t, "valid", instanceName)
					assert.Equal(t, "bind1", primaryBind)
					return nil
				},
			},
		},
		{
			name:         "delete upstream options manager error",
			instance:     "error",
			bind:         "bind2",
			expectedCode: http.StatusInternalServerError,
			expectedBody: `{"message":"manager error"}`,
			manager: &fake.RpaasManager{
				FakeDeleteUpstreamOptions: func(instanceName, primaryBind string) error {
					return fmt.Errorf("manager error")
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			srv := newTestingServer(t, tt.manager)
			defer srv.Close()

			path := fmt.Sprintf("%s/resources/%s/upstream-options/%s", srv.URL, tt.instance, tt.bind)
			request, err := http.NewRequest(http.MethodDelete, path, nil)
			assert.NoError(t, err)

			rsp, err := srv.Client().Do(request)
			assert.NoError(t, err)
			assert.Equal(t, tt.expectedCode, rsp.StatusCode)
			if tt.expectedBody != "" {
				assert.Regexp(t, tt.expectedBody, bodyContent(rsp))
			}
		})
	}
}
