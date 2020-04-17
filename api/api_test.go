// Copyright 2020 tsuru authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package api

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRequestBinder_Bind(t *testing.T) {
	type t1 struct {
		Name    string                 `form:"name"`
		Tags    []string               `form:"tags"`
		Complex map[string]interface{} `form:"complex"`
		Ignored bool                   `form:"-"`
	}

	tests := []struct {
		name   string
		c      echo.Context
		data   interface{}
		assert func(*testing.T, error, interface{}, echo.Context)
	}{
		{
			name: "when content-type is not application/x-www-form-urleconded",
			c: func() echo.Context {
				body := `{"name": "my-instance"}`
				e := newEcho()
				req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(body))
				req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
				return e.NewContext(req, httptest.NewRecorder())
			}(),
			assert: func(t *testing.T, err error, d interface{}, c echo.Context) {
				require.Error(t, err)
				assert.EqualError(t, err, "code=415, message=Unsupported Media Type, internal=<nil>")
				assert.Equal(t, "application/x-www-form-urlencoded", c.Response().Header().Get("Accept"))
			},
		},
		{
			name: "submitting a complex object",
			c: func() echo.Context {
				body := `name=my-instance&tags.0=tag1&tags.1=tag2&tags.2=tag3&ignored=true&complex.key1=1&complex.other.key=value`
				e := newEcho()
				req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(body))
				req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationForm)
				return e.NewContext(req, httptest.NewRecorder())
			}(),
			data: &t1{},
			assert: func(t *testing.T, err error, d interface{}, c echo.Context) {
				require.NoError(t, err)
				assert.Equal(t, &t1{
					Name: "my-instance",
					Tags: []string{"tag1", "tag2", "tag3"},
					Complex: map[string]interface{}{
						"key1": "1",
						"other": map[string]interface{}{
							"key": "value",
						},
					},
				}, d)
			},
		},
		{
			name: "when some error occurs on decode method",
			c: func() echo.Context {
				body := `name=my-instance&tags.0=tag1&tags.1=tag2&tags.2=tag3&ignored=true&complex.key1=1&complex.other.key=value`
				e := newEcho()
				req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(body))
				req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationForm)
				return e.NewContext(req, httptest.NewRecorder())
			}(),
			data: func() string { return "cannot decode a function" },
			assert: func(t *testing.T, err error, d interface{}, c echo.Context) {
				require.Error(t, err)
				assert.EqualError(t, err, "cannot decode the parameters: func() string has unsupported kind func")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.NotNil(t, tt.assert)
			b := &requestBinder{}
			err := b.Bind(tt.data, tt.c)
			tt.assert(t, err, tt.data, tt.c)
		})
	}
}
