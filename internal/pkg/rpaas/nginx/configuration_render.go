// Copyright 2019 tsuru authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package nginx

import (
	"bytes"
	"crypto/x509"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"text/template"

	sprig "github.com/Masterminds/sprig/v3"
	nginxv1alpha1 "github.com/tsuru/nginx-operator/api/v1alpha1"
	"github.com/tsuru/rpaas-operator/api/v1alpha1"
	"github.com/tsuru/rpaas-operator/pkg/util"
	"k8s.io/apimachinery/pkg/api/resource"
)

var trimTrailingSpacesRegex = regexp.MustCompile(`[ \t]+?\n`)

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
	// Modules is a map of installed modules, using a map instead of a slice
	// allow us to use `hasKey` inside templates.
	Modules          map[string]interface{}
	FullCertificates []CertificateData
}

type CertificateData struct {
	Certificate *x509.Certificate
	SecretItem  nginxv1alpha1.TLSSecretItem
}

type rpaasConfigurationRenderer struct {
	t *template.Template
}

func (r *rpaasConfigurationRenderer) Render(c ConfigurationData) (string, error) {
	buffer := &bytes.Buffer{}
	err := r.t.Execute(buffer, c)
	if err != nil {
		return "", err
	}
	result := buffer.String()
	return trimTrailingSpacesRegex.ReplaceAllString(result, "\n"), nil
}

