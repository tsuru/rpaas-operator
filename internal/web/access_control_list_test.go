// Copyright 2021 tsuru authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package web

import (
	"fmt"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/tsuru/rpaas-operator/api/v1alpha1"
	"github.com/tsuru/rpaas-operator/internal/pkg/rpaas"
	"github.com/tsuru/rpaas-operator/internal/pkg/rpaas/fake"
)

func pointerToInt(x int) *int {
	return &x
}

func TestGetAccessControlList(t *testing.T) {
	tests := []struct {
		name         string
		instance     string
		expectedCode int
		expectedBody string
		manager      rpaas.RpaasManager
	}{
		{
			name:         "invalid instance",
			instance:     "invalid",
			expectedCode: http.StatusNotFound,
			expectedBody: `{"Msg":"ACL for instance invalid not found"}`,
			manager: &fake.RpaasManager{
				FakeGetAccessControlList: func(instance string) (*v1alpha1.RpaasAccessControlList, error) {
					return nil, nil
				},
			},
		},
		{
			name:         "acl found",
			instance:     "valid",
			expectedCode: http.StatusOK,
			expectedBody: `[{"host":"host1","port":"8888"},{"host":"host2","port":"8889"}]`,
			manager: &fake.RpaasManager{
				FakeGetAccessControlList: func(instance string) (*v1alpha1.RpaasAccessControlList, error) {
					acl := &v1alpha1.RpaasAccessControlList{
						Spec: v1alpha1.RpaasAccessControlListSpec{
							Items: []v1alpha1.RpaasAccessControlListItem{
								{Host: "host1", Port: pointerToInt(8888)},
								{Host: "host2", Port: pointerToInt(8889)},
							},
						},
					}
					return acl, nil
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

func TestAddAccessControlList(t *testing.T) {
	tests := []struct {
		name         string
		instance     string
		expectedCode int
		expectedBody string
		manager      rpaas.RpaasManager
	}{}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			srv := newTestingServer(t, tt.manager)
			defer srv.Close()

			path := fmt.Sprintf("%s/resources/%s/acl", srv.URL, tt.instance)
			request, err := http.NewRequest(http.MethodPost, path, nil)
			assert.NoError(t, err)

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
		expectedCode int
		expectedBody string
		manager      rpaas.RpaasManager
	}{}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			srv := newTestingServer(t, tt.manager)
			defer srv.Close()

			path := fmt.Sprintf("%s/resources/%s/acl", srv.URL, tt.instance)
			request, err := http.NewRequest(http.MethodDelete, path, nil)
			assert.NoError(t, err)

			rsp, err := srv.Client().Do(request)
			assert.NoError(t, err)
			assert.Equal(t, tt.expectedCode, rsp.StatusCode)
			assert.Regexp(t, tt.expectedBody, bodyContent(rsp))
		})
	}
}
