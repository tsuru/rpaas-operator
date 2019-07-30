package nginx

import (
	"bytes"
	"fmt"
	"strings"
	"text/template"

	"github.com/tsuru/rpaas-operator/pkg/apis/extensions/v1alpha1"
)

type ConfigurationRenderer interface {
	Render(ConfigurationData) (string, error)
}

type ConfigurationBlocks struct {
	RootBlock   string
	HttpBlock   string
	ServerBlock string
}

type ConfigurationData struct {
	Config   *v1alpha1.NginxConfig
	Instance *v1alpha1.RpaasInstanceSpec
}

type rpaasConfigurationRenderer struct {
	t *template.Template
}

func (r *rpaasConfigurationRenderer) Render(c ConfigurationData) (string, error) {
	buffer := &bytes.Buffer{}
	err := r.t.Execute(buffer, c)
	return buffer.String(), err
}

func NewRpaasConfigurationRenderer(cb ConfigurationBlocks) ConfigurationRenderer {
	baseTemplate := template.Must(defaultRpaasConfigurationTemplate.Clone())
	template.Must(baseTemplate.New("root").Parse(cb.RootBlock))
	template.Must(baseTemplate.New("http").Parse(cb.HttpBlock))
	template.Must(baseTemplate.New("server").Parse(cb.ServerBlock))
	return &rpaasConfigurationRenderer{t: baseTemplate}
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

var defaultRpaasConfigurationTemplate = template.Must(template.New("rpaas-configuration-template").
	Funcs(template.FuncMap(map[string]interface{}{
		"buildLocationKey": buildLocationKey,
		"hasRootPath":      hasRootPath,
	})).
	Parse(rawNginxConfiguration))

// NOTE: This nginx's configuration works fine with the "tsuru/nginx-tsuru"
// container image. We rely on this image to load some required modules
// (such as echo, uuid4, more_set_headers, vts, etc), as well as point to some
// files in the system directory. Be aware when using a different container
// image.
var rawNginxConfiguration = `
{{ $all := . }}
{{ $config := .Config }}
{{ $instance := .Instance }}

# This file was generated by RPaaS (https://github.com/tsuru/rpaas-operator.git)
# Do not modify this file, any change will be lost.

user {{with .Config.User}}{{.}}{{else}}nginx{{end}};
worker_processes {{with .Config.WorkerProcesses}}{{.}}{{else}}1{{end}};

include modules/*.conf;

events {
    worker_connections {{with .Config.WorkerConnections}}{{.}}{{else}}1024{{end}};
}

{{template "root" .}}

http {
    include       mime.types;
    default_type  application/octet-stream;
    server_tokens off;

    sendfile          on;
    keepalive_timeout 65;

{{if .Config.RequestIDEnabled}}
    uuid4 $request_id_uuid;
    map $http_x_request_id $request_id_final {
        default $request_id_uuid;
        "~."    $http_x_request_id;
    }
{{end}}

    map $http_x_real_ip $real_ip_final {
        default $remote_addr;
        "~."    $http_x_real_ip;
    }

    map $http_x_forwarded_proto $forwarded_proto_final {
        default $scheme;
        "~."    $http_x_forwarded_proto;
    }

    map $http_x_forwarded_host $forwarded_host_final {
        default $host;
        "~." $http_x_forwarded_host;
    }

    log_format rpaas_combined
        '${remote_addr}\t${host}\t${request_method}\t${request_uri}\t${server_protocol}\t'
        '${http_referer}\t${http_x_mobile_group}\t'
        'Local:\t${status}\t*${connection}\t${body_bytes_sent}\t${request_time}\t'
        'Proxy:\t${upstream_addr}\t${upstream_status}\t${upstream_cache_status}\t'
        '${upstream_response_length}\t${upstream_response_time}\t${request_uri}\t'
{{if .Config.RequestIDEnabled}}
        'Agent:\t${http_user_agent}\t$request_id_final\t'
{{else}}
        'Agent:\t${http_user_agent}\t'
{{end}}
        'Fwd:\t${http_x_forwarded_for}';

{{if .Config.SyslogEnabled}}
    access_log syslog:server={{.Config.SyslogServerAddress}},facility={{with .Config.SyslogFacility}}{{.}}{{else}}local6{{end}},tag={{with .Config.SyslogTag}}{{.}}{{else}}rpaas{{end}} rpaas_combined;
    error_log syslog:server={{.Config.SyslogServerAddress}},facility={{with .Config.SyslogFacility}}{{.}}{{else}}local6{{end}},tag={{with .Config.SyslogTag}}{{.}}{{else}}rpaas{{end}};
{{else}}
    access_log /dev/stdout rpaas_combined;
    error_log  /dev/stderr;
{{end}}

{{if .Config.CacheEnabled}}
    proxy_cache_path {{.Config.CachePath}}/nginx levels=1:2 keys_zone=rpaas:{{.Config.CacheZoneSize}} inactive={{.Config.CacheInactive}} max_size={{.Config.CacheSize}} loader_files={{.Config.CacheLoaderFiles}};
    proxy_temp_path  {{.Config.CachePath}}/nginx_temp 1 2;
{{end}}

    gzip                on;
    gzip_buffers        128 4k;
    gzip_comp_level     5;
    gzip_http_version   1.0;
    gzip_min_length     20;
    gzip_proxied        any;
    gzip_vary           on;
    gzip_types          application/atom+xml application/javascript
                        application/json application/rss+xml
                        application/xml application/x-javascript
                        text/css text/javascript text/plain text/xml;

{{if .Config.VTSEnabled}}
    vhost_traffic_status_zone;
{{end}}

{{if $instance.Host}}
    upstream rpaas_default_upstream {
        server {{$instance.Host}};
        {{with $config.UpstreamKeepalive}}keepalive {{.}};{{end}}
    }
{{end}}

{{range $_, $location := $instance.Locations}}
{{if $location.Destination}}
    upstream {{buildLocationKey "" $location.Path}} {
        server {{$location.Destination}};
        {{with $config.UpstreamKeepalive}}keepalive {{.}};{{end}}
    }
{{end}}
{{end}}

    {{template "http" .}}

		server {
			listen 8800;

{{if .Config.VTSEnabled}}
			location /status {
				vhost_traffic_status_display;
        vhost_traffic_status_display_format prometheus;
			}
{{end}}

		}

    server {
        listen 8080 default_server{{with .Config.HTTPListenOptions}} {{.}}{{end}};

{{if .Instance.Certificates }}
{{ $opts := .Config.HTTPSListenOptions }}
{{range $index, $item := .Instance.Certificates.Items}}
{{if and (eq $item.CertificateField "default.crt") (eq $item.KeyField "default.key")}}
        listen 8443 ssl{{with $opts}} {{.}}{{end}};

        ssl_certificate     certs/{{with $item.CertificatePath}}{{.}}{{else}}{{$item.CertificateField}}{{end}};
        ssl_certificate_key certs/{{with $item.KeyPath}}{{.}}{{else}}{{$item.KeyField}}{{end}};

        ssl_protocols TLSv1 TLSv1.1 TLSv1.2;
        ssl_ciphers 'ECDHE-ECDSA-CHACHA20-POLY1305:ECDHE-RSA-CHACHA20-POLY1305:ECDHE-ECDSA-AES128-GCM-SHA256:ECDHE-RSA-AES128-GCM-SHA256:ECDHE-ECDSA-AES256-GCM-SHA384:ECDHE-RSA-AES256-GCM-SHA384:DHE-RSA-AES128-GCM-SHA256:DHE-RSA-AES256-GCM-SHA384:ECDHE-ECDSA-AES128-SHA256:ECDHE-RSA-AES128-SHA256:ECDHE-ECDSA-AES128-SHA:ECDHE-RSA-AES256-SHA384:ECDHE-RSA-AES128-SHA:ECDHE-ECDSA-AES256-SHA384:ECDHE-ECDSA-AES256-SHA:ECDHE-RSA-AES256-SHA:DHE-RSA-AES128-SHA256:DHE-RSA-AES128-SHA:DHE-RSA-AES256-SHA256:DHE-RSA-AES256-SHA:ECDHE-ECDSA-DES-CBC3-SHA:ECDHE-RSA-DES-CBC3-SHA:EDH-RSA-DES-CBC3-SHA:AES128-GCM-SHA256:AES256-GCM-SHA384:AES128-SHA256:AES256-SHA256:AES128-SHA:AES256-SHA:DES-CBC3-SHA:!DSS';
        ssl_prefer_server_ciphers on;
        ssl_session_cache shared:SSL:200m;
        ssl_session_timeout 1h;
{{end}}
{{end}}
{{end}}

        port_in_redirect off;
{{if .Config.CacheEnabled}}
        proxy_cache rpaas;
        proxy_cache_use_stale error timeout updating invalid_header http_500 http_502 http_503 http_504;
        proxy_cache_lock on;
        proxy_cache_lock_age 60s;
        proxy_cache_lock_timeout 60s;
{{end}}
        proxy_read_timeout 20s;
        proxy_connect_timeout 10s;
        proxy_send_timeout 20s;
        proxy_http_version 1.1;

        location = /_nginx_healthcheck {
            default_type "text/plain";
            echo "WORKING";
        }

{{if $instance.Locations}}
{{range $_, $location := .Instance.Locations}}
        location {{$location.Path}} {

{{if $location.Destination}}
{{if $location.ForceHTTPS}}
            if ($scheme = 'http') {
                return 301 https://$http_host$request_uri;
            }
{{end}}
            proxy_set_header Host {{$location.Destination}};
            proxy_set_header X-Real-IP $remote_addr;
            proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
            proxy_set_header X-Forwarded-Proto $scheme;
            proxy_set_header X-Forwarded-Host $host;
            proxy_set_header Connection "";
            proxy_http_version 1.1;
            proxy_pass http://{{$location.Destination}}/;
            proxy_redirect ~^http://{{buildLocationKey "" $location.Path}}(:\d+)?/(.*)$ {{$location.Path}}$2;
{{else}}
{{with $location.Value}}
            {{.}}
{{end}}
{{end}}
        }
{{end}}
{{end}}

{{if not (hasRootPath $instance.Locations)}}
{{if $instance.Host}}
        location / {
            proxy_set_header Host {{$instance.Host}};
            proxy_set_header X-Real-IP $remote_addr;
            proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
            proxy_set_header X-Forwarded-Proto $scheme;
            proxy_set_header X-Forwarded-Host $host;
            proxy_set_header Connection "";
            proxy_http_version 1.1;
            proxy_pass http://rpaas_default_upstream/;
            proxy_redirect ~^http://rpaas_default_upstream(:\d+)?/(.*)$ /$2;
        }
{{else}}
        location / {
            default_type "text/plain";
            echo "instance not bound yet";
        }
{{end}}
{{end}}

        {{template "server" .}}
    }
}
`
