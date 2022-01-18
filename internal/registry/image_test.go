// Copyright 2022 tsuru authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package registry_test

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/tsuru/rpaas-operator/internal/registry"
)

func TestImageMetadataRetriever_Modules(t *testing.T) {
	var apiCalls int

	tests := map[string]struct {
		handler  http.Handler
		image    string
		expected []string
	}{
		"getting labels from image without tag identifier": {
			handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				defer func() { apiCalls++ }()

				w.Header().Set("Content-Type", "application/json")

				if apiCalls == 0 && r.Method == "GET" && r.URL.Path == "/v2/" {
					fmt.Fprintf(w, `{}`)
					return
				}

				if apiCalls == 1 && r.Method == "GET" && r.URL.Path == "/v2/tsuru/nginx-tsuru/manifests/latest" {
					fmt.Fprintf(w, `{"mediaType": "application/vnd.docker.distribution.manifest.v2+json", "schemaVersion": 2, "config": {"mediaType": "application/vnd.docker.container.image.v1+json", "digest": "sha256:7eb99f285b03ef54e2d47e69747f60b2e5c407d394b4c80c31d54d21d377cff3", "size": 65}}`)
					return
				}

				if apiCalls == 2 && r.Method == "GET" && r.URL.Path == "/v2/tsuru/nginx-tsuru/blobs/sha256:7eb99f285b03ef54e2d47e69747f60b2e5c407d394b4c80c31d54d21d377cff3" {
					fmt.Fprintf(w, `{"config": {"Labels": {"io.tsuru.nginx-modules": "foo,bar,baz"}}}`)
					return
				}

				w.WriteHeader(http.StatusInternalServerError)
			}),
			image: "tsuru/nginx-tsuru",
			expected: []string{
				"foo",
				"bar",
				"baz",
			},
		},

		"getting labels from image w/ specific tag version": {
			handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				defer func() { apiCalls++ }()

				w.Header().Set("Content-Type", "application/json")

				if apiCalls == 0 && r.Method == "GET" && r.URL.Path == "/v2/" {
					fmt.Fprintf(w, `{}`)
					return
				}

				if apiCalls == 1 && r.Method == "GET" && r.URL.Path == "/v2/tsuru/nginx-tsuru/manifests/1.20.1-0.7.0" {
					fmt.Fprintf(w, `{"mediaType": "application/vnd.docker.distribution.manifest.v2+json", "schemaVersion": 2, "config": {"mediaType": "application/vnd.docker.container.image.v1+json", "digest": "sha256:ba558c72b6e5a2c8aa1abbd86ee4a8ee3e3e2956dac8095c3f4aac194e0314ad", "size": 10512}}`)
					return
				}

				if apiCalls == 2 && r.Method == "GET" && r.URL.Path == "/v2/tsuru/nginx-tsuru/blobs/sha256:ba558c72b6e5a2c8aa1abbd86ee4a8ee3e3e2956dac8095c3f4aac194e0314ad" {
					fmt.Fprintf(w, `{"architecture":"amd64","config":{"User":"nginx","ExposedPorts":{"80/tcp":{},"8080/tcp":{},"8443/tcp":{}},"Env":["PATH=/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin","NGINX_VERSION=1.20.1","NJS_VERSION=0.5.3","PKG_RELEASE=1~buster","LUAJIT_LIB=/usr/local/lib","LUAJIT_INC=/usr/local/include/luajit-2.1"],"Entrypoint":["/docker-entrypoint.sh"],"Cmd":["nginx","-g","daemon off;"],"WorkingDir":"/etc/nginx","Labels":{"io.tsuru.nginx-modules":"ndk_http_module,ngx_http_cache_purge_module,ngx_http_dav_ext_module,ngx_http_echo_module,ngx_http_fancyindex_module,ngx_http_geoip2_module,ngx_http_geoip_module,ngx_http_headers_more_filter_module,ngx_http_image_filter_module,ngx_http_js_module,ngx_http_location_name_module,ngx_http_lua_module,ngx_http_lua_ssl_module,ngx_http_modsecurity_module,ngx_http_push_stream_module,ngx_http_subs_filter_module,ngx_http_uploadprogress_module,ngx_http_vhost_traffic_status_module,ngx_http_xslt_filter_module,ngx_stream_geoip2_module,ngx_stream_geoip_module,ngx_stream_js_module","maintainer":"NGINX Docker Maintainers \u003cdocker-maint@nginx.com\u003e"},"StopSignal":"SIGQUIT","OnBuild":null},"created":"2021-09-24T18:08:36.729792877Z","history":[{"created":"2021-09-03T01:21:46.511313656Z","created_by":"/bin/sh -c #(nop) ADD file:4ff85d9f6aa246746912db62dea02eb71750474bb29611e770516a1fcd217add in / "},{"created":"2021-09-03T01:21:46.935145833Z","created_by":"/bin/sh -c #(nop)  CMD [\"bash\"]","empty_layer":true},{"created":"2021-09-03T07:39:35.716288163Z","created_by":"/bin/sh -c #(nop)  LABEL maintainer=NGINX Docker Maintainers \u003cdocker-maint@nginx.com\u003e","empty_layer":true},{"created":"2021-09-03T07:41:26.168778474Z","created_by":"/bin/sh -c #(nop)  ENV NGINX_VERSION=1.20.1","empty_layer":true},{"created":"2021-09-03T07:41:26.450332701Z","created_by":"/bin/sh -c #(nop)  ENV NJS_VERSION=0.5.3","empty_layer":true},{"created":"2021-09-03T07:41:26.891349489Z","created_by":"/bin/sh -c #(nop)  ENV PKG_RELEASE=1~buster","empty_layer":true},{"created":"2021-09-03T07:42:02.599948668Z","created_by":"/bin/sh -c set -x     \u0026\u0026 addgroup --system --gid 101 nginx     \u0026\u0026 adduser --system --disabled-login --ingroup nginx --no-create-home --home /nonexistent --gecos \"nginx user\" --shell /bin/false --uid 101 nginx     \u0026\u0026 apt-get update     \u0026\u0026 apt-get install --no-install-recommends --no-install-suggests -y gnupg1 ca-certificates     \u0026\u0026     NGINX_GPGKEY=573BFD6B3D8FBC641079A6ABABF5BD827BD9BF62;     found='';     for server in         ha.pool.sks-keyservers.net         hkp://keyserver.ubuntu.com:80         hkp://p80.pool.sks-keyservers.net:80         pgp.mit.edu     ; do         echo \"Fetching GPG key $NGINX_GPGKEY from $server\";         apt-key adv --keyserver \"$server\" --keyserver-options timeout=10 --recv-keys \"$NGINX_GPGKEY\" \u0026\u0026 found=yes \u0026\u0026 break;     done;     test -z \"$found\" \u0026\u0026 echo \u003e\u00262 \"error: failed to fetch GPG key $NGINX_GPGKEY\" \u0026\u0026 exit 1;     apt-get remove --purge --auto-remove -y gnupg1 \u0026\u0026 rm -rf /var/lib/apt/lists/*     \u0026\u0026 dpkgArch=\"$(dpkg --print-architecture)\"     \u0026\u0026 nginxPackages=\"         nginx=${NGINX_VERSION}-${PKG_RELEASE}         nginx-module-xslt=${NGINX_VERSION}-${PKG_RELEASE}         nginx-module-geoip=${NGINX_VERSION}-${PKG_RELEASE}         nginx-module-image-filter=${NGINX_VERSION}-${PKG_RELEASE}         nginx-module-njs=${NGINX_VERSION}+${NJS_VERSION}-${PKG_RELEASE}     \"     \u0026\u0026 case \"$dpkgArch\" in         amd64|i386|arm64)             echo \"deb https://nginx.org/packages/debian/ buster nginx\" \u003e\u003e /etc/apt/sources.list.d/nginx.list             \u0026\u0026 apt-get update             ;;         *)             echo \"deb-src https://nginx.org/packages/debian/ buster nginx\" \u003e\u003e /etc/apt/sources.list.d/nginx.list                         \u0026\u0026 tempDir=\"$(mktemp -d)\"             \u0026\u0026 chmod 777 \"$tempDir\"                         \u0026\u0026 savedAptMark=\"$(apt-mark showmanual)\"                         \u0026\u0026 apt-get update             \u0026\u0026 apt-get build-dep -y $nginxPackages             \u0026\u0026 (                 cd \"$tempDir\"                 \u0026\u0026 DEB_BUILD_OPTIONS=\"nocheck parallel=$(nproc)\"                     apt-get source --compile $nginxPackages             )                         \u0026\u0026 apt-mark showmanual | xargs apt-mark auto \u003e /dev/null             \u0026\u0026 { [ -z \"$savedAptMark\" ] || apt-mark manual $savedAptMark; }                         \u0026\u0026 ls -lAFh \"$tempDir\"             \u0026\u0026 ( cd \"$tempDir\" \u0026\u0026 dpkg-scanpackages . \u003e Packages )             \u0026\u0026 grep '^Package: ' \"$tempDir/Packages\"             \u0026\u0026 echo \"deb [ trusted=yes ] file://$tempDir ./\" \u003e /etc/apt/sources.list.d/temp.list             \u0026\u0026 apt-get -o Acquire::GzipIndexes=false update             ;;     esac         \u0026\u0026 apt-get install --no-install-recommends --no-install-suggests -y                         $nginxPackages                         gettext-base                         curl     \u0026\u0026 apt-get remove --purge --auto-remove -y \u0026\u0026 rm -rf /var/lib/apt/lists/* /etc/apt/sources.list.d/nginx.list         \u0026\u0026 if [ -n \"$tempDir\" ]; then         apt-get purge -y --auto-remove         \u0026\u0026 rm -rf \"$tempDir\" /etc/apt/sources.list.d/temp.list;     fi     \u0026\u0026 ln -sf /dev/stdout /var/log/nginx/access.log     \u0026\u0026 ln -sf /dev/stderr /var/log/nginx/error.log     \u0026\u0026 mkdir /docker-entrypoint.d"},{"created":"2021-09-03T07:42:03.133022707Z","created_by":"/bin/sh -c #(nop) COPY file:65504f71f5855ca017fb64d502ce873a31b2e0decd75297a8fb0a287f97acf92 in / "},{"created":"2021-09-03T07:42:03.447690672Z","created_by":"/bin/sh -c #(nop) COPY file:0b866ff3fc1ef5b03c4e6c8c513ae014f691fb05d530257dfffd07035c1b75da in /docker-entrypoint.d "},{"created":"2021-09-03T07:42:03.751748066Z","created_by":"/bin/sh -c #(nop) COPY file:0fd5fca330dcd6a7de297435e32af634f29f7132ed0550d342cad9fd20158258 in /docker-entrypoint.d "},{"created":"2021-09-03T07:42:04.223396915Z","created_by":"/bin/sh -c #(nop) COPY file:09a214a3e07c919af2fb2d7c749ccbc446b8c10eb217366e5a65640ee9edcc25 in /docker-entrypoint.d "},{"created":"2021-09-03T07:42:04.499385465Z","created_by":"/bin/sh -c #(nop)  ENTRYPOINT [\"/docker-entrypoint.sh\"]","empty_layer":true},{"created":"2021-09-03T07:42:04.811649944Z","created_by":"/bin/sh -c #(nop)  EXPOSE 80","empty_layer":true},{"created":"2021-09-03T07:42:05.186046709Z","created_by":"/bin/sh -c #(nop)  STOPSIGNAL SIGQUIT","empty_layer":true},{"created":"2021-09-03T07:42:05.533312829Z","created_by":"/bin/sh -c #(nop)  CMD [\"nginx\" \"-g\" \"daemon off;\"]","empty_layer":true},{"created":"2021-09-24T18:08:19.146117274Z","created_by":"COPY /usr/local/bin /usr/local/bin # buildkit","comment":"buildkit.dockerfile.v0"},{"created":"2021-09-24T18:08:19.179626025Z","created_by":"COPY /usr/local/include /usr/local/include # buildkit","comment":"buildkit.dockerfile.v0"},{"created":"2021-09-24T18:08:22.056161016Z","created_by":"COPY /usr/local/lib /usr/local/lib # buildkit","comment":"buildkit.dockerfile.v0"},{"created":"2021-09-24T18:08:22.115351308Z","created_by":"COPY /usr/local/etc /usr/local/etc # buildkit","comment":"buildkit.dockerfile.v0"},{"created":"2021-09-24T18:08:22.136120093Z","created_by":"COPY /usr/local/share /usr/local/share # buildkit","comment":"buildkit.dockerfile.v0"},{"created":"2021-09-24T18:08:22.180907433Z","created_by":"COPY /usr/lib/nginx/modules /usr/lib/nginx/modules # buildkit","comment":"buildkit.dockerfile.v0"},{"created":"2021-09-24T18:08:36.729792877Z","created_by":"ENV LUAJIT_LIB=/usr/local/lib LUAJIT_INC=/usr/local/include/luajit-2.1","comment":"buildkit.dockerfile.v0","empty_layer":true},{"created":"2021-09-24T18:08:36.729792877Z","created_by":"RUN /bin/sh -c set -x     \u0026\u0026 apt-get update     \u0026\u0026 apt-get install -y --no-install-suggests       ca-certificates       curl       dnsutils       iputils-ping       libcurl4-openssl-dev       libyajl-dev       libxml2       lua5.1-dev       net-tools       procps       tcpdump       rsync       unzip       vim-tiny       libmaxminddb0     \u0026\u0026 apt-get clean     \u0026\u0026 rm -rf /var/lib/apt/lists/*     \u0026\u0026 ldconfig -v     \u0026\u0026 ls /etc/nginx/modules/*.so | grep -v debug     |  xargs -I{} sh -c 'echo \"load_module {};\" | tee -a  /etc/nginx/modules/all.conf'     \u0026\u0026 sed -i -E 's|listen\\s+80|\u002680|g' /etc/nginx/conf.d/default.conf     \u0026\u0026 ln -sf /dev/stdout /var/log/modsec_audit.log     \u0026\u0026 touch /var/run/nginx.pid     \u0026\u0026 mkdir -p /var/cache/nginx     \u0026\u0026 mkdir -p /var/cache/cache-heater     \u0026\u0026 chown -R nginx:nginx /etc/nginx /var/log/nginx /var/cache/nginx /var/run/nginx.pid /var/log/modsec_audit.log /var/cache/cache-heater # buildkit","comment":"buildkit.dockerfile.v0"},{"created":"2021-09-24T18:08:36.729792877Z","created_by":"EXPOSE map[8080/tcp:{} 8443/tcp:{}]","comment":"buildkit.dockerfile.v0","empty_layer":true},{"created":"2021-09-24T18:08:36.729792877Z","created_by":"USER nginx","comment":"buildkit.dockerfile.v0","empty_layer":true},{"created":"2021-09-24T18:08:36.729792877Z","created_by":"WORKDIR /etc/nginx","comment":"buildkit.dockerfile.v0","empty_layer":true}],"os":"linux","rootfs":{"type":"layers","diff_ids":["sha256:d000633a56813933cb0ac5ee3246cf7a4c0205db6290018a169d7cb096581046","sha256:e8fe83da41c065dbd6839bcd6a64231c7b907db44fa3972a26082215530d5335","sha256:fa7a9b5be61bae580862e9e9d20ca479b3960da39be573db98040d17cd91beab","sha256:9a6fb2a1f2da17f095ed490202c7b9b63cdc00c73ff76bc3ab6103d44b7d68f9","sha256:f68562b026ea376a28d5782cbcc6879131d1840cbd09e210216d81c70cc6ac23","sha256:93acac7be8868316e41f0417183f898cdcae620004ba98ee3a1482199d402270","sha256:8dded7b67aeadbedef49ca5b4332c88bb7683155ec0282340416cafee0a26e9c","sha256:fc9f9539814901131574f154212c049ff6071b2660b46e3eba5c6f12553308e7","sha256:102cf28da3b7fb5fa0e5413b0071e8f64f992d29dbe313b0c624bb9ae7810070","sha256:f27972b7319bee519d1c4b398da4a3dd6739a07ec0270ff1c09fee5dd7eff257","sha256:573f7d9705980d58eb6a2b9ae0c23374506287371ed331187ae727ca6fa3a745","sha256:17f5edb40d7cc636f37093ab8b0d17612e9469ace6102026c81c93457f103dd1","sha256:e55c9cbb872174ef79d5c5ea1c1636f89aec3b2cc6338394d4180582a711bd01"]}}`)
					return
				}

				w.WriteHeader(http.StatusInternalServerError)
			}),
			image: "tsuru/nginx-tsuru:1.20.1-0.7.0",
			expected: []string{
				"ndk_http_module",
				"ngx_http_cache_purge_module",
				"ngx_http_dav_ext_module",
				"ngx_http_echo_module",
				"ngx_http_fancyindex_module",
				"ngx_http_geoip2_module",
				"ngx_http_geoip_module",
				"ngx_http_headers_more_filter_module",
				"ngx_http_image_filter_module",
				"ngx_http_js_module",
				"ngx_http_location_name_module",
				"ngx_http_lua_module",
				"ngx_http_lua_ssl_module",
				"ngx_http_modsecurity_module",
				"ngx_http_push_stream_module",
				"ngx_http_subs_filter_module",
				"ngx_http_uploadprogress_module",
				"ngx_http_vhost_traffic_status_module",
				"ngx_http_xslt_filter_module",
				"ngx_stream_geoip2_module",
				"ngx_stream_geoip_module",
				"ngx_stream_js_module",
			},
		},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			apiCalls = 0

			require.NotNil(t, tt.handler, "you must provide an HTTP handler for this test case")

			srv := httptest.NewServer(tt.handler)
			defer srv.Close()

			image := fmt.Sprintf("%s/%s", strings.TrimPrefix(srv.URL, "http://"), tt.image)

			modules, err := registry.NewImageMetadata().Modules(context.TODO(), image)
			require.NoError(t, err)

			assert.Equal(t, tt.expected, modules)
		})
	}
}
