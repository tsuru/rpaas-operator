package registry

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/docker/libtrust"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestImageMetadataRetriever_Modules(t *testing.T) {
	r := NewImageMetadata()
	r.insecure = true
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/v2/my/img/manifests/v1", r.URL.Path)
		assert.Equal(t, "GET", r.Method)
		w.Header().Add("Content-Type", "application/vnd.docker.distribution.manifest.v1+json")
		sig, err := libtrust.NewJSONSignatureFromMap(map[string]interface{}{
			"schemaVersion": 1,
			"name":          "my/img",
			"tag":           "v1",
			"history": []map[string]interface{}{{
				"v1Compatibility": `{
					"config":{
						"Labels":{"io.tsuru.nginx-modules":"mod1,mod2"}
					}
				}`,
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
	mod, err := r.Modules(u.Host + "/my/img:v1")
	require.NoError(t, err)
	assert.Equal(t, []string{"mod1", "mod2"}, mod)
}

func TestParseImage(t *testing.T) {
	tests := []struct {
		imageURI         string
		expectedRegistry string
		expectedImage    string
		expectedTag      string
	}{
		{"f064bf4", "registry-1.docker.io", "f064bf4", ""},
		{"", "registry-1.docker.io", "", ""},
		{"registry.io/tsuru/app-img:v1", "registry.io", "tsuru/app-img", "v1"},
		{"tsuru/app-img:v1", "registry-1.docker.io", "tsuru/app-img", "v1"},
		{"tsuru/app-img", "registry-1.docker.io", "tsuru/app-img", ""},
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
