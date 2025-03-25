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
	_, err := macro.Execute("cors", map[string]string{})
	assert.Error(t, err)
	assert.Equal(t, "missing required argument \"origins\"", err.Error())
}

func TestExecuteCorsBasic(t *testing.T) {
	result, err := macro.Execute("cors", map[string]string{
		"origins": "http://example.com",
	})
	assert.NoError(t, err)
	assert.Equal(t, `

if ($http_origin ~* ((http://example\.com))$ ) { set $cors 'true'; }

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
