// Copyright 2025 tsuru authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package macro

import (
	"text/template"

	"k8s.io/utils/ptr"
)

var proxyPassWithHeaders = Macro{
	Name: "proxy_pass_with_headers",
	Args: []MacroArg{
		{Name: "destination", Required: true, Type: MacroArgTypeString, Pos: ptr.To(0)},
		{Name: "headerHost", Default: "${server_name}", Required: false, Type: MacroArgTypeString},
		{Name: "geoip2", Default: "false", Required: false, Type: MacroArgTypeBool},
	},
	Template: `
proxy_set_header Connection        '';
proxy_set_header Host              {{ .HeaderHost }};
proxy_set_header X-Forwarded-For   ${proxy_add_x_forwarded_for};
proxy_set_header X-Forwarded-Host  {{ .HeaderHost }};
proxy_set_header X-Forwarded-Proto ${scheme};
proxy_set_header X-Real-IP         ${remote_addr};
proxy_set_header X-Request-Id      ${request_id_final};
proxy_set_header Early-Data        ${ssl_early_data};
{{- if .Geoip2 }}
{{ geoip2_headers }}
{{- end }}

proxy_pass {{ .Destination }};`,
	TemplateFuncMap: template.FuncMap{
		"geoip2_headers": func() string {
			return geoIP2Headers.Template
		},
	},
}

var geoIP2Headers = Macro{
	Name: "geoip2_headers",
	Args: []MacroArg{},
	Template: `
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
proxy_set_header X-Geoip-Connection-Type                  ${geoip2_data_connection_type};`,
}
