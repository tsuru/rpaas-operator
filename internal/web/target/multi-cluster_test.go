package target

import (
	"context"
	"io/ioutil"
	"net/http"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/tsuru/rpaas-operator/internal/config"
)

var ctx = context.Background()

func TestMultiClusterToken(t *testing.T) {
	target := NewMultiClustersFactory([]config.ClusterConfig{
		{
			Name:  "my-cluster",
			Token: "my-token",
		},
	})

	multiClusterTarget := target.(*multiClusterFactory)
	token, err := multiClusterTarget.getToken("my-cluster")

	assert.NoError(t, err)
	assert.Equal(t, token, "my-token")
}

func TestMultiClusterTokenFile(t *testing.T) {
	tmpfile, err := ioutil.TempFile("", "example")
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
	token, err := multiClusterTarget.getToken("my-cluster")

	assert.NoError(t, err)
	assert.Equal(t, token, "token-from-file")

	os.Remove(tmpfile.Name())
	token, err = multiClusterTarget.getToken("my-cluster")

	assert.NoError(t, err)
	assert.Equal(t, token, "token-from-file")
}

func TestMultiClusterNoToken(t *testing.T) {
	tmpfile, err := ioutil.TempFile("", "example")
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
	token, err := multiClusterTarget.getToken("my-wrong-cluster")

	assert.NoError(t, err)
	assert.Equal(t, token, "")
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
	token, err := multiClusterTarget.getToken("my-other-cluster")

	assert.NoError(t, err)
	assert.Equal(t, token, "my-token")
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
	assert.Equal(t, err, ErrNoClusterProvided)
}
