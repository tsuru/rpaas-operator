package generator

import (
	"bytes"
	"fmt"
	"strings"
	"text/template"

	"github.com/tsuru/rpaas-operator/pkg/apis/extensions/v1alpha1"
)

type ConfigRefReader interface {
	ReadConfigRef(ref v1alpha1.ConfigRef, ns string) (string, error)
}

type fullConfigParams struct {
	DefaultRootBlock   string
	RootBlock          string
	HTTPDefaultBlock   string
	HTTPBlock          string
	AdminServerBlock   string
	ServerDefaultBlock string
	ServerBlock        string
	Locations          []string
}

var fullConfigTpl = template.Must(template.New("full-config").Parse(`
{{.DefaultRootBlock}}
{{.RootBlock}}

http {
	{{.HTTPDefaultBlock}}
	{{.HTTPBlock}}
	{{.AdminServerBlock}}

	server {
		{{.ServerDefaultBlock}}
		{{.ServerBlock}}

		{{range .Locations -}}
			{{.}}
		{{end}}
	}
}
`))

var defaultBlocks = map[v1alpha1.BlockType]string{
	v1alpha1.BlockTypeRootDefault: `
user {{.Config.User}};
worker_processes {{.Config.WorkerProcesses}};
# include /etc/nginx/modules-enabled/*.conf;

events {
	worker_connections {{.Config.WorkerConnections}};
}
`,
	v1alpha1.BlockTypeHTTPDefault: `
# include      mime.types;
default_type application/octet-stream;
server_tokens off;

sendfile          on;
keepalive_timeout 65;

{{if .Config.RequestIDEnabled}}
	uuid4 $request_id_uuid;
	map $http_x_request_id $request_id_final {
	default $request_id_uuid;
	"~." $http_x_request_id;
	}
{{end}}

map $http_x_real_ip $real_ip_final {
	default $remote_addr;
	"~." $http_x_real_ip;
}

map $http_x_forwarded_proto $forwarded_proto_final {
	default $scheme;
	"~." $http_x_forwarded_proto;
}

map $http_x_forwarded_host $forwarded_host_final {
	default $host;
	"~." $http_x_forwarded_host;
}

{{if .Config.LocalLog}}
	access_log /var/log/nginx/access.log main;
	error_log  /var/log/nginx/error.log;
{{end}}



{{if .Config.SyslogServer}}
	log_format main
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

	access_log syslog:server={{.Config.SyslogServer}},facility=local6,tag={{if .Config.SyslogTag}}{{.Config.SyslogTag}}{{else}}rpaas{{end}} main;
	error_log syslog:server={{.Config.SyslogServer}},facility=local6,tag={{if .Config.SyslogTag}}{{.Config.SyslogTag}}{{else}}rpaas{{end}};
{{end}}


{{range $file, $codes := .Config.CustomErrorCodes}}
	error_page {{$codes | join " "}} /_nginx_errordocument/{{$file}};
{{end}}

proxy_cache_path /var/cache/nginx levels=1:2 keys_zone=rpaas:{{.Config.KeyZoneSize}} inactive={{.Config.CacheInactive}} max_size={{.Config.CacheSize}} loader_files={{.Config.LoaderFiles}};
proxy_temp_path  /var/cache/nginx_temp 1 2;

gzip                on;
gzip_buffers        128 4k;
gzip_comp_level     5;
gzip_http_version   1.0;
gzip_min_length     20;
gzip_proxied        any;
gzip_vary           on;
# Additional types, "text/html" is always compressed:
gzip_types          application/atom+xml application/javascript
					application/json application/rss+xml
					application/xml application/x-javascript
					text/css text/javascript text/plain text/xml;

{{if .Config.VtsEnabled}}
	vhost_traffic_status_zone;
{{end}}

# include sites-enabled/consul/upstreams.conf;
# include sites-enabled/consul/blocks/httb.conf;

{{if .Config.Lua}}
	lua_package_path "/usr/local/share/lualib/?.lua;;";
	lua_shared_dict my_cache 10m;
	lua_shared_dict locks 1m;
	include sites-enabled/consul/blocks/lua_*.conf;
{{end}}
`,
	v1alpha1.BlockTypeServerDefault: `
listen {{.Config.Listen}} default_server backlog={{.Config.ListenBacklog}};
# include /etc/nginx/main_ssl.conf;
server_name  _tsuru_nginx_app;
port_in_redirect off;

proxy_cache rpaas;
proxy_cache_use_stale error timeout updating invalid_header http_500 http_502 http_503 http_504;
proxy_cache_lock on;
proxy_cache_lock_age 60s;
proxy_cache_lock_timeout 60s;
more_set_input_headers "X-Real-IP: $real_ip_final";
more_set_input_headers "X-Forwarded-For: $proxy_add_x_forwarded_for";
more_set_input_headers "X-Forwarded-Proto: $forwarded_proto_final";
more_set_input_headers "X-Forwarded-Host: $forwarded_host_final";

{{if .Config.RequestIDEnabled}}
	more_set_input_headers "X-Request-ID: $request_id_final";
	{{if not .Config.DisableResponseRequestID}}
		more_set_headers "X-Request-ID: $request_id_final";
	{{end}}
{{end}}

more_clear_input_headers "X-Debug-Router";
proxy_read_timeout 20s;
proxy_connect_timeout 10s;
proxy_send_timeout 20s;
proxy_http_version 1.1;

{{if .Config.AdminLocationPurge}}
	proxy_cache_key $scheme$request_uri;
{{end}}


{{if and .Config.CustomErrorDir .Config.InterceptErrors}}
	proxy_intercept_errors on;
{{end}}

{{if .Config.CustomErrorDir}}
	location ~ ^/_nginx_errordocument/(.+) {
		internal;
		alias {{.Config.CustomErrorDir}}/$1;
	}
{{end}}
`,
}

