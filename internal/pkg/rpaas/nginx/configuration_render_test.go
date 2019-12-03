// Copyright 2019 tsuru authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package nginx

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	nginxv1alpha1 "github.com/tsuru/nginx-operator/pkg/apis/nginx/v1alpha1"
	"github.com/tsuru/rpaas-operator/pkg/apis/extensions/v1alpha1"
)

func TestRpaasConfigurationRenderer_Render(t *testing.T) {
	tests := []struct {
		name      string
		blocks    ConfigurationBlocks
		data      ConfigurationData
		assertion func(*testing.T, string)
	}{
		{
			name: "with false values",
			data: ConfigurationData{
				Config:   &v1alpha1.NginxConfig{},
				Instance: &v1alpha1.RpaasInstance{},
			},
			assertion: func(t *testing.T, result string) {
				assert.NotRegexp(t, `user(.+);`, result)
				assert.NotRegexp(t, `worker_processes(.+);`, result)
				assert.NotRegexp(t, `worker_connections(.+);`, result)
				assert.Regexp(t, `access_log /dev/stdout combined;`, result)
				assert.Regexp(t, `error_log  /dev/stderr;`, result)
				assert.Regexp(t, `server {\n\s+listen 8800;\n\s+}\n+`, result)
				assert.Regexp(t, `server {
\s+listen 8080 default_server;\n+
\s+location = /_nginx_healthcheck {\n+
\s+access_log off;\n+
\s+default_type "text/plain";
\s+return 200 "WORKING\\n";
\s+}
\s+location / {
\s+default_type "text/plain";
\s+return 404 "instance not bound\\n";
\s+}
\s+}\n+`, result)
			},
		},
		{
			name: "with custom user, worker_processes and worker_connections",
			data: ConfigurationData{
				Config: &v1alpha1.NginxConfig{
					User:              "www-data",
					WorkerProcesses:   8,
					WorkerConnections: 8192,
				},
				Instance: &v1alpha1.RpaasInstance{},
			},
			assertion: func(t *testing.T, result string) {
				assert.Regexp(t, `user www-data;`, result)
				assert.Regexp(t, `worker_processes 8;`, result)
				assert.Regexp(t, `worker_connections 8192;`, result)
			},
		},
		{
			name: "with cache enabled",
			data: ConfigurationData{
				Config: &v1alpha1.NginxConfig{
					CacheEnabled:  v1alpha1.Bool(true),
					CachePath:     "/path/to/cache/dir",
					CacheZoneSize: "100m",
				},
				Instance: &v1alpha1.RpaasInstance{},
			},
			assertion: func(t *testing.T, result string) {
				assert.Regexp(t, `proxy_cache_path /path/to/cache/dir/nginx levels=1:2 keys_zone=rpaas:100m;`, result)
				assert.Regexp(t, `proxy_temp_path /path/to/cache/dir/nginx_tmp 1 2;`, result)
				assert.Regexp(t, `server {
\s+listen 8800;
\s+location ~ \^/purge/\(\.\+\) {
\s+proxy_cache_purge rpaas \$1\$is_args\$args;
\s+}
\s+}`, result)
			},
		},
		{
			name: "with cache enabled and custom loader_files, inactive, and max cache size",
			data: ConfigurationData{
				Config: &v1alpha1.NginxConfig{
					CacheEnabled:     v1alpha1.Bool(true),
					CachePath:        "/path/to/cache/dir",
					CacheInactive:    "12h",
					CacheLoaderFiles: 1000,
					CacheSize:        "300m",
					CacheZoneSize:    "100m",
				},
				Instance: &v1alpha1.RpaasInstance{},
			},
			assertion: func(t *testing.T, result string) {
				assert.Regexp(t, `proxy_cache_path /path/to/cache/dir/nginx levels=1:2 keys_zone=rpaas:100m inactive=12h max_size=300m loader_files=1000;`, result)
				assert.Regexp(t, `proxy_temp_path /path/to/cache/dir/nginx_tmp 1 2;`, result)
				assert.Regexp(t, `server {
\s+listen 8800;
\s+location ~ \^/purge/\(\.\+\) {
\s+proxy_cache_purge rpaas \$1\$is_args\$args;
\s+}
\s+}`, result)
			},
		},
		{
			name: "with logs to syslog server",
			data: ConfigurationData{
				Config: &v1alpha1.NginxConfig{
					SyslogEnabled:       v1alpha1.Bool(true),
					SyslogServerAddress: "syslog.server.example.com",
				},
				Instance: &v1alpha1.RpaasInstance{},
			},
			assertion: func(t *testing.T, result string) {
				assert.Regexp(t, `access_log syslog:server=syslog.server.example.com\n\s+combined;`, result)
				assert.Regexp(t, `error_log syslog:server=syslog.server.example.com;`, result)
			},
		},
		{
			name: "with logs to syslogs server and custom facility and tag",
			data: ConfigurationData{
				Config: &v1alpha1.NginxConfig{
					SyslogEnabled:       v1alpha1.Bool(true),
					SyslogServerAddress: "syslog.server.example.com",
					SyslogFacility:      "local1",
					SyslogTag:           "my-tag",
				},
				Instance: &v1alpha1.RpaasInstance{},
			},
			assertion: func(t *testing.T, result string) {
				assert.Regexp(t, `access_log syslog:server=syslog.server.example.com,facility=local1,tag=my-tag\n\s+combined;`, result)
				assert.Regexp(t, `error_log syslog:server=syslog.server.example.com,facility=local1,tag=my-tag;`, result)
			},
		},
		{
			name: "with VTS enabled",
			data: ConfigurationData{
				Config: &v1alpha1.NginxConfig{
					VTSEnabled: v1alpha1.Bool(true),
				},
				Instance: &v1alpha1.RpaasInstance{},
			},
			assertion: func(t *testing.T, result string) {
				assert.Regexp(t, `vhost_traffic_status_zone;`, result)
				assert.Regexp(t, `\s+location /status {
\s+vhost_traffic_status_bypass_limit on;
\s+vhost_traffic_status_bypass_stats on;
\s+vhost_traffic_status_display;
\s+vhost_traffic_status_display_format prometheus;
\s+}`, result)
			},
		},
		{
			name: "with VTS enabled and custom vhost_traffic_histogram_buckets",
			data: ConfigurationData{
				Config: &v1alpha1.NginxConfig{
					VTSEnabled:                v1alpha1.Bool(true),
					VTSStatusHistogramBuckets: "0.001 0.005 0.01 0.025 0.05 0.1 0.25 0.5 1 2.5 5 10",
				},
				Instance: &v1alpha1.RpaasInstance{},
			},
			assertion: func(t *testing.T, result string) {
				assert.Regexp(t, `vhost_traffic_status_zone;`, result)
				assert.Regexp(t, `vhost_traffic_status_histogram_buckets 0.001 0.005 0.01 0.025 0.05 0.1 0.25 0.5 1 2.5 5 10;`, result)
				assert.Regexp(t, `\s+location /status {
\s+vhost_traffic_status_bypass_limit on;
\s+vhost_traffic_status_bypass_stats on;
\s+vhost_traffic_status_display;
\s+vhost_traffic_status_display_format prometheus;
\s+}`, result)
			},
		},
		{
			name: "with TLS actived",
			data: ConfigurationData{
				Config: &v1alpha1.NginxConfig{},
				Instance: &v1alpha1.RpaasInstance{
					Spec: v1alpha1.RpaasInstanceSpec{
						Certificates: &nginxv1alpha1.TLSSecret{
							SecretName: "secret-name",
							Items: []nginxv1alpha1.TLSSecretItem{
								{
									CertificateField: "default.crt",
									KeyField:         "default.key",
								},
							},
						},
					},
				},
			},
			assertion: func(t *testing.T, result string) {
				assert.Regexp(t, `listen 8443 default_server ssl http2;`, result)
				assert.Regexp(t, `ssl_certificate\s+certs/default.crt;`, result)
				assert.Regexp(t, `ssl_certificate_key certs/default.key;`, result)
			},
		},
		{
			name: "with TLS actived and custom listen options",
			data: ConfigurationData{
				Config: &v1alpha1.NginxConfig{
					HTTPSListenOptions: "backlog=2048 deferred reuseport",
				},
				Instance: &v1alpha1.RpaasInstance{
					Spec: v1alpha1.RpaasInstanceSpec{
						Certificates: &nginxv1alpha1.TLSSecret{
							SecretName: "secret-name",
							Items: []nginxv1alpha1.TLSSecretItem{
								{
									CertificateField: "default.crt",
									CertificatePath:  "custom_certificate_name.crt",
									KeyField:         "default.key",
									KeyPath:          "custom_key_name.key",
								},
							},
						},
					},
				},
			},
			assertion: func(t *testing.T, result string) {
				assert.Regexp(t, `listen 8443 default_server ssl http2 backlog=2048 deferred reuseport;`, result)
				assert.Regexp(t, `ssl_certificate\s+certs/custom_certificate_name.crt;`, result)
				assert.Regexp(t, `ssl_certificate_key certs/custom_key_name.key;`, result)
			},
		},
		{
			name: "with custom config blocks",
			blocks: ConfigurationBlocks{
				RootBlock:      `# some custom conf at {{ "root" }} context`,
				HttpBlock:      "# some custom conf at http context",
				ServerBlock:    "# some custom conf at server context",
				LuaServerBlock: "# some custom conf at init_by_lua_block context",
				LuaWorkerBlock: "# some custom conf at init_worker_by_lua_block context",
			},
			data: ConfigurationData{
				Config:   &v1alpha1.NginxConfig{},
				Instance: &v1alpha1.RpaasInstance{},
			},
			assertion: func(t *testing.T, result string) {
				assert.Regexp(t, `# some custom conf at root context`, result)
				assert.Regexp(t, `# some custom conf at http context`, result)
				assert.Regexp(t, `# some custom conf at server context`, result)
				assert.Regexp(t, `# some custom conf at init_by_lua_block context`, result)
				assert.Regexp(t, `# some custom conf at init_worker_by_lua_block context`, result)
			},
		},
		{
			name: "with app bound",
			data: ConfigurationData{
				Config: &v1alpha1.NginxConfig{},
				Instance: &v1alpha1.RpaasInstance{
					Spec: v1alpha1.RpaasInstanceSpec{
						Host: "app1.tsuru.example.com",
					},
				},
			},
			assertion: func(t *testing.T, result string) {
				assert.Regexp(t, `upstream rpaas_default_upstream {
\s+server app1.tsuru.example.com;
\s+}`, result)
				assert.Regexp(t, `location / {
\s+proxy_set_header Connection "";
\s+proxy_set_header Host app1.tsuru.example.com;

\s+proxy_pass http://rpaas_default_upstream/;
\s+proxy_redirect ~\^http://rpaas_default_upstream\(:\\d\+\)\?/\(\.\*\)\$ /\$2;
\s+}`, result)
			},
		},
		{
			name: "with app bound + keepalive",
			data: ConfigurationData{
				Config: &v1alpha1.NginxConfig{
					UpstreamKeepalive: 64,
				},
				Instance: &v1alpha1.RpaasInstance{
					Spec: v1alpha1.RpaasInstanceSpec{
						Host: "app1.tsuru.example.com",
					},
				},
			},
			assertion: func(t *testing.T, result string) {
				assert.Regexp(t, `upstream rpaas_default_upstream {
\s+server app1.tsuru.example.com;
\s+keepalive 64;
\s+}`, result)
				assert.Regexp(t, `location / {
\s+proxy_set_header Connection "";
\s+proxy_set_header Host app1.tsuru.example.com;

\s+proxy_pass http://rpaas_default_upstream/;
\s+proxy_redirect ~\^http://rpaas_default_upstream\(:\\d\+\)\?/\(\.\*\)\$ /\$2;
\s+}`, result)
			},
		},
		{
			name: "with paths (destination and custom configs) + keepalive",
			data: ConfigurationData{
				Config: &v1alpha1.NginxConfig{
					UpstreamKeepalive: 128,
				},
				Instance: &v1alpha1.RpaasInstance{
					Spec: v1alpha1.RpaasInstanceSpec{
						Locations: []v1alpha1.Location{
							{
								Path:        "/path1",
								Destination: "app1.tsuru.example.com",
							},
							{
								Path:        "/path2",
								Destination: "app2.tsuru.example.com",
								ForceHTTPS:  true,
							},
							{
								Path: "/path3",
								Content: &v1alpha1.Value{
									Value: "# My custom configuration for /path3",
								},
							},
						},
					},
				},
			},
			assertion: func(t *testing.T, result string) {
				assert.Regexp(t, `upstream rpaas_locations__path1 {
\s+server app1\.tsuru\.example\.com;
\s+keepalive 128;
\s+}`, result)
				assert.Regexp(t, `upstream rpaas_locations__path2 {
\s+server app2\.tsuru\.example\.com;
\s+keepalive 128;
\s+}`, result)
				assert.Regexp(t, `location /path1 {\n+
\s+proxy_set_header Connection "";
\s+proxy_set_header Host app1\.tsuru\.example\.com;

\s+proxy_pass http://rpaas_locations__path1/;
\s+proxy_redirect ~\^http://rpaas_locations__path1\(:\\d\+\)\?/\(\.\*\)\$ /path1\$2;
\s+}`, result)
				assert.Regexp(t, `location /path2 {
\s+if \(\$scheme = 'http'\) {
\s+return 301 https://\$http_host\$request_uri;
\s+}

\s+proxy_set_header Connection "";
\s+proxy_set_header Host app2\.tsuru\.example\.com;

\s+proxy_pass http://rpaas_locations__path2/;
\s+proxy_redirect ~\^http://rpaas_locations__path2\(:\\d\+\)\?/\(\.\*\)\$ /path2\$2;
\s+}`, result)
				assert.Regexp(t, `location /path3 {
\s+# My custom configuration for /path3
\s+}`, result)
			},
		},
		{
			name: "with custom NGINX config template",
			blocks: ConfigurationBlocks{
				MainBlock: "# My custom main NGINX template.\nuser {{ .Config.User }};\n...",
			},
			data: ConfigurationData{
				Config: &v1alpha1.NginxConfig{
					User: "my-user",
				},
				Instance: &v1alpha1.RpaasInstance{},
			},
			assertion: func(t *testing.T, result string) {
				assert.Regexp(t, "# My custom main NGINX template.\nuser my-user;\n...", result)
			},
		},
		{
			name: "with pod using the host network",
			data: ConfigurationData{
				Config: &v1alpha1.NginxConfig{},
				Instance: &v1alpha1.RpaasInstance{
					Spec: v1alpha1.RpaasInstanceSpec{
						Certificates: &nginxv1alpha1.TLSSecret{
							SecretName: "secret-name",
							Items: []nginxv1alpha1.TLSSecretItem{
								{
									CertificateField: "default.crt",
									CertificatePath:  "custom_certificate_name.crt",
									KeyField:         "default.key",
									KeyPath:          "custom_key_name.key",
								},
							},
						},
						PodTemplate: nginxv1alpha1.NginxPodTemplateSpec{
							HostNetwork: true,
						},
					},
				},
			},
			assertion: func(t *testing.T, result string) {
				assert.Regexp(t, `listen 80 default_server;`, result)
				assert.Regexp(t, `listen 443 default_server ssl http2;`, result)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cr, err := NewConfigurationRenderer(tt.blocks)
			require.NoError(t, err)
			result, err := cr.Render(tt.data)
			require.NoError(t, err)
			tt.assertion(t, result)
		})
	}
}

