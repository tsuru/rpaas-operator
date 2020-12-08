package registry

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/docker/libtrust"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestImageMetadataRetriever_Modules(t *testing.T) {
	var apiCalls []string
	var labels string
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		apiCalls = append(apiCalls, r.URL.Path)
		sig, err := libtrust.NewJSONSignatureFromMap(map[string]interface{}{
			"history": []map[string]interface{}{{
				"v1Compatibility": fmt.Sprintf(`{
					"config":{%s}
				}`, labels),
			}},
		})
		require.NoError(t, err)
		pk, err := libtrust.GenerateECP256PrivateKey()
		require.NoError(t, err)
		err = sig.Sign(pk)
		require.NoError(t, err)
		data, err := sig.PrettySignature("signatures")
		require.NoError(t, err)
		w.Write(data)
	}))
	defer srv.Close()
	u, err := url.Parse(srv.URL)
	require.NoError(t, err)

	tests := []struct {
		image           string
		labels          string
		expectedModules []string
		expectedCalls   []string
	}{
		{
			image:           "my/img:v1",
			expectedModules: nil,
			expectedCalls:   []string{"/v2/my/img/manifests/v1"},
		},
		{
			image:           "my/img:v1",
			labels:          `"Labels":{"io.tsuru.nginx-modules":"mod1,mod2"}`,
			expectedModules: []string{"mod1", "mod2"},
			expectedCalls:   []string{"/v2/my/img/manifests/v1"},
		},
		{
			image:           "my/img:latest",
			expectedModules: nil,
			expectedCalls:   []string{"/v2/my/img/manifests/latest", "/v2/my/img/manifests/latest"},
		},
		{
			image:           "my/img:edge",
			expectedModules: nil,
			expectedCalls:   []string{"/v2/my/img/manifests/edge", "/v2/my/img/manifests/edge"},
		},
		{
			image:           "my/img:v1",
			labels:          `"Labels":{}`,
			expectedModules: nil,
			expectedCalls:   []string{"/v2/my/img/manifests/v1"},
		},
		{
			image:           "my/img:v1",
			labels:          `"Labels":null`,
			expectedModules: nil,
			expectedCalls:   []string{"/v2/my/img/manifests/v1"},
		},
		{
			image:           "my/img:v1",
			labels:          `"Labels":{"other":"mod1,mod2"}`,
			expectedModules: nil,
			expectedCalls:   []string{"/v2/my/img/manifests/v1"},
		},
		{
			image:           "my/img:v1",
			labels:          `"Labels":{"io.tsuru.nginx-modules":""}`,
			expectedModules: nil,
			expectedCalls:   []string{"/v2/my/img/manifests/v1"},
		},
	}

	for _, tt := range tests {
		t.Run("", func(t *testing.T) {
			apiCalls = nil
			labels = tt.labels
			r := NewImageMetadata()
			r.insecure = true
			mod, err := r.Modules(u.Host + "/" + tt.image)
			require.NoError(t, err)
			assert.Equal(t, tt.expectedModules, mod)
			mod, err = r.Modules(u.Host + "/" + tt.image)
			require.NoError(t, err)
			assert.Equal(t, tt.expectedModules, mod)
			assert.Equal(t, tt.expectedCalls, apiCalls)

		})
	}
}

func TestParseImage(t *testing.T) {
	tests := []struct {
		imageURI         string
		expectedRegistry string
		expectedImage    string
		expectedTag      string
	}{
		{"f064bf4", "registry-1.docker.io", "f064bf4", "latest"},
		{"", "registry-1.docker.io", "", "latest"},
		{"registry.io/tsuru/app-img:v1", "registry.io", "tsuru/app-img", "v1"},
		{"tsuru/app-img:v1", "registry-1.docker.io", "tsuru/app-img", "v1"},
		{"tsuru/app-img", "registry-1.docker.io", "tsuru/app-img", "latest"},
		{"f064bf4:v1", "registry-1.docker.io", "f064bf4", "v1"},
		{"registry:5000/app-img:v1", "registry:5000", "app-img", "v1"},
		{"registry.io/app-img:v1", "registry.io", "app-img", "v1"},
		{"localhost/app-img:v1", "localhost", "app-img", "v1"},
	}

	for _, tt := range tests {
		t.Run("", func(t *testing.T) {
			parts := parseImage(tt.imageURI)
			assert.Equal(t, parts.registry, tt.expectedRegistry)
			assert.Equal(t, parts.image, tt.expectedImage)
			assert.Equal(t, parts.tag, tt.expectedTag)
		})
	}
}
