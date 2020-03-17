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
	"github.com/tsuru/rpaas-operator/internal/pkg/rpaas"
	"github.com/tsuru/rpaas-operator/internal/pkg/rpaas/fake"
)

func Test_deleteBlock(t *testing.T) {
	tests := []struct {
		name         string
		instance     string
		block        string
		expectedCode int
		expectedBody string
		manager      rpaas.RpaasManager
	}{
		{
			name:         "when successfully deleting a instance block",
			instance:     "my-instance",
			block:        "root",
			expectedCode: http.StatusOK,
			manager: &fake.RpaasManager{
				FakeDeleteBlock: func(instance, block string) error {
					assert.Equal(t, "my-instance", instance)
					assert.Equal(t, "root", block)
					return nil
				},
			},
		},
		{
			name:         "when the manager returns an error",
			instance:     "my-instance",
			block:        "http",
			expectedCode: http.StatusNotFound,
			expectedBody: `block \\"http\\" not found`,
			manager: &fake.RpaasManager{
				FakeDeleteBlock: func(instance, block string) error {
					return rpaas.NotFoundError{Msg: "block \"http\" not found"}
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			srv := newTestingServer(t, tt.manager)
			defer srv.Close()
			path := fmt.Sprintf("%s/resources/%s/block/%s", srv.URL, tt.instance, tt.block)
			request, err := http.NewRequest(http.MethodDelete, path, nil)
			assert.NoError(t, err)
			rsp, err := srv.Client().Do(request)
			assert.NoError(t, err)
			assert.Equal(t, tt.expectedCode, rsp.StatusCode)
			assert.Regexp(t, tt.expectedBody, bodyContent(rsp))
		})
	}
}

func Test_listBlocks(t *testing.T) {
	type listBlocksResponse struct {
		Blocks []rpaas.ConfigurationBlock `json:"blocks"`
	}

	tests := []struct {
		name           string
		instance       string
		expectedCode   int
		expectedBlocks listBlocksResponse
		manager        rpaas.RpaasManager
	}{
		{
			name:         "when instance has no blocks (nil blocks)",
			instance:     "my-instance",
			expectedCode: http.StatusOK,
			expectedBlocks: listBlocksResponse{
				Blocks: make([]rpaas.ConfigurationBlock, 0),
			},
			manager: &fake.RpaasManager{
				FakeListBlocks: func(instance string) ([]rpaas.ConfigurationBlock, error) {
					assert.Equal(t, "my-instance", instance)
					return nil, nil
				},
			},
		},
		{
			name:         "when successfully listing instances blocks",
			instance:     "another-instance",
			expectedCode: http.StatusOK,
			expectedBlocks: listBlocksResponse{
				Blocks: []rpaas.ConfigurationBlock{
					{Name: "http", Content: "# my nginx configuration"},
					{Name: "root", Content: "events {\nworker_connections 8192;\n}"},
				},
			},
			manager: &fake.RpaasManager{
				FakeListBlocks: func(instance string) ([]rpaas.ConfigurationBlock, error) {
					assert.Equal(t, "another-instance", instance)
					return []rpaas.ConfigurationBlock{
						{Name: "http", Content: "# my nginx configuration"},
						{Name: "root", Content: "events {\nworker_connections 8192;\n}"},
					}, nil
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			srv := newTestingServer(t, tt.manager)
			defer srv.Close()
			path := fmt.Sprintf("%s/resources/%s/block", srv.URL, tt.instance)
			request, err := http.NewRequest(http.MethodGet, path, nil)
			assert.NoError(t, err)
			rsp, err := srv.Client().Do(request)
			assert.NoError(t, err)
			assert.Equal(t, tt.expectedCode, rsp.StatusCode)
			var got listBlocksResponse
			err = json.Unmarshal([]byte(bodyContent(rsp)), &got)
			assert.NoError(t, err)
			assert.Equal(t, got, tt.expectedBlocks)
		})
	}
}

func Test_updateBlock(t *testing.T) {
	tests := []struct {
		name         string
		instance     string
		requestBody  string
		expectedCode int
		expectedBody string
		manager      rpaas.RpaasManager
	}{
		{
			name:         "when a request has no body message",
			instance:     "my-instance",
			expectedCode: http.StatusBadRequest,
			expectedBody: "{\"message\":\"Request body can't be empty\"}",
			manager:      &fake.RpaasManager{},
		},
		{
			name:         "when the manager returns an error",
			instance:     "my-instance",
			requestBody:  "block_name=invalid-block&content=some%20content",
			expectedCode: http.StatusBadRequest,
			expectedBody: "some error",
			manager: &fake.RpaasManager{
				FakeUpdateBlock: func(instance string, block rpaas.ConfigurationBlock) error {
					assert.Equal(t, instance, "my-instance")
					assert.Equal(t, block, rpaas.ConfigurationBlock{Name: "invalid-block", Content: "some content"})
					return rpaas.ValidationError{Msg: "some error"}
				},
			},
		},
		{
			name:         "when a block is successfully created/updated",
			instance:     "my-instance",
			requestBody:  "block_name=server&content=%23%20My%20nginx%20custom%20conf",
			expectedCode: http.StatusOK,
			manager: &fake.RpaasManager{
				FakeUpdateBlock: func(instance string, block rpaas.ConfigurationBlock) error {
					assert.Equal(t, instance, "my-instance")
					assert.Equal(t, block, rpaas.ConfigurationBlock{Name: "server", Content: "# My nginx custom conf"})
					return nil
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			srv := newTestingServer(t, tt.manager)
			defer srv.Close()
			path := fmt.Sprintf("%s/resources/%s/block", srv.URL, tt.instance)
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
