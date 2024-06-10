// Copyright 2024 tsuru authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package web

import (
	"fmt"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/tsuru/rpaas-operator/internal/pkg/rpaas"
	"github.com/tsuru/rpaas-operator/internal/pkg/rpaas/fake"
	clientTypes "github.com/tsuru/rpaas-operator/pkg/rpaas/client/types"
)

func Test_getMetadata(t *testing.T) {
	testCases := []struct {
		name         string
		instance     string
		expectedCode int
		manager      rpaas.RpaasManager
	}{
		{
			name:         "when successfully getting metadata",
			instance:     "my-instance",
			expectedCode: http.StatusOK,
			manager: &fake.RpaasManager{
				FakeGetMetadata: func(instance string) (*clientTypes.Metadata, error) {
					assert.Equal(t, "my-instance", instance)
					return &clientTypes.Metadata{
						Labels: []clientTypes.MetadataItem{
							{Name: "rpaas_instance", Value: "my-instance"},
						},
						Annotations: []clientTypes.MetadataItem{
							{Name: "custom-annotation", Value: "my-annotation"},
						},
					}, nil
				},
			},
		},
		{
			name:         "when get metadata returns an error",
			instance:     "my-instance",
			expectedCode: http.StatusNotFound,
			manager: &fake.RpaasManager{
				FakeGetMetadata: func(instance string) (*clientTypes.Metadata, error) {
					return nil, rpaas.NotFoundError{Msg: "instance not found"}
				},
			},
		},
	}

	for _, tt := range testCases {
		t.Run(tt.name, func(t *testing.T) {
			srv := newTestingServer(t, tt.manager)
			defer srv.Close()

			path := fmt.Sprintf("%s/resources/%s/metadata", srv.URL, tt.instance)

			req, err := http.NewRequest(http.MethodGet, path, nil)
			assert.NoError(t, err)

			rsp, err := srv.Client().Do(req)
			assert.NoError(t, err)

			assert.Equal(t, tt.expectedCode, rsp.StatusCode)
		})
	}
}

func Test_setMetadata(t *testing.T) {
	testCases := []struct {
		name         string
		instance     string
		expectedCode int
		manager      rpaas.RpaasManager
	}{
		{
			name:         "when successfully setting metadata",
			instance:     "my-instance",
			expectedCode: http.StatusOK,
			manager: &fake.RpaasManager{
				FakeSetMetadata: func(instance string, metadata *clientTypes.Metadata) error {
					return nil
				},
			},
		},
		{
			name:         "when set metadata instance not found",
			instance:     "my-instance",
			expectedCode: http.StatusNotFound,
			manager: &fake.RpaasManager{
				FakeSetMetadata: func(instance string, metadata *clientTypes.Metadata) error {
					return rpaas.NotFoundError{Msg: "instance not found"}
				},
			},
		},
		{
			name:         "when set metadata returns an error",
			instance:     "my-instance",
			expectedCode: http.StatusBadRequest,
			manager: &fake.RpaasManager{
				FakeSetMetadata: func(instance string, metadata *clientTypes.Metadata) error {
					return rpaas.ValidationError{Msg: "invalid metadata"}
				},
			},
		},
	}

	for _, tt := range testCases {
		t.Run(tt.name, func(t *testing.T) {
			srv := newTestingServer(t, tt.manager)
			defer srv.Close()

			path := fmt.Sprintf("%s/resources/%s/metadata", srv.URL, tt.instance)

			req, err := http.NewRequest(http.MethodPost, path, nil)
			assert.NoError(t, err)

			req.Header.Set("Content-Type", "application/json")

			rsp, err := srv.Client().Do(req)
			assert.NoError(t, err)

			assert.Equal(t, tt.expectedCode, rsp.StatusCode)
		})
	}
}

func Test_unsetMetadata(t *testing.T) {
	testCases := []struct {
		name         string
		instance     string
		expectedCode int
		manager      rpaas.RpaasManager
	}{
		{
			name:         "when successfully unsetting metadata",
			instance:     "my-instance",
			expectedCode: http.StatusOK,
			manager: &fake.RpaasManager{
				FakeUnsetMetadata: func(instance string, metadata *clientTypes.Metadata) error {
					return nil
				},
			},
		},
		{
			name:         "when unset metadata instance not found",
			instance:     "my-instance",
			expectedCode: http.StatusNotFound,
			manager: &fake.RpaasManager{
				FakeUnsetMetadata: func(instance string, metadata *clientTypes.Metadata) error {
					return rpaas.NotFoundError{Msg: "instance not found"}
				},
			},
		},
		{
			name:         "when unset metadata returns an error",
			instance:     "my-instance",
			expectedCode: http.StatusBadRequest,
			manager: &fake.RpaasManager{
				FakeUnsetMetadata: func(instance string, metadata *clientTypes.Metadata) error {
					return rpaas.ValidationError{Msg: "invalid metadata"}
				},
			},
		},
	}

	for _, tt := range testCases {
		t.Run(tt.name, func(t *testing.T) {
			srv := newTestingServer(t, tt.manager)
			defer srv.Close()

			path := fmt.Sprintf("%s/resources/%s/metadata", srv.URL, tt.instance)

			req, err := http.NewRequest(http.MethodDelete, path, nil)
			assert.NoError(t, err)

			req.Header.Set("Content-Type", "application/json")

			rsp, err := srv.Client().Do(req)
			assert.NoError(t, err)

			assert.Equal(t, tt.expectedCode, rsp.StatusCode)
		})
	}
}
