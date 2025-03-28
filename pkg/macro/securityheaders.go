// Copyright 2025 tsuru authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package macro

var securityHeaders = Macro{
	Name: "security_headers",
	Args: []MacroArg{
		{Name: "noSniffContentType", Default: "true", Required: false, Type: MacroArgTypeBool},
		{Name: "xssProtection", Default: "true", Required: false, Type: MacroArgTypeBool},
		{Name: "frameOptions", Default: "SAMEORIGIN", Required: false, Type: MacroArgTypeString},
		{Name: "upgradeInsecureRequests", Default: "true", Required: false, Type: MacroArgTypeBool},
		{Name: "forceHTTPS", Default: "true", Required: false, Type: MacroArgTypeBool},
		{Name: "forceHTTPMaxAge", Default: "31536000", Required: false, Type: MacroArgTypeInt},
		{Name: "forceHTTPIncludeSubDomains", Default: "true", Required: false, Type: MacroArgTypeBool},
	},
	Template: `{{- if .NoSniffContentType -}}
more_set_headers 'X-Content-Type-Options: nosniff';
{{ end -}}
{{- if .XssProtection -}}
more_set_headers 'X-XSS-Protection: 1; mode=block';
{{ end -}}
{{- if ne .FrameOptions "NONE" -}}
more_set_headers 'X-Frame-Options: {{ .FrameOptions }}';
{{ end -}}
{{- if .UpgradeInsecureRequests -}}
more_set_headers 'Content-Security-Policy: upgrade-insecure-requests';
{{ end -}}
{{- if .ForceHTTPS -}}
more_set_headers 'Strict-Transport-Security: max-age={{ .ForceHTTPMaxAge }}; {{ if .ForceHTTPIncludeSubDomains }}includeSubDomains{{ end }}';
{{ end -}}`,
}
