// Copyright 2019 tsuru authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package nginx

import (
	"bytes"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"text/template"

	sprig "github.com/Masterminds/sprig/v3"
	nginxv1alpha1 "github.com/tsuru/nginx-operator/api/v1alpha1"
	"k8s.io/apimachinery/pkg/api/resource"

	"github.com/tsuru/rpaas-operator/api/v1alpha1"
	clientTypes "github.com/tsuru/rpaas-operator/pkg/rpaas/client/types"
	"github.com/tsuru/rpaas-operator/pkg/util"
)

var trimTrailingSpacesRegex = regexp.MustCompile(`[ \t]+?\n`)
var nginxTemplate = &template.Template{}
var errRenderInnerTemplate = fmt.Errorf("template contains renderInnerTemplate")

type ConfigurationRenderer interface {
	Render(ConfigurationData) (string, error)
}

type ConfigurationBlocks struct {
	MainBlock      string
	RootBlock      string
	HttpBlock      string
	ServerBlock    string
	LuaServerBlock string
	LuaWorkerBlock string
}

type ConfigurationData struct {
	Config   *v1alpha1.NginxConfig
	Instance *v1alpha1.RpaasInstance
	Plan     *v1alpha1.RpaasPlan
	NginxTLS []nginxv1alpha1.NginxTLS
	Servers  []*Server
	Binds    []clientTypes.Bind

	// DEPRECATED: Modules is a map of installed modules, using a map instead of a slice
	// allow us to use `hasKey` inside templates.
	Modules map[string]interface{}
}

type rpaasConfigurationRenderer struct {
	t *template.Template
}

func (r *rpaasConfigurationRenderer) Render(c ConfigurationData) (string, error) {
	buffer := &bytes.Buffer{}
	if c.Servers == nil {
		c.Servers = produceServers(&c.Instance.Spec, c.NginxTLS)
	}
	initListenOptions(c.Servers, c.Config)
	c.Binds = clientTypes.NewBinds(c.Instance.Spec.Binds)
	err := r.t.Execute(buffer, c)
	if err != nil {
		return "", err
	}
	result := buffer.String()
	return trimTrailingSpacesRegex.ReplaceAllString(result, "\n"), nil
}

func NewConfigurationRenderer(cb ConfigurationBlocks) (ConfigurationRenderer, error) {
	var err error
	nginxTemplate, err = defaultMainTemplate.Clone()
	if err != nil {
		return nil, err
	}

	if cb.MainBlock != "" {
		nginxTemplate, err = template.New("main").Funcs(templateFuncs).Parse(cb.MainBlock)
		if err != nil {
			return nil, err
		}
	}

	if _, err = nginxTemplate.New("root").Parse(cb.RootBlock); err != nil {
		return nil, err
	}

	if _, err = nginxTemplate.New("http").Parse(cb.HttpBlock); err != nil {
		return nil, err
	}

	if _, err = nginxTemplate.New("server").Parse(cb.ServerBlock); err != nil {
		return nil, err
	}

	if _, err = nginxTemplate.New("lua-server").Parse(cb.LuaServerBlock); err != nil {
		return nil, err
	}

	if _, err = nginxTemplate.New("lua-worker").Parse(cb.LuaWorkerBlock); err != nil {
		return nil, err
	}

	return &rpaasConfigurationRenderer{t: nginxTemplate}, nil
}

func renderInnerTemplate(name string, nginx ConfigurationData) (string, error) {
	tpl := nginxTemplate.Lookup(name)
	parsedTemplate := tpl.Tree.Root.String()
	if strings.Contains(parsedTemplate, "renderInnerTemplate") {
		return "", errRenderInnerTemplate
	}
	r := &rpaasConfigurationRenderer{t: tpl}
	return r.Render(nginx)
}

func buildLocationKey(prefix, path string) string {
	if path == "" {
		panic("cannot build location key due path is missing")
	}

	if prefix == "" {
		prefix = "rpaas_locations_"
	}

	key := "root"
	if path != "/" {
		key = strings.ReplaceAll(path, "/", "_")
	}

	return fmt.Sprintf("%s%s", prefix, key)
}

func hasRootPath(locations []v1alpha1.Location) bool {
	for _, location := range locations {
		if location.Path == "/" {
			return true
		}
	}
	return false
}

func httpPort(instance *v1alpha1.RpaasInstance) int32 {
	if instance != nil {
		port := util.PortByName(instance.Spec.PodTemplate.Ports, PortNameHTTP)
		if port != 0 {
			return port
		}

		if instance.Spec.PodTemplate.HostNetwork {
			return 80
		}
	}

	return 8080
}

