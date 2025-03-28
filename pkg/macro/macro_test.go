// Copyright 2025 tsuru authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package macro_test

import (
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/tsuru/rpaas-operator/pkg/macro"
)

func TestExpand(t *testing.T) {
	output, err := macro.Expand(
		`
SECURITY_HEADERS;
location / {
	CORS origins="facebook.com,google.com";
	#CORS origins=facebook.com,google.com;
	blah do="something";
	PROXY_PASS_WITH_HEADERS http://app1;
}`)
	require.NoError(t, err)
	require.Equal(t, `
more_set_headers 'X-Content-Type-Options: nosniff';
more_set_headers 'X-XSS-Protection: 1; mode=block';
more_set_headers 'X-Frame-Options: SAMEORIGIN';
more_set_headers 'Content-Security-Policy: upgrade-insecure-requests';
more_set_headers 'Strict-Transport-Security: max-age=31536000; includeSubDomains';

location / {
	if ($http_origin ~* ((facebook\.com)|(google\.com))$ ) { set $cors 'true'; }

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

	#CORS origins=facebook.com,google.com;
	blah do="something";

	proxy_set_header Connection        '';
	proxy_set_header Host              ${server_name};
	proxy_set_header X-Forwarded-For   ${proxy_add_x_forwarded_for};
	proxy_set_header X-Forwarded-Host  ${server_name};
	proxy_set_header X-Forwarded-Proto ${scheme};
	proxy_set_header X-Real-IP         ${remote_addr};
	proxy_set_header X-Request-Id      ${request_id_final};
	proxy_set_header Early-Data        ${ssl_early_data};

	proxy_pass http://app1;
}
`, output)
}
