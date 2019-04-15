package nginx

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	nginxv1alpha1 "github.com/tsuru/nginx-operator/pkg/apis/nginx/v1alpha1"
	"github.com/tsuru/rpaas-operator/pkg/apis/extensions/v1alpha1"
)

func TestRpaasConfigurationRenderer_Render(t *testing.T) {
	testCases := []struct {
		data      ConfigurationData
		assertion func(*testing.T, string, error)
	}{
		{
			data: ConfigurationData{
				Config:   &v1alpha1.NginxConfig{},
				Instance: &v1alpha1.RpaasInstanceSpec{},
			},
			assertion: func(t *testing.T, result string, err error) {
				require.NoError(t, err)
				assert.Regexp(t, `user nginx;`, result)
				assert.Regexp(t, `worker_processes 1;`, result)
				assert.Regexp(t, `worker_connections 1024;`, result)
				assert.Regexp(t, `access_log /dev/stdout rpaas_combined;`, result)
				assert.Regexp(t, `error_log  /dev/stderr;`, result)
				assert.Regexp(t, `listen 8080 default_server;`, result)
			},
		},
		{
			data: ConfigurationData{
				Config: &v1alpha1.NginxConfig{
					RequestIDEnabled: true,
				},
				Instance: &v1alpha1.RpaasInstanceSpec{},
			},
			assertion: func(t *testing.T, result string, err error) {
				require.NoError(t, err)
				assert.Regexp(t, `uuid4 \$request_id_uuid;`, result)
				assert.Regexp(t, `map \$http_x_request_id \$request_id_final {\n\s+default \$request_id_uuid;\n\s+"~\."\s+\$http_x_request_id;\n\s*}`, result)
			},
		},
		{
			data: ConfigurationData{
				Config: &v1alpha1.NginxConfig{
					CacheEnabled:     true,
					CachePath:        "/path/to/cache/dir",
					CacheInactive:    "12h",
					CacheLoaderFiles: 1000,
					CacheSize:        "300m",
					CacheZoneSize:    "100m",
				},
				Instance: &v1alpha1.RpaasInstanceSpec{},
			},
			assertion: func(t *testing.T, result string, err error) {
				require.NoError(t, err)
				assert.Regexp(t, `proxy_cache_path /path/to/cache/dir/nginx levels=1:2 keys_zone=rpaas:100m inactive=12h max_size=300m loader_files=1000;`, result)
				assert.Regexp(t, `proxy_temp_path  /path/to/cache/dir/nginx_temp 1 2;`, result)
				assert.Regexp(t, `proxy_cache rpaas;`, result)
				assert.Regexp(t, `proxy_cache_use_stale error timeout updating invalid_header http_500 http_502 http_503 http_504;`, result)
				assert.Regexp(t, `proxy_cache_lock on;`, result)
				assert.Regexp(t, `proxy_cache_lock_age 60s;`, result)
				assert.Regexp(t, `proxy_cache_lock_timeout 60s;`, result)
			},
		},
		{
			data: ConfigurationData{
				Config: &v1alpha1.NginxConfig{
					SyslogEnabled:       true,
					SyslogServerAddress: "syslog.server.example.com",
				},
				Instance: &v1alpha1.RpaasInstanceSpec{},
			},
			assertion: func(t *testing.T, result string, err error) {
				require.NoError(t, err)
				assert.Regexp(t, `access_log syslog:server=syslog.server.example.com,facility=local6,tag=rpaas rpaas_combined;`, result)
				assert.Regexp(t, `error_log syslog:server=syslog.server.example.com,facility=local6,tag=rpaas;`, result)
			},
		},
		{
			data: ConfigurationData{
				Config: &v1alpha1.NginxConfig{
					SyslogEnabled:       true,
					SyslogServerAddress: "syslog.server.example.com",
					SyslogFacility:      "local1",
					SyslogTag:           "my-tag",
				},
				Instance: &v1alpha1.RpaasInstanceSpec{},
			},
			assertion: func(t *testing.T, result string, err error) {
				require.NoError(t, err)
				assert.Regexp(t, `access_log syslog:server=syslog.server.example.com,facility=local1,tag=my-tag rpaas_combined;`, result)
				assert.Regexp(t, `error_log syslog:server=syslog.server.example.com,facility=local1,tag=my-tag;`, result)
			},
		},
		{
			data: ConfigurationData{
				Config: &v1alpha1.NginxConfig{
					VTSEnabled: true,
				},
				Instance: &v1alpha1.RpaasInstanceSpec{},
			},
			assertion: func(t *testing.T, result string, err error) {
				require.NoError(t, err)
				assert.Regexp(t, `vhost_traffic_status_zone;`, result)
			},
		},
		{
			data: ConfigurationData{
				Config: &v1alpha1.NginxConfig{},
				Instance: &v1alpha1.RpaasInstanceSpec{
					Certificates: map[string]nginxv1alpha1.TLSSecret{
						"default": nginxv1alpha1.TLSSecret{
							CertificatePath: "default.crt.pem",
							KeyPath:         "default.key.pem",
						},
					},
				},
			},
			assertion: func(t *testing.T, result string, err error) {
				require.NoError(t, err)
				assert.Regexp(t, `listen 8443 ssl;`, result)
				assert.Regexp(t, `ssl_certificate\s+certs/default.crt.pem;`, result)
				assert.Regexp(t, `ssl_certificate_key certs/default.key.pem;`, result)
				assert.Regexp(t, `ssl_protocols TLSv1 TLSv1.1 TLSv1.2;`, result)
				assert.Regexp(t, `ssl_ciphers 'ECDHE-ECDSA-CHACHA20-POLY1305:ECDHE-RSA-CHACHA20-POLY1305:ECDHE-ECDSA-AES128-GCM-SHA256:ECDHE-RSA-AES128-GCM-SHA256:ECDHE-ECDSA-AES256-GCM-SHA384:ECDHE-RSA-AES256-GCM-SHA384:DHE-RSA-AES128-GCM-SHA256:DHE-RSA-AES256-GCM-SHA384:ECDHE-ECDSA-AES128-SHA256:ECDHE-RSA-AES128-SHA256:ECDHE-ECDSA-AES128-SHA:ECDHE-RSA-AES256-SHA384:ECDHE-RSA-AES128-SHA:ECDHE-ECDSA-AES256-SHA384:ECDHE-ECDSA-AES256-SHA:ECDHE-RSA-AES256-SHA:DHE-RSA-AES128-SHA256:DHE-RSA-AES128-SHA:DHE-RSA-AES256-SHA256:DHE-RSA-AES256-SHA:ECDHE-ECDSA-DES-CBC3-SHA:ECDHE-RSA-DES-CBC3-SHA:EDH-RSA-DES-CBC3-SHA:AES128-GCM-SHA256:AES256-GCM-SHA384:AES128-SHA256:AES256-SHA256:AES128-SHA:AES256-SHA:DES-CBC3-SHA:!DSS';`, result)
				assert.Regexp(t, `ssl_prefer_server_ciphers on;`, result)
				assert.Regexp(t, `ssl_session_cache shared:SSL:200m;`, result)
				assert.Regexp(t, `ssl_session_timeout 1h;`, result)
			},
		},
		{
			data: ConfigurationData{
				Config: &v1alpha1.NginxConfig{
					HTTPSListenOptions: "http2",
				},
				Instance: &v1alpha1.RpaasInstanceSpec{
					Certificates: map[string]nginxv1alpha1.TLSSecret{
						"default": nginxv1alpha1.TLSSecret{
							CertificatePath: "default.crt.pem",
							KeyPath:         "default.key.pem",
						},
					},
				},
			},
			assertion: func(t *testing.T, result string, err error) {
				require.NoError(t, err)
				assert.Regexp(t, `listen 8443 ssl http2;`, result)
				assert.Regexp(t, `ssl_certificate\s+certs/default.crt.pem;`, result)
				assert.Regexp(t, `ssl_certificate_key certs/default.key.pem;`, result)
				assert.Regexp(t, `ssl_protocols TLSv1 TLSv1.1 TLSv1.2;`, result)
				assert.Regexp(t, `ssl_ciphers 'ECDHE-ECDSA-CHACHA20-POLY1305:ECDHE-RSA-CHACHA20-POLY1305:ECDHE-ECDSA-AES128-GCM-SHA256:ECDHE-RSA-AES128-GCM-SHA256:ECDHE-ECDSA-AES256-GCM-SHA384:ECDHE-RSA-AES256-GCM-SHA384:DHE-RSA-AES128-GCM-SHA256:DHE-RSA-AES256-GCM-SHA384:ECDHE-ECDSA-AES128-SHA256:ECDHE-RSA-AES128-SHA256:ECDHE-ECDSA-AES128-SHA:ECDHE-RSA-AES256-SHA384:ECDHE-RSA-AES128-SHA:ECDHE-ECDSA-AES256-SHA384:ECDHE-ECDSA-AES256-SHA:ECDHE-RSA-AES256-SHA:DHE-RSA-AES128-SHA256:DHE-RSA-AES128-SHA:DHE-RSA-AES256-SHA256:DHE-RSA-AES256-SHA:ECDHE-ECDSA-DES-CBC3-SHA:ECDHE-RSA-DES-CBC3-SHA:EDH-RSA-DES-CBC3-SHA:AES128-GCM-SHA256:AES256-GCM-SHA384:AES128-SHA256:AES256-SHA256:AES128-SHA:AES256-SHA:DES-CBC3-SHA:!DSS';`, result)
				assert.Regexp(t, `ssl_prefer_server_ciphers on;`, result)
				assert.Regexp(t, `ssl_session_cache shared:SSL:200m;`, result)
				assert.Regexp(t, `ssl_session_timeout 1h;`, result)
			},
		},
	}

	for _, testCase := range testCases {
		t.Run("", func(t *testing.T) {
			configRenderer := NewRpaasConfigurationRenderer()
			result, err := configRenderer.Render(testCase.data)
			testCase.assertion(t, result, err)
		})
	}
}
