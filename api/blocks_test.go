package api

import (
	"errors"
	"fmt"
	"net/http"
	"strings"
	"testing"

	"github.com/labstack/echo"
	"github.com/stretchr/testify/assert"
	"github.com/tsuru/rpaas-operator/rpaas"
	"github.com/tsuru/rpaas-operator/rpaas/fake"
)

func Test_deleteBlock(t *testing.T) {
	instanceName := "my-instance"
	blockName := "http"

	testCases := []struct {
		expectedCode int
		expectedBody string
		manager      rpaas.RpaasManager
	}{
		{
			expectedCode: http.StatusOK,
			expectedBody: `block "http" was successfully removed`,
			manager:      &fake.RpaasManager{},
		},
		{
			expectedCode: http.StatusBadRequest,
			expectedBody: "rpaas: block is not valid (acceptable values are: [root http server])",
			manager: &fake.RpaasManager{
				FakeDeleteBlock: func(i, b string) error {
					return rpaas.ErrBlockInvalid
				},
			},
		},
		{
			expectedCode: http.StatusNoContent,
			expectedBody: "",
			manager: &fake.RpaasManager{
				FakeDeleteBlock: func(i, b string) error {
					return rpaas.ErrBlockIsNotDefined
				},
			},
		},
		{
			expectedCode: http.StatusInternalServerError,
			expectedBody: fmt.Sprintf("{\"message\":\"Internal Server Error\"}\n"),
			manager: &fake.RpaasManager{
				FakeDeleteBlock: func(i, b string) error {
					return errors.New("some error")
				},
			},
		},
	}

	for _, tt := range testCases {
		t.Run("", func(t *testing.T) {
			srv := newTestingServer(t, tt.manager)
			defer srv.Close()
			path := fmt.Sprintf("%s/resources/%s/block/%s", srv.URL, instanceName, blockName)
			request, err := http.NewRequest(http.MethodDelete, path, nil)
			assert.NoError(t, err)
			rsp, err := srv.Client().Do(request)
			assert.NoError(t, err)
			assert.Equal(t, tt.expectedCode, rsp.StatusCode)
			assert.Equal(t, tt.expectedBody, bodyContent(rsp))
		})
	}
}

func Test_listBlocks(t *testing.T) {
	testCases := []struct {
		expectedCode  int
		expectedBody  string
		expectedError error
		manager       rpaas.RpaasManager
	}{
		{
			http.StatusOK,
			fmt.Sprintf("{\"blocks\":[]}\n"),
			nil,
			&fake.RpaasManager{
				FakeListBlocks: func(i string) ([]rpaas.ConfigurationBlock, error) {
					return []rpaas.ConfigurationBlock{}, nil
				},
			},
		},
		{
			http.StatusOK,
			fmt.Sprintf("{\"blocks\":[{\"block_name\":\"http\",\"content\":\"# my nginx configuration\"}]}\n"),
			nil,
			&fake.RpaasManager{
				FakeListBlocks: func(i string) ([]rpaas.ConfigurationBlock, error) {
					return []rpaas.ConfigurationBlock{{Name: "http", Content: "# my nginx configuration"}}, nil
				},
			},
		},
		{
			http.StatusInternalServerError,
			fmt.Sprintf("{\"message\":\"Internal Server Error\"}\n"),
			errors.New("some error"),
			&fake.RpaasManager{
				FakeListBlocks: func(i string) ([]rpaas.ConfigurationBlock, error) {
					return nil, errors.New("some error")
				},
			},
		},
	}

	for _, tt := range testCases {
		t.Run("", func(t *testing.T) {
			srv := newTestingServer(t, tt.manager)
			defer srv.Close()
			path := fmt.Sprintf("%s/resources/%s/block", srv.URL, "my-instance")
			request, err := http.NewRequest(http.MethodGet, path, nil)
			assert.NoError(t, err)
			rsp, err := srv.Client().Do(request)
			assert.NoError(t, err)
			assert.Equal(t, tt.expectedCode, rsp.StatusCode)
			assert.Equal(t, tt.expectedBody, bodyContent(rsp))
		})
	}
}

func Test_updateBlock(t *testing.T) {
	testCases := []struct {
		requestBody  string
		expectedCode int
		expectedBody string
		manager      rpaas.RpaasManager
	}{
		{
			requestBody:  "",
			expectedCode: http.StatusBadRequest,
			expectedBody: fmt.Sprintf("{\"message\":\"Request body can't be empty\"}\n"),
			manager:      &fake.RpaasManager{},
		},
		{
			requestBody:  "block_name=invalid-block&content=",
			expectedCode: http.StatusBadRequest,
			expectedBody: `rpaas: block is not valid (acceptable values are: [root http server])`,
			manager: &fake.RpaasManager{
				FakeUpdateBlock: func(i string, b rpaas.ConfigurationBlock) error {
					return rpaas.ErrBlockInvalid
				},
			},
		},
		{
			requestBody:  "block_name=server&content=%23%20My%20nginx%20custom%20conf",
			expectedCode: http.StatusCreated,
			expectedBody: "",
			manager:      &fake.RpaasManager{},
		},
		{
			requestBody:  "block_name=server&content=%23%20My%20nginx%20custom%20conf",
			expectedCode: http.StatusInternalServerError,
			expectedBody: fmt.Sprintf("{\"message\":\"Internal Server Error\"}\n"),
			manager: &fake.RpaasManager{
				FakeUpdateBlock: func(i string, b rpaas.ConfigurationBlock) error {
					return errors.New("just another error")
				},
			},
		},
	}

	for _, tt := range testCases {
		t.Run("", func(t *testing.T) {
			srv := newTestingServer(t, tt.manager)
			defer srv.Close()
			path := fmt.Sprintf("%s/resources/%s/block", srv.URL, "my-instance")
			request, err := http.NewRequest(http.MethodPost, path, strings.NewReader(tt.requestBody))
			assert.NoError(t, err)
			request.Header.Set(echo.HeaderContentType, echo.MIMEApplicationForm)
			rsp, err := srv.Client().Do(request)
			assert.NoError(t, err)
			assert.Equal(t, tt.expectedCode, rsp.StatusCode)
			assert.Equal(t, tt.expectedBody, bodyContent(rsp))
		})
	}
}
