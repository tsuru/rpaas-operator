// Copyright 2025 tsuru authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package macro

import (
	"fmt"
	"regexp"
	"strings"
	"text/template"
)

// cors macro inspired by:
// https://pkg.go.dev/k8s.io/ingress-nginx/internal/ingress/annotations/cors#Config
// https://github.com/alibaba/tengine-ingress/blob/79c30697fe844e798f70b17c9b1bd7212806da68/rootfs/etc/nginx/template/nginx.tmpl#L836
var corsMacro = Macro{
	Name: "cors",
	Args: []MacroArg{
		{Name: "origins", Required: true, Type: MacroArgTypeString},
		{Name: "allowMethods", Default: "GET, POST, PUT, DELETE, OPTIONS", Required: false, Type: MacroArgTypeString},
		{Name: "allowHeaders", Default: "Content-Type, Authorization", Required: false, Type: MacroArgTypeString},
		{Name: "allowCredentials", Default: "true", Required: false, Type: MacroArgTypeBool},
		{Name: "exposeHeaders", Required: false, Type: MacroArgTypeString},
		{Name: "maxAge", Default: "3600", Required: false, Type: MacroArgTypeInt},
		{Name: "optionsStatusCode", Default: "200", Required: false, Type: MacroArgTypeInt},
	},

	Template: `
{{ if .Origins }}
{{ buildCorsOriginRegex .Origins }}
{{ end }}
if ($request_method = 'OPTIONS') {
	set $cors ${cors}options;
}

if ($cors = "true") {
    more_set_headers 'Access-Control-Allow-Origin: $http_origin';
    {{ if .AllowCredentials }}more_set_headers 'Access-Control-Allow-Credentials: {{ .AllowCredentials }}';{{ end }}
    more_set_headers 'Access-Control-Allow-Methods: {{ .AllowMethods }}';
    more_set_headers 'Access-Control-Allow-Headers: {{ .AllowHeaders }}';
    more_set_headers 'Access-Control-Max-Age: {{ .MaxAge }}';
 }

if ($cors = "trueoptions") {
    more_set_headers 'Access-Control-Allow-Origin: $http_origin';
    {{ if .AllowCredentials }}more_set_headers 'Access-Control-Allow-Credentials: {{ .AllowCredentials }}';{{ end }}
    more_set_headers 'Access-Control-Allow-Methods: {{ .AllowMethods }}';
    more_set_headers 'Access-Control-Allow-Headers: {{ .AllowHeaders }}';
    more_set_headers 'Access-Control-Max-Age: {{ .MaxAge }}';
    more_set_headers 'Content-Type: text/plain charset=UTF-8';
    more_set_headers 'Content-Length: 0';
    return {{ .OptionsStatusCode }};
 }
`,
	TemplateFuncMap: template.FuncMap{
		"buildCorsOriginRegex": buildCorsOriginRegex,
	},
}

func buildCorsOriginRegex(origin string) string {
	corsOrigins := strings.Split(origin, ",")
	for i, origin := range corsOrigins {
		corsOrigins[i] = strings.TrimSpace(origin)
	}
	if len(corsOrigins) == 1 && corsOrigins[0] == "*" {
		return "set $http_origin *;\nset $cors 'true';"
	}

	var originsRegex string = "if ($http_origin ~* ("
	for i, origin := range corsOrigins {
		originTrimmed := strings.TrimSpace(origin)
		if len(originTrimmed) > 0 {
			builtOrigin := buildOriginRegex(originTrimmed)
			originsRegex += builtOrigin
			if i != len(corsOrigins)-1 {
				originsRegex = originsRegex + "|"
			}
		}
	}
	originsRegex = originsRegex + ")$ ) { set $cors 'true'; }"
	return originsRegex
}

func buildOriginRegex(origin string) string {
	origin = regexp.QuoteMeta(origin)
	origin = strings.Replace(origin, "\\*", `[A-Za-z0-9\-]+`, 1)
	return fmt.Sprintf("(%s)", origin)
}