var adminServerBlock = `
server {
	listen {{.Config.AdminListen}};
	
{{if .Config.AdminEnableSsl}}
	# include /etc/nginx/admin_ssl.conf;
{{end}}

	server_name  _tsuru_nginx_admin;

	location /healthcheck {
		echo "WORKING";
	}

{{if .Config.AdminLocationPurge}}
	location ~ ^/purge/(.+) {
		deny            all;
		proxy_cache_purge  rpaas $1$is_args$args;
	}
{{end}}

{{if .Config.VtsEnabled}}
	location /vts_status {
	  vhost_traffic_status_display;
	  vhost_traffic_status_display_format json;
	}
{{end}}
# include nginx_admin_locations.conf;
}
`

func renderTemplate(name, templateStr string, config v1alpha1.RpaasPlanSpec) (string, error) {
	funcMap := template.FuncMap{
		"join": strings.Join,
	}
	tpl, err := template.New(name).Funcs(funcMap).Parse(templateStr)
	if err != nil {
		return "", err
	}
	buf := bytes.NewBuffer(nil)
	err = tpl.Execute(buf, config)
	if err != nil {
		return "", err
	}
	return buf.String(), nil
}

type ConfigBuilder struct {
	RefReader ConfigRefReader
}

func (b *ConfigBuilder) configPart(instance v1alpha1.RpaasInstance, config v1alpha1.RpaasPlanSpec, blockType v1alpha1.BlockType) (string, error) {
	blockTemplateValue, _ := defaultBlocks[blockType]
	var err error
	configRef, ok := instance.Spec.Blocks[blockType]
	if ok {
		blockTemplateValue, err = b.RefReader.ReadConfigRef(configRef, instance.Namespace)
		if err != nil {
			return "", err
		}
	}
	rendered, err := renderTemplate(string(blockType), blockTemplateValue, config)
	if err != nil {
		return "", err
	}
	return rendered, nil
}

func (b *ConfigBuilder) renderLocations(instance v1alpha1.RpaasInstance, config v1alpha1.RpaasPlanSpec) ([]string, error) {
	locStrs := make([]string, len(instance.Spec.Locations))
	for i, loc := range instance.Spec.Locations {
		refValue, err := b.RefReader.ReadConfigRef(loc.Config, instance.Namespace)
		if err != nil {
			return nil, err
		}
		locStrs[i], err = renderTemplate(loc.Config.Name, refValue, config)
		if err != nil {
			return nil, err
		}
	}
	return locStrs, nil
}

func validateConfig(config v1alpha1.RpaasPlanSpec) error {
	notEmpty := []struct {
		value, field string
	}{
		{config.Config.User, "User"},
		{config.Config.Listen, "Listen"},
		{config.Config.AdminListen, "AdminListen"},
		{config.Config.KeyZoneSize, "KeyZoneSize"},
		{config.Config.CacheInactive, "CacheInactive"},
		{config.Config.CacheSize, "CacheSize"},
	}
	for _, ne := range notEmpty {
		if ne.value == "" {
			return fmt.Errorf("invalid empty %s config", ne.field)
		}
	}
	if config.Config.WorkerProcesses <= 0 {
		return fmt.Errorf("invalid WorkerProcesses config: %d", config.Config.WorkerProcesses)
	}
	if config.Config.ListenBacklog <= 0 {
		return fmt.Errorf("invalid ListenBacklog config: %d", config.Config.ListenBacklog)
	}
	if config.Config.WorkerConnections <= 0 {
		return fmt.Errorf("invalid WorkerConnections config: %d", config.Config.WorkerConnections)
	}
	if config.Config.LoaderFiles <= 0 {
		return fmt.Errorf("invalid LoaderFiles config: %d", config.Config.LoaderFiles)
	}
	return nil
}

func (b *ConfigBuilder) Interpolate(instance v1alpha1.RpaasInstance, config v1alpha1.RpaasPlanSpec) (string, error) {
	err := validateConfig(config)
	if err != nil {
		return "", err
	}
	rootDefaultData, err := b.configPart(instance, config, v1alpha1.BlockTypeRootDefault)
	if err != nil {
		return "", err
	}
	rootData, err := b.configPart(instance, config, v1alpha1.BlockTypeRoot)
	if err != nil {
		return "", err
	}
	serverDefaultData, err := b.configPart(instance, config, v1alpha1.BlockTypeServerDefault)
	if err != nil {
		return "", err
	}
	serverData, err := b.configPart(instance, config, v1alpha1.BlockTypeServer)
	if err != nil {
		return "", err
	}
	httpDefaultData, err := b.configPart(instance, config, v1alpha1.BlockTypeHTTPDefault)
	if err != nil {
		return "", err
	}
	httpData, err := b.configPart(instance, config, v1alpha1.BlockTypeHTTP)
	if err != nil {
		return "", err
	}
	renderedAdmin, err := renderTemplate("admin-block", adminServerBlock, config)
	if err != nil {
		return "", err
	}
	renderedLocations, err := b.renderLocations(instance, config)
	if err != nil {
		return "", err
	}
	configParams := fullConfigParams{
		DefaultRootBlock:   rootDefaultData,
		RootBlock:          rootData,
		ServerDefaultBlock: serverDefaultData,
		ServerBlock:        serverData,
		HTTPDefaultBlock:   httpDefaultData,
		HTTPBlock:          httpData,
		AdminServerBlock:   renderedAdmin,
		Locations:          renderedLocations,
	}
	buf := bytes.NewBuffer(nil)
	err = fullConfigTpl.Execute(buf, configParams)
	if err != nil {
		return "", err
	}
	return buf.String(), nil
}