func httpsPort(instance *v1alpha1.RpaasInstance) int32 {
	if instance != nil {
		port := util.PortByName(instance.Spec.PodTemplate.Ports, PortNameHTTPS)
		if port != 0 {
			return port
		}

		if instance.Spec.PodTemplate.HostNetwork {
			return 443
		}
	}

	return 8443
}

func proxyProtocolHTTPPort(instance *v1alpha1.RpaasInstance) int32 {
	if instance != nil {
		port := util.PortByName(instance.Spec.PodTemplate.Ports, PortNameProxyProtocolHTTP)
		if port != 0 {
			return port
		}
	}

	return DefaultProxyProtocolHTTPPort
}

func proxyProtocolHTTPSPort(instance *v1alpha1.RpaasInstance) int32 {
	if instance != nil {
		port := util.PortByName(instance.Spec.PodTemplate.Ports, PortNameProxyProtocolHTTPS)
		if port != 0 {
			return port
		}
	}

	return DefaultProxyProtocolHTTPSPort
}

func managePort(instance *v1alpha1.RpaasInstance) int32 {
	if instance != nil {
		port := util.PortByName(instance.Spec.PodTemplate.Ports, PortNameManagement)
		if port != 0 {
			return port
		}
	}

	return DefaultManagePort
}

func k8sQuantityToNginx(quantity *resource.Quantity) string {
	if quantity == nil || quantity.IsZero() {
		return "0"
	}

	bytesN, _ := quantity.AsInt64()
	return strconv.Itoa(int(bytesN))
}

func tlsSessionTicketEnabled(instance *v1alpha1.RpaasInstance) bool {
	return instance != nil &&
		instance.Spec.TLSSessionResumption != nil &&
		instance.Spec.TLSSessionResumption.SessionTicket != nil
}

func tlsSessionTicketKeys(instance *v1alpha1.RpaasInstance) int {
	if !tlsSessionTicketEnabled(instance) {
		return 0
	}

	return int(instance.Spec.TLSSessionResumption.SessionTicket.KeepLastKeys) + 1
}

func tlsSessionTicketTimeout(instance *v1alpha1.RpaasInstance) int {
	nkeys := tlsSessionTicketKeys(instance)

	keyRotationInterval := v1alpha1.DefaultSessionTicketKeyRotationInteval
	if tlsSessionTicketEnabled(instance) &&
		instance.Spec.TLSSessionResumption.SessionTicket.KeyRotationInterval != uint32(0) {
		keyRotationInterval = instance.Spec.TLSSessionResumption.SessionTicket.KeyRotationInterval
	}

	return nkeys * int(keyRotationInterval)
}

func defaultCertificate(instance *v1alpha1.RpaasInstance) *nginxv1alpha1.NginxTLS {
	if len(instance.Spec.TLS) == 0 {
		return nil
	}

	for _, tls := range instance.Spec.TLS {
		if len(tls.Hosts) == 0 {
			return &tls
		}
	}

	return &instance.Spec.TLS[0]
}

var internalTemplateFuncs = template.FuncMap(map[string]interface{}{
	"renderInnerTemplate":     renderInnerTemplate,
	"boolValue":               v1alpha1.BoolValue,
	"buildLocationKey":        buildLocationKey,
	"hasRootPath":             hasRootPath,
	"toLower":                 strings.ToLower,
	"toUpper":                 strings.ToUpper,
	"managePort":              managePort,
	"httpPort":                httpPort,
	"httpsPort":               httpsPort,
	"proxyProtocolHTTPPort":   proxyProtocolHTTPPort,
	"proxyProtocolHTTPSPort":  proxyProtocolHTTPSPort,
	"purgeLocationMatch":      purgeLocationMatch,
	"vtsLocationMatch":        vtsLocationMatch,
	"contains":                strings.Contains,
	"hasPrefix":               strings.HasPrefix,
	"hasSuffix":               strings.HasSuffix,
	"k8sQuantityToNginx":      k8sQuantityToNginx,
	"tlsSessionTicketEnabled": tlsSessionTicketEnabled,
	"tlsSessionTicketKeys":    tlsSessionTicketKeys,
	"tlsSessionTicketTimeout": tlsSessionTicketTimeout,
	"defaultCertificate":      defaultCertificate,

	"iterate": func(n int) []int {
		v := make([]int, n)
		for i := 0; i < n; i++ {
			v[i] = i
		}
		return v
	},
})

var templateFuncs = func() template.FuncMap {
	funcs := sprig.GenericFuncMap()
	for k, v := range internalTemplateFuncs {
		funcs[k] = v
	}
	return template.FuncMap(funcs)
}()

var defaultMainTemplate = template.Must(template.New("main").
	Funcs(templateFuncs).
	Parse(rawNginxConfiguration))

