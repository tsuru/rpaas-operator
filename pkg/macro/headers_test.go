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

func TestProxyPassWithHeaders(t *testing.T) {
	result, err := macro.Execute("proxy_pass_with_headers", nil, map[string]string{
		"destination": "http://example.com",
	})
	require.NoError(t, err)
	assert.Equal(t, `
proxy_set_header Connection        '';
proxy_set_header Host              ${server_name};
proxy_set_header X-Forwarded-For   ${proxy_add_x_forwarded_for};
proxy_set_header X-Forwarded-Host  ${server_name};
proxy_set_header X-Forwarded-Proto ${scheme};
proxy_set_header X-Real-IP         ${remote_addr};
proxy_set_header X-Request-Id      ${request_id_final};
proxy_set_header Early-Data        ${ssl_early_data};

proxy_pass http://example.com;`, result)

}

func TestProxyPassWithHeadersSimple(t *testing.T) {
	result, err := macro.Execute("proxy_pass_with_headers", []string{
		"http://example.com",
	}, nil)
	require.NoError(t, err)
	assert.Equal(t, `
proxy_set_header Connection        '';
proxy_set_header Host              ${server_name};
proxy_set_header X-Forwarded-For   ${proxy_add_x_forwarded_for};
proxy_set_header X-Forwarded-Host  ${server_name};
proxy_set_header X-Forwarded-Proto ${scheme};
proxy_set_header X-Real-IP         ${remote_addr};
proxy_set_header X-Request-Id      ${request_id_final};
proxy_set_header Early-Data        ${ssl_early_data};

proxy_pass http://example.com;`, result)

}

func TestProxyPassWithHeadersFullFeatured(t *testing.T) {
	result, err := macro.Execute("proxy_pass_with_headers", nil, map[string]string{
		"destination": "http://example.com",
		"headerHost":  "${host}",
		"geoip2":      "true",
	})
	require.NoError(t, err)
	assert.Equal(t, `
proxy_set_header Connection        '';
proxy_set_header Host              ${host};
proxy_set_header X-Forwarded-For   ${proxy_add_x_forwarded_for};
proxy_set_header X-Forwarded-Host  ${host};
proxy_set_header X-Forwarded-Proto ${scheme};
proxy_set_header X-Real-IP         ${remote_addr};
proxy_set_header X-Request-Id      ${request_id_final};
proxy_set_header Early-Data        ${ssl_early_data};

# GEOIP2 Headers
proxy_set_header X-Geoip-City-Database-Build              ${geoip2_metadata_city_build};
proxy_set_header X-Geoip-Country-Code                     ${geoip2_data_country_code};
proxy_set_header X-Geoip-Country-Name                     ${geoip2_data_country_name};
proxy_set_header X-Geoip-City-Name                        ${geoip2_data_city_name};
proxy_set_header X-Geoip-Region-Name                      ${geoip2_data_region_name};
proxy_set_header X-Geoip-Continent-Name                   ${geoip2_data_continent_name};
proxy_set_header X-Geoip-Latitude                         ${geoip2_data_latitude};
proxy_set_header X-Geoip-Longitude                        ${geoip2_data_longitude};
proxy_set_header X-Geoip-Location-Precision               ${geoip2_data_location_precision};
proxy_set_header X-Geoip-Postal-Code                      ${geoip2_data_postal_code};

proxy_set_header X-Geoip-Anonymous-Database-Build         ${geoip2_metadata_anonymous_build};
proxy_set_header X-Geoip-Is-Anonymous                     ${geoip2_data_is_anonymous};

proxy_set_header X-Geoip-Connection-Type-Database-Build   ${geoip2_metadata_connection_type_build};
proxy_set_header X-Geoip-Connection-Type                  ${geoip2_data_connection_type};

proxy_pass http://example.com;`, result)

}
