// Copyright 2019 tsuru authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package web

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"testing"

	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/tsuru/rpaas-operator/internal/pkg/rpaas"
	"github.com/tsuru/rpaas-operator/internal/pkg/rpaas/fake"
)

func Test_listExtraFiles(t *testing.T) {
	testCases := []struct {
		instance     string
		showContent  bool
		expectedCode int
		expected     []rpaas.File
		manager      rpaas.RpaasManager
	}{
		{
			instance:     "my-instance",
			expectedCode: http.StatusOK,
			expected:     nil,
			manager:      &fake.RpaasManager{},
		},
		{
			instance:     "my-instance",
			expectedCode: http.StatusOK,
			expected: []rpaas.File{
				{Name: "www/index.html"},
				{Name: "waf/ddos-rules.cnf"},
			},
			manager: &fake.RpaasManager{
				FakeGetExtraFiles: func(string) ([]rpaas.File, error) {
					return []rpaas.File{
						{Name: "www/index.html", Content: []byte("c1")},
						{Name: "waf/ddos-rules.cnf", Content: []byte("c1")},
					}, nil
				},
			},
		},
		{
			instance:     "my-instance",
			showContent:  true,
			expectedCode: http.StatusOK,
			expected: []rpaas.File{
				{Name: "www/index.html", Content: []byte("c1")},
				{Name: "waf/ddos-rules.cnf", Content: []byte("c2")},
			},
			manager: &fake.RpaasManager{
				FakeGetExtraFiles: func(string) ([]rpaas.File, error) {
					return []rpaas.File{
						{Name: "www/index.html", Content: []byte("c1")},
						{Name: "waf/ddos-rules.cnf", Content: []byte("c2")},
					}, nil
				},
			},
		},
		{
			instance:     "my-instance",
			expectedCode: http.StatusOK,
			expected:     []rpaas.File{},
			manager: &fake.RpaasManager{
				FakeGetExtraFiles: func(string) ([]rpaas.File, error) {
					return []rpaas.File{}, nil
				},
			},
		},
	}

	for _, tt := range testCases {
		t.Run("", func(t *testing.T) {
			srv := newTestingServer(t, tt.manager)
			defer srv.Close()
			path := fmt.Sprintf("%s/resources/%s/files?show-content=%s", srv.URL, tt.instance, strconv.FormatBool(tt.showContent))
			request, err := http.NewRequest(http.MethodGet, path, nil)
			require.NoError(t, err)
			rsp, err := srv.Client().Do(request)
			require.NoError(t, err)
			assert.Equal(t, tt.expectedCode, rsp.StatusCode)
			var gotFiles []rpaas.File
			err = json.Unmarshal([]byte(bodyContent(rsp)), &gotFiles)
			assert.NoError(t, err)
			assert.Equal(t, tt.expected, gotFiles)
		})
	}
}

func Test_getExtraFiles(t *testing.T) {
	testCases := []struct {
		instance     string
		filename     string
		expectedCode int
		expected     map[string]string
		manager      rpaas.RpaasManager
	}{
		{
			instance:     "my-instance",
			filename:     "www%2Fhtml%2Findex.html",
			expectedCode: http.StatusOK,
			expected: map[string]string{
				"name":    "www/html/index.html",
				"content": "PGgxPkhlbGxvIHdvcmxkPC9oMT4=",
				"sha256":  "ceaf61387be7b18784964bfee77424ab9a8e58e71476ee6283613aece598232e",
			},
			manager: &fake.RpaasManager{
				FakeGetExtraFiles: func(string) ([]rpaas.File, error) {
					return []rpaas.File{
						{
							Name:    "www/html/index.html",
							Content: []byte(`<h1>Hello world</h1>`),
						},
					}, nil
				},
			},
		},
		{
			instance:     "my-instance",
			filename:     "not-found-file.cnf",
			expectedCode: http.StatusNotFound,
			manager: &fake.RpaasManager{
				FakeGetExtraFiles: func(string) ([]rpaas.File, error) {
					return []rpaas.File{}, nil
				},
			},
		},
	}

	for _, tt := range testCases {
		t.Run("", func(t *testing.T) {
			srv := newTestingServer(t, tt.manager)
			defer srv.Close()
			path := fmt.Sprintf("%s/resources/%s/files/%s", srv.URL, tt.instance, tt.filename)
			request, err := http.NewRequest(http.MethodGet, path, nil)
			require.NoError(t, err)
			rsp, err := srv.Client().Do(request)
			require.NoError(t, err)
			assert.Equal(t, tt.expectedCode, rsp.StatusCode)
			if tt.expected != nil {
				var gotFile map[string]string
				err = json.Unmarshal([]byte(bodyContent(rsp)), &gotFile)
				assert.NoError(t, err)
				assert.Equal(t, tt.expected, gotFile)
			}
		})
	}
}

