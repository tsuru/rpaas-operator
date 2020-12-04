package registry

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

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
