// Copyright 2025 tsuru authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package macro_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/tsuru/rpaas-operator/pkg/macro"
)

func TestExecuteCorsEmpty(t *testing.T) {
	_, err := macro.Execute("cors", nil, map[string]string{})
	assert.Error(t, err)
	assert.Equal(t, "missing required argument \"origins\"", err.Error())
}

func TestExecuteCorsWithInvalidMaxAge(t *testing.T) {
	_, err := macro.Execute("cors", nil, map[string]string{
		"origins": "testing.com",
		"maxAge":  "invalid",
	})
	assert.Error(t, err)
	assert.Equal(t, "argument \"maxAge\" must be an integer", err.Error())
}

func TestExecuteCorsWithInvalidParam(t *testing.T) {
	_, err := macro.Execute("cors", nil, map[string]string{
		"origins": "testing.com",
		"maxAgee": "invalid",
	})
	assert.Error(t, err)
	assert.Equal(t, "unknown argument \"maxAgee\"", err.Error())
}

func TestExecuteCorsBasic(t *testing.T) {
	result, err := macro.Execute("cors", nil, map[string]string{
		"origins": "http://example.com",
	})
	assert.NoError(t, err)
	assert.Equal(t, `if ($http_origin ~* ((http://example\.com))$ ) { set $cors 'true'; }

if ($request_method = 'OPTIONS') {
	set $cors ${cors}options;
}

if ($cors = "true") {
    more_set_headers 'Access-Control-Allow-Origin: $http_origin';
    more_set_headers 'Access-Control-Allow-Credentials: true';
    more_set_headers 'Access-Control-Allow-Methods: GET, POST, PUT, DELETE, OPTIONS';
    more_set_headers 'Access-Control-Allow-Headers: Content-Type, Authorization';
    more_set_headers 'Access-Control-Max-Age: 3600';
}

if ($cors = "trueoptions") {
    more_set_headers 'Access-Control-Allow-Origin: $http_origin';
    more_set_headers 'Access-Control-Allow-Credentials: true';
    more_set_headers 'Access-Control-Allow-Methods: GET, POST, PUT, DELETE, OPTIONS';
    more_set_headers 'Access-Control-Allow-Headers: Content-Type, Authorization';
    more_set_headers 'Access-Control-Max-Age: 3600';
    more_set_headers 'Content-Type: text/plain charset=UTF-8';
    more_set_headers 'Content-Length: 0';
    return 200;
}
`, result)
}

func TestExecuteCorsWithArg(t *testing.T) {
	result, err := macro.Execute("cors", []string{
		"http://example.com",
	}, nil)
	assert.NoError(t, err)
	assert.Equal(t, `if ($http_origin ~* ((http://example\.com))$ ) { set $cors 'true'; }

if ($request_method = 'OPTIONS') {
	set $cors ${cors}options;
}

if ($cors = "true") {
    more_set_headers 'Access-Control-Allow-Origin: $http_origin';
    more_set_headers 'Access-Control-Allow-Credentials: true';
    more_set_headers 'Access-Control-Allow-Methods: GET, POST, PUT, DELETE, OPTIONS';
    more_set_headers 'Access-Control-Allow-Headers: Content-Type, Authorization';
    more_set_headers 'Access-Control-Max-Age: 3600';
}

if ($cors = "trueoptions") {
    more_set_headers 'Access-Control-Allow-Origin: $http_origin';
    more_set_headers 'Access-Control-Allow-Credentials: true';
    more_set_headers 'Access-Control-Allow-Methods: GET, POST, PUT, DELETE, OPTIONS';
    more_set_headers 'Access-Control-Allow-Headers: Content-Type, Authorization';
    more_set_headers 'Access-Control-Max-Age: 3600';
    more_set_headers 'Content-Type: text/plain charset=UTF-8';
    more_set_headers 'Content-Length: 0';
    return 200;
}
`, result)
}

func TestExecuteCorsWildcard(t *testing.T) {
	result, err := macro.Execute("cors", nil, map[string]string{
		"origins": "*",
	})
	assert.NoError(t, err)
	assert.Equal(t, `set $http_origin *;
set $cors 'true';

if ($request_method = 'OPTIONS') {
	set $cors ${cors}options;
}

if ($cors = "true") {
    more_set_headers 'Access-Control-Allow-Origin: $http_origin';
    more_set_headers 'Access-Control-Allow-Credentials: true';
    more_set_headers 'Access-Control-Allow-Methods: GET, POST, PUT, DELETE, OPTIONS';
    more_set_headers 'Access-Control-Allow-Headers: Content-Type, Authorization';
    more_set_headers 'Access-Control-Max-Age: 3600';
}

if ($cors = "trueoptions") {
    more_set_headers 'Access-Control-Allow-Origin: $http_origin';
    more_set_headers 'Access-Control-Allow-Credentials: true';
    more_set_headers 'Access-Control-Allow-Methods: GET, POST, PUT, DELETE, OPTIONS';
    more_set_headers 'Access-Control-Allow-Headers: Content-Type, Authorization';
    more_set_headers 'Access-Control-Max-Age: 3600';
    more_set_headers 'Content-Type: text/plain charset=UTF-8';
    more_set_headers 'Content-Length: 0';
    return 200;
}
`, result)
}

func TestExecuteCorsFullFeatured(t *testing.T) {
	result, err := macro.Execute("cors", nil, map[string]string{
		"origins":           "http://example.com, https://facebook.com, http://*.example.com",
		"allowMethods":      "GET, POST, PUT, PATCH",
		"allowHeaders":      "Content-Type, Authorization, X-Custom-Header",
		"maxAge":            "7200",
		"optionsStatusCode": "204",
		"allowCredentials":  "false",
	})
	assert.NoError(t, err)
	assert.Equal(t,
		`if ($http_origin ~* ((http://example\.com)|(https://facebook\.com)|(http://[A-Za-z0-9\-]+\.example\.com))$ ) { set $cors 'true'; }

if ($request_method = 'OPTIONS') {
	set $cors ${cors}options;
}

if ($cors = "true") {
    more_set_headers 'Access-Control-Allow-Origin: $http_origin';
    more_set_headers 'Access-Control-Allow-Methods: GET, POST, PUT, PATCH';
    more_set_headers 'Access-Control-Allow-Headers: Content-Type, Authorization, X-Custom-Header';
    more_set_headers 'Access-Control-Max-Age: 7200';
}

if ($cors = "trueoptions") {
    more_set_headers 'Access-Control-Allow-Origin: $http_origin';
    more_set_headers 'Access-Control-Allow-Methods: GET, POST, PUT, PATCH';
    more_set_headers 'Access-Control-Allow-Headers: Content-Type, Authorization, X-Custom-Header';
    more_set_headers 'Access-Control-Max-Age: 7200';
    more_set_headers 'Content-Type: text/plain charset=UTF-8';
    more_set_headers 'Content-Length: 0';
    return 204;
}
`, result)
}
