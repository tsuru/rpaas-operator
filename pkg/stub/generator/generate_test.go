package generator

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tsuru/rpaas-operator/pkg/apis/extensions/v1alpha1"
)

type fakeRefReader struct{}

func (f *fakeRefReader) ReadConfigRef(ref v1alpha1.ConfigRef, ns string) (string, error) {
	return ref.Value, nil
}

var minimumConfig = v1alpha1.NginxConfig{
	User:              "usr1",
	Listen:            "0.0.0.0:80",
	AdminListen:       "0.0.0.0:88",
	KeyZoneSize:       "1024",
	CacheInactive:     "10",
	CacheSize:         "11",
	WorkerProcesses:   1,
	ListenBacklog:     2,
	WorkerConnections: 3,
	LoaderFiles:       4,
}

func TestInterpolateConfigFile(t *testing.T) {
	b := ConfigBuilder{RefReader: &fakeRefReader{}}
	result, err := b.Interpolate(v1alpha1.RpaasInstance{}, v1alpha1.RpaasPlanSpec{
		Config: minimumConfig,
	})
	require.NoError(t, err)
	assert.Regexp(t, `user usr1;`, result)
	assert.Regexp(t, `worker_processes 1;`, result)
	assert.Regexp(t, `worker_connections 3;`, result)
	assert.Regexp(t, `proxy_cache_path /var/cache/nginx levels=1:2 keys_zone=rpaas:1024 inactive=10 max_size=11 loader_files=4;`, result)
	assert.Regexp(t, `listen 0\.0\.0\.0:80 default_server backlog=2;`, result)
	assert.Regexp(t, `listen 0\.0\.0\.0:88;`, result)
}
