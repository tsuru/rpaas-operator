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
	"github.com/tsuru/rpaas-operator/internal/pkg/rpaas"
	"github.com/tsuru/rpaas-operator/internal/pkg/rpaas/fake"
)

func Test_getServiceFlavors(t *testing.T) {
	tests := []struct {
		name         string
		manager      rpaas.RpaasManager
		expectedCode int
		expectedBody string
	}{
		{
			name: "when no flavors are available, should return an empty array",
			manager: &fake.RpaasManager{
				FakeGetFlavors: func() ([]rpaas.Flavor, error) {
					return nil, nil
				},
			},
			expectedCode: http.StatusOK,
			expectedBody: `\[\]`,
		},
		{
			name: "when there are many flavors, should return them",
			manager: &fake.RpaasManager{
				FakeGetFlavors: func() ([]rpaas.Flavor, error) {
					return []rpaas.Flavor{
						{
							Name:        "a-flavor",
							Description: "The greatest A flavor",
						},
						{
							Name:        "flavor-1",
							Description: "Some description about flavor 1",
						},
					}, nil
				},
			},
			expectedCode: http.StatusOK,
			expectedBody: `\[\{"name":"a-flavor","description":"The greatest A flavor"\},\{"name":"flavor-1","description":"Some description about flavor 1"\}\]`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			srv := newTestingServer(t, tt.manager)
			defer srv.Close()
			path := fmt.Sprintf("%s/resources/flavors", srv.URL)
			request, err := http.NewRequest(http.MethodGet, path, nil)
			require.NoError(t, err)
			rsp, err := srv.Client().Do(request)
			require.NoError(t, err)
			assert.Equal(t, tt.expectedCode, rsp.StatusCode)
			assert.Regexp(t, tt.expectedBody, bodyContent(rsp))
		})
	}
}

func Test_getInstanceFlavors(t *testing.T) {
	tests := []struct {
		name         string
		manager      rpaas.RpaasManager
		instance     string
		expectedCode int
		expectedBody string
	}{
		{
			name:         "when no flavors are available, should return an empty array",
			manager:      &fake.RpaasManager{},
			instance:     "my-instance",
			expectedCode: http.StatusOK,
			expectedBody: `\[\]`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			srv := newTestingServer(t, tt.manager)
			defer srv.Close()
			path := fmt.Sprintf("%s/resources/%s/flavors", srv.URL, tt.instance)
			request, err := http.NewRequest(http.MethodGet, path, nil)
			require.NoError(t, err)
			rsp, err := srv.Client().Do(request)
			require.NoError(t, err)
			assert.Equal(t, tt.expectedCode, rsp.StatusCode)
			assert.Regexp(t, tt.expectedBody, bodyContent(rsp))
		})
	}
}