func NewConfigurationRenderer(cb ConfigurationBlocks) (ConfigurationRenderer, error) {
	tpl, err := defaultMainTemplate.Clone()
	if err != nil {
		return nil, err
	}

	if cb.MainBlock != "" {
		tpl, err = template.New("main").Funcs(templateFuncs).Parse(cb.MainBlock)
		if err != nil {
			return nil, err
		}
	}

	if _, err = tpl.New("root").Parse(cb.RootBlock); err != nil {
		return nil, err
	}

	if _, err = tpl.New("http").Parse(cb.HttpBlock); err != nil {
		return nil, err
	}

	if _, err = tpl.New("server").Parse(cb.ServerBlock); err != nil {
		return nil, err
	}

	if _, err = tpl.New("lua-server").Parse(cb.LuaServerBlock); err != nil {
		return nil, err
	}

	if _, err = tpl.New("lua-worker").Parse(cb.LuaWorkerBlock); err != nil {
		return nil, err
	}

	return &rpaasConfigurationRenderer{t: tpl}, nil
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

type nginxServer struct {
	Default     bool
	Certificate *x509.Certificate
	SecretItem  nginxv1alpha1.TLSSecretItem
}

func nginxServers(c ConfigurationData) []nginxServer {
	if len(c.FullCertificates) == 0 {
		return []nginxServer{
			{Certificate: nil, Default: true},
		}
	}

	servers := []nginxServer{}

	for i, cert := range c.FullCertificates {
		servers = append(servers, nginxServer{
			Default:     i == 0,
			Certificate: cert.Certificate,
			SecretItem:  cert.SecretItem,
		})
	}

	return servers
}

var internalTemplateFuncs = template.FuncMap(map[string]interface{}{
	"boolValue":               v1alpha1.BoolValue,
	"buildLocationKey":        buildLocationKey,
	"hasRootPath":             hasRootPath,
	"toLower":                 strings.ToLower,
	"toUpper":                 strings.ToUpper,
	"managePort":              managePort,
	"httpPort":                httpPort,
	"httpsPort":               httpsPort,
	"purgeLocationMatch":      purgeLocationMatch,
	"vtsLocationMatch":        vtsLocationMatch,
	"contains":                strings.Contains,
	"hasPrefix":               strings.HasPrefix,
	"hasSuffix":               strings.HasSuffix,
	"k8sQuantityToNginx":      k8sQuantityToNginx,
	"tlsSessionTicketEnabled": tlsSessionTicketEnabled,
	"tlsSessionTicketKeys":    tlsSessionTicketKeys,
	"tlsSessionTicketTimeout": tlsSessionTicketTimeout,
	"nginxServers":            nginxServers,
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
{{- $modules := .Modules -}}
{{- $nginxServers := . | nginxServers -}}

# This file was generated by RPaaS (https://github.com/tsuru/rpaas-operator.git)
# Do not modify this file, any change will be lost.

{{- with $config.User }}
user {{ . }};
{{- end }}

{{- with $config.WorkerProcesses }}
worker_processes {{ . }};
{{- end }}


{{- range $mod, $_ := $modules }}
load_module "modules/{{ $mod }}.so";
{{- else }}
include modules/*.conf;
{{- end }}

{{ template "root" . }}

events {
    {{- with $config.WorkerConnections }}
    worker_connections {{ . }};
    {{- end }}
}

http {
    include       mime.types;
    default_type  application/octet-stream;

    {{- if not (boolValue $config.SyslogEnabled) }}
    access_log /dev/stdout combined;
    error_log  /dev/stderr;
    {{- else }}
    access_log syslog:server={{ $config.SyslogServerAddress }}
        {{- with $config.SyslogFacility }},facility={{ . }}{{ end }}
        {{- with $config.SyslogTag }},tag={{ . }}{{ end}}
        combined;

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

    {{- range $index, $bind := $instance.Spec.Binds }}

      {{- if eq $index 0 }}
        upstream rpaas_default_upstream {
          server {{ $bind.Host }};

          {{- with $config.UpstreamKeepalive }}
          keepalive {{ . }};
          {{- end }}
      }
      {{- end }}

      upstream rpaas_backend_{{ $bind.Name }} {
        server {{ $bind.Host }};
      {{- with $config.UpstreamKeepalive }}
      keepalive {{ . }};
      {{- end }}
      }

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

    {{ template "http" . }}

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

    {{- range $_, $nginxServer := $nginxServers }}
    server {
        listen {{ httpPort $instance }}{{ with $nginxServer.Default }} default_server{{ end }}
            {{- with $config.HTTPListenOptions }} {{ . }}{{ end }};

        {{- with $nginxServer.Certificate }}
        listen {{ httpsPort $instance }}{{ with $nginxServer.Default }} default_server{{ end }} ssl http2
            {{- with $config.HTTPSListenOptions }} {{ . }}{{ end }};

        {{- with $nginxServer.Certificate.DNSNames}}
        server_name {{- range $_, $dnsName := $nginxServer.Certificate.DNSNames }} {{ $dnsName }}{{- end }};
        {{- end }}

        ssl_certificate     certs/{{ with $nginxServer.SecretItem.CertificatePath }}{{ . }}{{ else }}{{ $nginxServer.SecretItem.CertificateField }}{{ end }};
        ssl_certificate_key certs/{{ with $nginxServer.SecretItem.KeyPath }}{{ . }}{{ else }}{{ $nginxServer.SecretItem.KeyField }}{{ end }};
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

        {{- if $instance.Spec.Locations }}
        {{- range $_, $location := $instance.Spec.Locations }}
        location {{ $location.Path }} {
        {{- if $location.Destination }}
            {{- if $location.ForceHTTPS }}
            if ($scheme = 'http') {
                return 301 https://$http_host$request_uri;
            }
            {{- end }}

            proxy_set_header Connection "";
            proxy_set_header Host {{ $location.Destination }};

            proxy_pass http://{{ buildLocationKey "" $location.Path }}/;
            proxy_redirect ~^http://{{ buildLocationKey "" $location.Path }}(:\d+)?/(.*)$ {{ $location.Path }}$2;
        {{- else }}
        {{- with $location.Content.Value }}
            {{ . }}
        {{- end }}
        {{- end }}
        }
        {{- end }}
        {{- end }}

        {{- if not (hasRootPath $instance.Spec.Locations) }}
        {{- if $instance.Spec.Binds }}
        location / {
            proxy_set_header Connection "";
            proxy_set_header Host {{ (index $instance.Spec.Binds 0).Host }};

            proxy_pass http://rpaas_default_upstream/;
            proxy_redirect ~^http://rpaas_default_upstream(:\d+)?/(.*)$ /$2;
        }
        {{- else }}
        location / {
            default_type "text/plain";
            return 404 "instance not bound\n";
        }
        {{- end}}
        {{- end}}

        {{ template "server" .}}
    }
    {{- end}}
}
`
