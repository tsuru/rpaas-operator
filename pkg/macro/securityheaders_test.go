// Copyright 2025 tsuru authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package macro_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/tsuru/rpaas-operator/pkg/macro"
)

func TestSecurityHeadersBasic(t *testing.T) {
	result, err := macro.Execute("security_headers", map[string]string{})
	require.NoError(t, err)
	assert.Equal(t, `more_set_headers 'X-Content-Type-Options: nosniff';
more_set_headers 'X-XSS-Protection: 1; mode=block';
more_set_headers 'X-Frame-Options: SAMEORIGIN';
more_set_headers 'Content-Security-Policy: upgrade-insecure-requests';
more_set_headers 'Strict-Transport-Security: max-age=31536000; includeSubDomains';
`, result)

}

func TestSecurityHeadersDisableForceHTTP(t *testing.T) {
	result, err := macro.Execute("security_headers", map[string]string{
		"forceHTTPS": "false",
	})
	require.NoError(t, err)
	assert.Equal(t, `more_set_headers 'X-Content-Type-Options: nosniff';
more_set_headers 'X-XSS-Protection: 1; mode=block';
more_set_headers 'X-Frame-Options: SAMEORIGIN';
more_set_headers 'Content-Security-Policy: upgrade-insecure-requests';
`, result)

}

func TestSecurityDenyFrameOptions(t *testing.T) {
	result, err := macro.Execute("security_headers", map[string]string{
		"frameOptions": "DENY",
	})
	require.NoError(t, err)
	assert.Equal(t, `more_set_headers 'X-Content-Type-Options: nosniff';
more_set_headers 'X-XSS-Protection: 1; mode=block';
more_set_headers 'X-Frame-Options: DENY';
more_set_headers 'Content-Security-Policy: upgrade-insecure-requests';
more_set_headers 'Strict-Transport-Security: max-age=31536000; includeSubDomains';
`, result)

}

func TestSecurityNoop(t *testing.T) {
	result, err := macro.Execute("security_headers", map[string]string{
		"noSniffContentType":         "false",
		"xssProtection":              "false",
		"frameOptions":               "NONE",
		"upgradeInsecureRequests":    "false",
		"forceHTTPS":                 "false",
		"forceHTTPMaxAge":            "0",
		"forceHTTPIncludeSubDomains": "false",
	})
	require.NoError(t, err)
	assert.Equal(t, "", result)

}
