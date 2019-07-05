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
		renderer  ConfigurationRenderer
		data      ConfigurationData
		assertion func(*testing.T, string, error)
	}{
		{
			renderer: NewRpaasConfigurationRenderer(ConfigurationBlocks{}),
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
				assert.Regexp(t, `location = /_nginx_healthcheck/ {\n\s+default_type "text/plain";\n\s+echo "WORKING";\n\s+}`, result)
				assert.Regexp(t, `location / {\n\s+default_type "text/plain";\n\s+echo "instance not bound yet";\n\s+}`, result)
			},
		},
		{
			renderer: NewRpaasConfigurationRenderer(ConfigurationBlocks{}),
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
			renderer: NewRpaasConfigurationRenderer(ConfigurationBlocks{}),
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
			renderer: NewRpaasConfigurationRenderer(ConfigurationBlocks{}),
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
			renderer: NewRpaasConfigurationRenderer(ConfigurationBlocks{}),
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
			renderer: NewRpaasConfigurationRenderer(ConfigurationBlocks{}),
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
			renderer: NewRpaasConfigurationRenderer(ConfigurationBlocks{}),
			data: ConfigurationData{
				Config: &v1alpha1.NginxConfig{},
				Instance: &v1alpha1.RpaasInstanceSpec{
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
			assertion: func(t *testing.T, result string, err error) {
				require.NoError(t, err)
				assert.Regexp(t, `listen 8443 ssl;`, result)
				assert.Regexp(t, `ssl_certificate\s+certs/default.crt;`, result)
				assert.Regexp(t, `ssl_certificate_key certs/default.key;`, result)
				assert.Regexp(t, `ssl_protocols TLSv1 TLSv1.1 TLSv1.2;`, result)
				assert.Regexp(t, `ssl_ciphers 'ECDHE-ECDSA-CHACHA20-POLY1305:ECDHE-RSA-CHACHA20-POLY1305:ECDHE-ECDSA-AES128-GCM-SHA256:ECDHE-RSA-AES128-GCM-SHA256:ECDHE-ECDSA-AES256-GCM-SHA384:ECDHE-RSA-AES256-GCM-SHA384:DHE-RSA-AES128-GCM-SHA256:DHE-RSA-AES256-GCM-SHA384:ECDHE-ECDSA-AES128-SHA256:ECDHE-RSA-AES128-SHA256:ECDHE-ECDSA-AES128-SHA:ECDHE-RSA-AES256-SHA384:ECDHE-RSA-AES128-SHA:ECDHE-ECDSA-AES256-SHA384:ECDHE-ECDSA-AES256-SHA:ECDHE-RSA-AES256-SHA:DHE-RSA-AES128-SHA256:DHE-RSA-AES128-SHA:DHE-RSA-AES256-SHA256:DHE-RSA-AES256-SHA:ECDHE-ECDSA-DES-CBC3-SHA:ECDHE-RSA-DES-CBC3-SHA:EDH-RSA-DES-CBC3-SHA:AES128-GCM-SHA256:AES256-GCM-SHA384:AES128-SHA256:AES256-SHA256:AES128-SHA:AES256-SHA:DES-CBC3-SHA:!DSS';`, result)
				assert.Regexp(t, `ssl_prefer_server_ciphers on;`, result)
				assert.Regexp(t, `ssl_session_cache shared:SSL:200m;`, result)
				assert.Regexp(t, `ssl_session_timeout 1h;`, result)
			},
		},
		{
			renderer: NewRpaasConfigurationRenderer(ConfigurationBlocks{}),
			data: ConfigurationData{
				Config: &v1alpha1.NginxConfig{
					HTTPSListenOptions: "http2",
				},
				Instance: &v1alpha1.RpaasInstanceSpec{
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
			assertion: func(t *testing.T, result string, err error) {
				require.NoError(t, err)
				assert.Regexp(t, `listen 8443 ssl http2;`, result)
				assert.Regexp(t, `ssl_certificate\s+certs/custom_certificate_name.crt;`, result)
				assert.Regexp(t, `ssl_certificate_key certs/custom_key_name.key;`, result)
				assert.Regexp(t, `ssl_protocols TLSv1 TLSv1.1 TLSv1.2;`, result)
				assert.Regexp(t, `ssl_ciphers 'ECDHE-ECDSA-CHACHA20-POLY1305:ECDHE-RSA-CHACHA20-POLY1305:ECDHE-ECDSA-AES128-GCM-SHA256:ECDHE-RSA-AES128-GCM-SHA256:ECDHE-ECDSA-AES256-GCM-SHA384:ECDHE-RSA-AES256-GCM-SHA384:DHE-RSA-AES128-GCM-SHA256:DHE-RSA-AES256-GCM-SHA384:ECDHE-ECDSA-AES128-SHA256:ECDHE-RSA-AES128-SHA256:ECDHE-ECDSA-AES128-SHA:ECDHE-RSA-AES256-SHA384:ECDHE-RSA-AES128-SHA:ECDHE-ECDSA-AES256-SHA384:ECDHE-ECDSA-AES256-SHA:ECDHE-RSA-AES256-SHA:DHE-RSA-AES128-SHA256:DHE-RSA-AES128-SHA:DHE-RSA-AES256-SHA256:DHE-RSA-AES256-SHA:ECDHE-ECDSA-DES-CBC3-SHA:ECDHE-RSA-DES-CBC3-SHA:EDH-RSA-DES-CBC3-SHA:AES128-GCM-SHA256:AES256-GCM-SHA384:AES128-SHA256:AES256-SHA256:AES128-SHA:AES256-SHA:DES-CBC3-SHA:!DSS';`, result)
				assert.Regexp(t, `ssl_prefer_server_ciphers on;`, result)
				assert.Regexp(t, `ssl_session_cache shared:SSL:200m;`, result)
				assert.Regexp(t, `ssl_session_timeout 1h;`, result)
			},
		},
		{
			renderer: NewRpaasConfigurationRenderer(ConfigurationBlocks{
				RootBlock:   "# some custom conf at root context",
				HttpBlock:   "# some custom conf at http context",
				ServerBlock: "# some custom conf at server context",
			}),
			data: ConfigurationData{
				Config:   &v1alpha1.NginxConfig{},
				Instance: &v1alpha1.RpaasInstanceSpec{},
			},
			assertion: func(t *testing.T, result string, err error) {
				require.NoError(t, err)
				assert.Regexp(t, `# some custom conf at root context`, result)
				assert.Regexp(t, `# some custom conf at http context`, result)
				assert.Regexp(t, `# some custom conf at server context`, result)
			},
		},
		{
			renderer: NewRpaasConfigurationRenderer(ConfigurationBlocks{
				RootBlock: "# I can use any block as a golang template: {{.Config.User}};",
			}),
			data: ConfigurationData{
				Config: &v1alpha1.NginxConfig{
					User: "another-user",
				},
				Instance: &v1alpha1.RpaasInstanceSpec{},
			},
			assertion: func(t *testing.T, result string, err error) {
				require.NoError(t, err)
				assert.Regexp(t, `# I can use any block as a golang template: another-user;`, result)
			},
		},
		{
			renderer: NewRpaasConfigurationRenderer(ConfigurationBlocks{}),
			data: ConfigurationData{
				Config: &v1alpha1.NginxConfig{},
				Instance: &v1alpha1.RpaasInstanceSpec{
					Host: "app1.tsuru.example.com",
				},
			},
			assertion: func(t *testing.T, result string, err error) {
				assert.NoError(t, err)
				assert.Regexp(t, `upstream rpaas_backend_default {\n\s+server app1.tsuru.example.com;\n\s+}`, result)
				assert.Regexp(t, `location / {\n\s+proxy_set_header Host app1.tsuru.example.com;\n\s+proxy_set_header X-Real-IP \$remote_addr;\n\s+proxy_set_header X-Forwarded-For \$proxy_add_x_forwarded_for;\n\s+proxy_set_header X-Forwarded-Proto \$scheme;\n\s+proxy_set_header X-Forwarded-Host \$host;\n+\s+proxy_pass http://rpaas_backend_default;\n\s+}`, result)
			},
		},
		{
			renderer: NewRpaasConfigurationRenderer(ConfigurationBlocks{}),
			data: ConfigurationData{
				Config: &v1alpha1.NginxConfig{},
				Instance: &v1alpha1.RpaasInstanceSpec{
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
							Path:  "/path3",
							Value: "# My custom configuration for /path3",
						},
					},
				},
			},
			assertion: func(t *testing.T, result string, err error) {
				assert.NoError(t, err)
				assert.Regexp(t, `location /path1 {\n+
\s+proxy_set_header Host app1\.tsuru\.example\.com;
\s+proxy_set_header X-Real-IP \$remote_addr;
\s+proxy_set_header X-Forwarded-For \$proxy_add_x_forwarded_for;
\s+proxy_set_header X-Forwarded-Proto \$scheme;
\s+proxy_set_header X-Forwarded-Host \$host;
\s+proxy_set_header Connection "";
\s+proxy_http_version 1.1;
\s+proxy_pass http://app1.tsuru.example.com/;
\s+proxy_redirect ~\^http://app1\.tsuru\.example\.com\(:\\d\+\)\?/\(\.\*\)\$ /path1\$2;\n+
\s+}`, result)

				assert.Regexp(t, `location /path2 {\n+
\s+if \(\$scheme = 'http'\) {
\s+return 301 https://\$http_host\$request_uri;
\s+}\n+
\s+proxy_set_header Host app2\.tsuru\.example\.com;
\s+proxy_set_header X-Real-IP \$remote_addr;
\s+proxy_set_header X-Forwarded-For \$proxy_add_x_forwarded_for;
\s+proxy_set_header X-Forwarded-Proto \$scheme;
\s+proxy_set_header X-Forwarded-Host \$host;
\s+proxy_set_header Connection "";
\s+proxy_http_version 1.1;
\s+proxy_pass http://app2.tsuru.example.com/;
\s+proxy_redirect ~\^http://app2\.tsuru\.example\.com\(:\\d\+\)\?/\(\.\*\)\$ /path2\$2;\n+
\s+}`, result)

				assert.Regexp(t, `location /path3 {\n+
\s+# My custom configuration for /path3\n+
\s+}`, result)
			},
		},
	}

	for _, testCase := range testCases {
		t.Run("", func(t *testing.T) {
			result, err := testCase.renderer.Render(testCase.data)
			testCase.assertion(t, result, err)
		})
	}
}