// NOTE: This nginx's configuration works fine with the "tsuru/nginx-tsuru"
// container image. We rely on this image to load some required modules
// (such as echo, uuid4, more_set_headers, vts, etc), as well as point to some
// files in the system directory. Be aware when using a different container
// image.
var rawNginxConfiguration = `
{{- $all := . -}}
{{- $config := .Config -}}
{{- $instance := .Instance -}}
{{- $nginxTLS := .NginxTLS -}}
{{- $binds := .Binds -}}
{{- $servers := .Servers -}}
{{- $httpBlock := renderInnerTemplate "http" . -}}

# This file was generated by RPaaS (https://github.com/tsuru/rpaas-operator.git)
# Do not modify this file, any change will be lost.

{{- with $config.User }}
user {{ . }};
{{- end }}

{{- with $config.WorkerProcesses }}
worker_processes {{ . }};
{{- end }}

include modules/*.conf;

{{ template "root" . }}

events {
    {{- with $config.WorkerConnections }}
    worker_connections {{ . }};
    {{- end }}
}

http {
    include       mime.types;
    default_type  application/octet-stream;

    {{- if $config.ResolverAddresses }}
    resolver {{ join " " $config.ResolverAddresses }}{{ with $config.ResolverTTL }} ttl={{ . }}{{ end }};
    {{- end }}

    {{- $logFormatName := default "rpaasv2" $config.LogFormatName }}

    {{- if $config.LogFormat }}
    log_format {{ $config.LogFormatName }} {{ with $config.LogFormatEscape}}escape={{ . }}{{ end }} {{ $config.LogFormat }};
    {{- else }}
    log_format {{ $logFormatName }} escape=json
    '{'
      {{- range $key, $value := $config.LogAdditionalFields }}
      '"{{ $key }}":"{{ $value }}",'
      {{- end }}
      '"remote_addr":"${remote_addr}",'
      '"remote_user":"${remote_user}",'
      '"time_local":"${time_local}",'
      '"request":"${request}",'
      '"status":"${status}",'
      '"body_bytes_sent":"${body_bytes_sent}",'
      '"referer":"${http_referer}",'
      '"user_agent":"${http_user_agent}"'
      {{- range $index, $header := $config.LogAdditionalHeaders }}
      {{- if not $index }}{{ "\n" }}','{{ end }}
      {{- $h := lower (replace "-" "_" $header) }}
      '"header_{{ $h }}":"${http_{{ $h }}}" {{- if lt (add1 $index) (len $config.LogAdditionalHeaders) }},{{ end }}'
      {{- end }}
    '}';
    {{- end }}

    {{- if not (boolValue $config.SyslogEnabled) }}
    access_log /dev/stdout {{ $logFormatName }};
    error_log  /dev/stderr;
    {{- else }}
    access_log syslog:server={{ $config.SyslogServerAddress }}
        {{- with $config.SyslogFacility }},facility={{ . }}{{ end }}
        {{- with $config.SyslogTag }},tag={{ . }}{{ end}}
        {{ $logFormatName }};

    error_log syslog:server={{ $config.SyslogServerAddress }}
        {{- with $config.SyslogFacility }},facility={{ . }}{{ end }}
        {{- with $config.SyslogTag }},tag={{ . }}{{ end }};
    {{- end }}

    proxy_http_version 1.1;

    {{- if boolValue $config.CacheEnabled }}
    proxy_cache_path {{ $config.CachePath }}/nginx levels=1:2 keys_zone=rpaas:{{ k8sQuantityToNginx $config.CacheZoneSize }}
        {{- with $config.CacheInactive }} inactive={{ . }}{{ end }}
        {{- with $config.CacheSize }} max_size={{ k8sQuantityToNginx . }}{{ end }}
        {{- with $config.CacheLoaderFiles }} loader_files={{ . }}{{ end }};

    proxy_temp_path {{ $config.CachePath }}/nginx_tmp 1 2;
    {{- end }}

    {{- if boolValue $config.VTSEnabled }}
    vhost_traffic_status_zone;

    {{- with $config.VTSStatusHistogramBuckets }}
    vhost_traffic_status_histogram_buckets {{ . }};
    {{- end }}
    {{- end}}

    {{- if tlsSessionTicketEnabled $instance }}
    {{- with $instance.Spec.TLSSessionResumption.SessionTicket }}{{ "\n" }}
    ssl_session_cache off;

    ssl_session_tickets    on;
    {{- range $index, $_ := (iterate (tlsSessionTicketKeys $instance)) }}
    ssl_session_ticket_key tickets/ticket.{{ $index }}.key;
    {{- end }}

    {{- with (tlsSessionTicketTimeout $instance) }}
    ssl_session_timeout {{ . }}m;
    {{- end }}
    {{- end }}
    {{- end }}

    {{- range $_, $bind := $binds }}
    {{- range $_, $upstream := $bind.Upstreams }}
      upstream {{ $upstream }} {
        server {{ $bind.Host }};
      {{- with $config.UpstreamKeepalive }}
      keepalive {{ . }};
      {{- end }}
      }
    {{- end }}
    {{- end }}

    {{- range $_, $location := $instance.Spec.Locations }}
    {{- if $location.Destination }}
    upstream {{ buildLocationKey "" $location.Path }} {
        server {{ $location.Destination }};

        {{- with $config.UpstreamKeepalive }}
        keepalive {{ . }};
        {{- end }}
    }
    {{- end }}
    {{- end }}

    init_by_lua_block {
        {{ template "lua-server" . }}
    }

    init_worker_by_lua_block {
        {{- if tlsSessionTicketEnabled $instance }}
        {{- with $instance.Spec.TLSSessionResumption.SessionTicket }}
        local rpaasv2_session_ticket_reloader = require('tsuru.rpaasv2.tls.session_ticket_reloader'):new({
            ticket_file      = '/etc/nginx/tickets/ticket.0.key',
            retain_last_keys = {{ tlsSessionTicketKeys $instance }},
            sync_interval    = 1,
        })
        rpaasv2_session_ticket_reloader:start_worker()
        {{- end }}
        {{- end }}

        {{ template "lua-worker" . }}
    }

    {{ $httpBlock }}

    server {
        listen {{ managePort $instance }};

        {{- if boolValue $config.CacheEnabled }}
        location ~ {{ purgeLocationMatch }} {
            proxy_cache_purge rpaas $1$is_args$args;
        }
        {{- end }}

        {{- if boolValue $config.VTSEnabled }}
        location {{ vtsLocationMatch }} {
            vhost_traffic_status_bypass_limit on;
            vhost_traffic_status_bypass_stats on;
            vhost_traffic_status_display;
            vhost_traffic_status_display_format prometheus;
        }
        {{- end }}
    }

    {{- range $_, $server := $servers }}
    server {
        listen {{ httpPort $instance }}{{ with $server.Default }} default_server{{ end }}{{- with $server.HTTPListenOptions }} {{ . }}{{ end }};
        {{- if $server.TLS }}
        listen {{ httpsPort $instance }} ssl http2{{- with $server.HTTPSListenOptions }} {{ . }}{{ end }};
        {{- end }}

        {{- with $server.Name }}
        server_name {{ . }};
        {{- end }}

        {{- if $server.TLS }}
        ssl_certificate     certs/{{ $server.TLSSecretName }}/tls.crt;
        ssl_certificate_key certs/{{ $server.TLSSecretName }}/tls.key;
        {{- end }}

        {{- if boolValue $config.CacheEnabled }}
        proxy_cache rpaas;
        {{- end }}

        location = /_nginx_healthcheck {
            {{- if boolValue $config.VTSEnabled }}
            vhost_traffic_status_bypass_limit on;
            vhost_traffic_status_bypass_stats on;
            {{- end }}

            access_log off;

            default_type "text/plain";
            return 200 "WORKING\n";
        }

        {{- if $server.Locations }}
        {{- range $_, $location := $server.Locations }}
        location {{ $location.Path }} {
        {{- if $location.Destination }}
            {{- if $location.ForceHTTPS }}
            if ($scheme = 'http') {
                return 301 https://$http_host$request_uri;
            }
            {{- end }}

            proxy_set_header Connection "";
            proxy_set_header Host {{ $location.Destination }};

            proxy_pass     http://{{ buildLocationKey "" $location.Path }}/;
            proxy_redirect ~^http://{{ buildLocationKey "" $location.Path }}(:\d+)?/(.*)$ {{ $location.Path }}$2;
        {{- else }}
        {{- with $location.Content.Value }}
            {{ . }}
        {{- end }}
        {{- end }}
        }
        {{- end }}
        {{- end }}

        {{- if not (hasRootPath $server.Locations) }}
        {{- if $instance.Spec.Binds }}
        location / {
            proxy_set_header Connection "";
            proxy_set_header Host {{ (index $instance.Spec.Binds 0).Host }};

            proxy_pass     http://rpaas_default_upstream/;
            proxy_redirect ~^http://rpaas_default_upstream(:\d+)?/(.*)$ /$2;
        }
        {{- else }}
        location / {
            default_type "text/plain";
            return 404 "instance not bound\n";
        }
        {{- end}}
        {{- end}}

        {{- if $server.HasBlockServer }}
        {{ $server.ServerBlockContent }}
        {{- else }}
        {{ template "server" $all }}
        {{- end }}
    }
{{- end }}
}
`
