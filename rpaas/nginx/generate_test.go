package nginx

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/tsuru/rpaas-operator/pkg/apis/extensions/v1alpha1"
)

func TestRpaasConfigurationRenderer_Render(t *testing.T) {
	testCases := []struct {
		data      ConfigurationData
		assertion func(*testing.T, string, error)
	}{
		{
			data: ConfigurationData{
				Config: &v1alpha1.NginxConfig{},
			},
			assertion: func(t *testing.T, result string, err error) {
				assert.NoError(t, err)
				assert.Regexp(t, `user nginx;`, result)
				assert.Regexp(t, `worker_process 1;`, result)
				assert.Regexp(t, `worker_connections 1024;`, result)
				assert.Regexp(t, `access_log /dev/stdout rpaas_combined;`, result)
				assert.Regexp(t, `error_log  /dev/stderr;`, result)
				assert.Regexp(t, `listen 80 default_server;`, result)
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
