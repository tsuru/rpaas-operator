package target

import (
	"context"
	"net/http"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/tsuru/rpaas-operator/internal/config"
)

var ctx = context.Background()

func TestMultiClusterTokenFile(t *testing.T) {
	tmpfile, err := os.CreateTemp("", "example")
	require.NoError(t, err)
	target := NewMultiClustersFactory([]config.ClusterConfig{
		{
			Name:      "my-cluster",
			TokenFile: tmpfile.Name(),
		},
	})

	_, err = tmpfile.Write([]byte("token-from-file"))
	assert.NoError(t, err)

	defer os.Remove(tmpfile.Name())

	multiClusterTarget := target.(*multiClusterFactory)
	restConfig, err := multiClusterTarget.getKubeConfig("my-cluster", "")

	assert.NoError(t, err)
	assert.Equal(t, "token-from-file", restConfig.BearerToken)

	os.Remove(tmpfile.Name())
	restConfig, err = multiClusterTarget.getKubeConfig("my-cluster", "")

	assert.NoError(t, err)
	assert.Equal(t, "token-from-file", restConfig.BearerToken)
}

func TestMultiClusterNoToken(t *testing.T) {
	tmpfile, err := os.CreateTemp("", "example")
	require.NoError(t, err)
	target := NewMultiClustersFactory([]config.ClusterConfig{
		{
			Name:      "my-cluster",
			TokenFile: tmpfile.Name(),
		},
	})

	_, err = tmpfile.Write([]byte("token-from-file"))
	assert.NoError(t, err)

	defer os.Remove(tmpfile.Name())

	multiClusterTarget := target.(*multiClusterFactory)
	_, err = multiClusterTarget.getKubeConfig("my-wrong-cluster", "")

	require.Error(t, err)
	assert.Equal(t, "cluster not found", err.Error())
}

func TestMultiClusterDefaultToken(t *testing.T) {

	target := NewMultiClustersFactory([]config.ClusterConfig{
		{
			Name:    "my-cluster",
			Token:   "my-token",
			Default: true,
		},
	})

	multiClusterTarget := target.(*multiClusterFactory)
	restConfig, err := multiClusterTarget.getKubeConfig("my-other-cluster", "")

	assert.NoError(t, err)
	assert.Equal(t, "my-token", restConfig.BearerToken)
}

func TestMultiClusterNoHeaders(t *testing.T) {
	target := NewMultiClustersFactory([]config.ClusterConfig{
		{
			Name:    "my-cluster",
			Token:   "my-token",
			Default: true,
		},
	})

	rpaasManager, err := target.Manager(ctx, http.Header{})

	assert.Nil(t, rpaasManager)
	assert.Equal(t, ErrNoClusterProvided, err)
}