func Test_addExtraFiles(t *testing.T) {
	bodyWithNoFiles, err := newMultipartFormBody("files")
	require.NoError(t, err)

	bodyWithTwoFiles, err := newMultipartFormBody("files",
		multipartFile{
			filename: "my-waf-rules.conf",
			content:  "# some custom conf",
		},
		multipartFile{
			filename: "my-another-waf-rules.cnf",
			content:  "...",
		},
	)
	require.NoError(t, err)

	testCases := []struct {
		instance     string
		requestBody  string
		expectedCode int
		expectedBody string
		manager      rpaas.RpaasManager
	}{
		{
			instance:     "my-instance",
			requestBody:  "",
			expectedCode: http.StatusBadRequest,
			expectedBody: "multipart form files is not valid",
			manager:      &fake.RpaasManager{},
		},
		{
			instance:     "my-instance",
			requestBody:  bodyWithNoFiles,
			expectedCode: http.StatusBadRequest,
			expectedBody: `files form field is required`,
			manager:      &fake.RpaasManager{},
		},
		{
			instance:     "my-instance",
			requestBody:  bodyWithTwoFiles,
			expectedCode: http.StatusCreated,
			expectedBody: "New 2 files were added",
			manager:      &fake.RpaasManager{},
		},
	}

	for _, tt := range testCases {
		t.Run("", func(t *testing.T) {
			srv := newTestingServer(t, tt.manager)
			defer srv.Close()
			path := fmt.Sprintf("%s/resources/%s/files", srv.URL, tt.instance)
			request, err := http.NewRequest(http.MethodPost, path, strings.NewReader(tt.requestBody))
			require.NoError(t, err)
			request.Header.Set(echo.HeaderContentType, fmt.Sprintf(`%s; boundary=%s`, echo.MIMEMultipartForm, boundary))
			rsp, err := srv.Client().Do(request)
			require.NoError(t, err)
			assert.Equal(t, tt.expectedCode, rsp.StatusCode)
			assert.Regexp(t, tt.expectedBody, bodyContent(rsp))
		})
	}
}

func Test_updateExtraFiles(t *testing.T) {
	bodyWithNoFiles, err := newMultipartFormBody("files")
	require.NoError(t, err)

	bodyWithTwoFiles, err := newMultipartFormBody("files",
		multipartFile{
			filename: "my-waf-rules.conf",
			content:  "# some custom conf",
		},
		multipartFile{
			filename: "my-another-waf-rules.cnf",
			content:  "...",
		},
	)
	require.NoError(t, err)

	testCases := []struct {
		instance     string
		requestBody  string
		expectedCode int
		expectedBody string
		manager      rpaas.RpaasManager
	}{
		{
			instance:     "my-instance",
			requestBody:  "",
			expectedCode: http.StatusBadRequest,
			expectedBody: "multipart form files is not valid",
			manager:      &fake.RpaasManager{},
		},
		{
			instance:     "my-instance",
			requestBody:  bodyWithNoFiles,
			expectedCode: http.StatusBadRequest,
			expectedBody: `files form field is required`,
			manager:      &fake.RpaasManager{},
		},
		{
			instance:     "my-instance",
			requestBody:  bodyWithTwoFiles,
			expectedCode: http.StatusOK,
			expectedBody: "2 files were successfully updated",
			manager:      &fake.RpaasManager{},
		},
	}

	for _, tt := range testCases {
		t.Run("", func(t *testing.T) {
			srv := newTestingServer(t, tt.manager)
			defer srv.Close()
			path := fmt.Sprintf("%s/resources/%s/files", srv.URL, tt.instance)
			request, err := http.NewRequest(http.MethodPut, path, strings.NewReader(tt.requestBody))
			require.NoError(t, err)
			request.Header.Set(echo.HeaderContentType, fmt.Sprintf(`%s; boundary=%s`, echo.MIMEMultipartForm, boundary))
			rsp, err := srv.Client().Do(request)
			require.NoError(t, err)
			assert.Equal(t, tt.expectedCode, rsp.StatusCode)
			assert.Regexp(t, tt.expectedBody, bodyContent(rsp))
		})
	}
}

func Test_deleteExtraFiles(t *testing.T) {
	testCases := []struct {
		instance     string
		files        []string
		expectedCode int
		expectedBody string
		manager      rpaas.RpaasManager
	}{
		{
			instance:     "my-instance",
			files:        []string{"waf%2Fsqli-rules.cnf"},
			expectedCode: http.StatusOK,
			expectedBody: `file(s) waf/sqli-rules.cnf removed`,
			manager:      &fake.RpaasManager{},
		},
		{
			instance:     "my-instance",
			files:        []string{"not-found.cnf"},
			expectedCode: http.StatusNotFound,
			expectedBody: "{\"Msg\":\"not found\"}",
			manager: &fake.RpaasManager{
				FakeDeleteExtraFiles: func(string, ...string) error {
					return &rpaas.NotFoundError{
						Msg: "not found",
					}
				},
			},
		},
	}

	for _, tt := range testCases {
		t.Run("", func(t *testing.T) {
			srv := newTestingServer(t, tt.manager)
			defer srv.Close()
			b, err := json.Marshal(tt.files)
			assert.NoError(t, err)
			body := bytes.NewReader(b)
			path := fmt.Sprintf("%s/resources/%s/files", srv.URL, tt.instance)
			request, err := http.NewRequest(http.MethodDelete, path, body)
			request.Header.Add("Content-Type", "application/json")
			require.NoError(t, err)
			rsp, err := srv.Client().Do(request)
			require.NoError(t, err)
			assert.Equal(t, tt.expectedCode, rsp.StatusCode)
			assert.EqualValues(t, tt.expectedBody, bodyContent(rsp))
		})
	}
}