func Test_buildLocationKey(t *testing.T) {
	tests := []struct {
		name        string
		prefix      string
		path        string
		expected    string
		shouldPanic bool
	}{
		{
			name:     "when using a custom prefix and root path",
			prefix:   "some_custom_prefix_",
			path:     "/",
			expected: "some_custom_prefix_root",
		},
		{
			name:     "when using a custom prefix and non-root path",
			prefix:   "some_custom_prefix_",
			path:     "/just/another/path",
			expected: "some_custom_prefix__just_another_path",
		},
		{
			name:     "when using the default prefix and root path",
			path:     "/",
			expected: "rpaas_locations_root",
		},
		{
			name:     "when using the default prefix and non-root path",
			path:     "/custom/path",
			expected: "rpaas_locations__custom_path",
		},
		{
			name:        "when using the default prefix with no path",
			expected:    "cannot build location key due path is missing",
			shouldPanic: true,
		},
		{
			name:        "when using a custom prefix with no path",
			prefix:      "some_custom_prefix_",
			expected:    "cannot build location key due path is missing",
			shouldPanic: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var got string
			if !tt.shouldPanic {
				got = buildLocationKey(tt.prefix, tt.path)
				assert.Equal(t, tt.expected, got)
			} else {
				assert.PanicsWithValue(t, tt.expected, func() { buildLocationKey(tt.prefix, tt.path) })
			}
		})
	}
}

func Test_hasRootPath(t *testing.T) {
	tests := []struct {
		name      string
		locations []v1alpha1.Location
		expected  bool
	}{
		{
			name:     "when locations is nil",
			expected: false,
		},
		{
			name: "when locations has no root path",
			locations: []v1alpha1.Location{
				{
					Path:        "/path1",
					Destination: "app.tsuru.example.com",
				},
			},
		},
		{
			name: "when locations has the root path",
			locations: []v1alpha1.Location{
				{
					Path:        "/path1",
					Destination: "app.tsuru.example.com",
				},
				{
					Path:        "/",
					Destination: "app.tsuru.example.com",
				},
			},
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := hasRootPath(tt.locations)
			assert.Equal(t, tt.expected, got)
		})
	}
}
