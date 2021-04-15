// Copyright 2021 tsuru authors. All rights reserved.
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

func TestGetUpstreams(t *testing.T) {
	tests := []struct {
		name         string
		instance     string
		expectedCode int
		expectedBody string
		manager      rpaas.RpaasManager
	}{
		{
			name:         "upstreams not empty",
			instance:     "valid",
			expectedCode: http.StatusOK,
			expectedBody: `[{"host":"host1","port":"8888"},{"host":"host2","port":"8889"}]`,
			manager: &fake.RpaasManager{
				FakeGetUpstreams: func(instance string) ([]v1alpha1.AllowedUpstream, error) {
					return []v1alpha1.AllowedUpstream{
						{Host: "host1", Port: 8888},
						{Host: "host2", Port: 8889},
					}, nil
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			srv := newTestingServer(t, tt.manager)
			defer srv.Close()

			path := fmt.Sprintf("%s/resources/%s/acl", srv.URL, tt.instance)
			request, err := http.NewRequest(http.MethodGet, path, nil)
			assert.NoError(t, err)

			rsp, err := srv.Client().Do(request)
			assert.NoError(t, err)
			assert.Equal(t, tt.expectedCode, rsp.StatusCode)
			assert.Regexp(t, tt.expectedBody, bodyContent(rsp))
		})
	}
}

func TestAddUpstream(t *testing.T) {
	tests := []struct {
		name         string
		instance     string
		requestBody  string
		expectedCode int
		expectedBody string
		manager      rpaas.RpaasManager
	}{
		{
			name:         "add upstream",
			instance:     "valid",
			requestBody:  `{"host":"host1","port":8888}`,
			expectedCode: http.StatusCreated,
			expectedBody: "",
			manager: &fake.RpaasManager{
				FakeAddUpstream: func(instanceName string, upstream v1alpha1.AllowedUpstream) error {
					assert.Equal(t, v1alpha1.AllowedUpstream{
						Host: "host1",
						Port: 8888,
					}, upstream)
					return nil
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			srv := newTestingServer(t, tt.manager)
			defer srv.Close()

			path := fmt.Sprintf("%s/resources/%s/acl", srv.URL, tt.instance)
			request, err := http.NewRequest(http.MethodPost, path, strings.NewReader(tt.requestBody))
			assert.NoError(t, err)
			request.Header.Add("Content-Type", "application/json")

			rsp, err := srv.Client().Do(request)
			assert.NoError(t, err)
			assert.Equal(t, tt.expectedCode, rsp.StatusCode)
			assert.Regexp(t, tt.expectedBody, bodyContent(rsp))
		})
	}
}

func TestRemoveAccessControlList(t *testing.T) {
	tests := []struct {
		name         string
		instance     string
		args         string
		expectedCode int
		expectedBody string
		manager      rpaas.RpaasManager
	}{
		{
			name:         "remove upstream",
			instance:     "valid",
			args:         "host=host1&port=8888",
			expectedCode: http.StatusNoContent,
			expectedBody: "",
			manager: &fake.RpaasManager{
				FakeDeleteUpstream: func(instanceName string, upstream v1alpha1.AllowedUpstream) error {
					expectedUpstream := v1alpha1.AllowedUpstream{Host: "host1", Port: 8888}
					assert.Equal(t, expectedUpstream, upstream)
					return nil
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			srv := newTestingServer(t, tt.manager)
			defer srv.Close()

			path := fmt.Sprintf("%s/resources/%s/acl", srv.URL, tt.instance)
			if tt.args != "" {
				path = fmt.Sprintf("%s?%s", path, tt.args)
			}
			request, err := http.NewRequest(http.MethodDelete, path, nil)
			assert.NoError(t, err)

			rsp, err := srv.Client().Do(request)
			assert.NoError(t, err)
			assert.Equal(t, tt.expectedCode, rsp.StatusCode)
			assert.Regexp(t, tt.expectedBody, bodyContent(rsp))
		})
	}
}
