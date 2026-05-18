package target

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"

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

func TestGetKubeConfigFromHeader(t *testing.T) {
	factory := NewMultiClustersFactory(nil).(*multiClusterFactory)

	t.Run("valid kube config", func(t *testing.T) {
		tsuruKubeConfig := TsuruKubeConfig{
			Cluster: clientcmdapi.Cluster{
				Server:                   "https://mycluster.example.com",
				CertificateAuthorityData: []byte("fake-ca-data"),
			},
			AuthInfo: clientcmdapi.AuthInfo{
				Token: "my-token",
			},
		}
		data, err := json.Marshal(tsuruKubeConfig)
		require.NoError(t, err)

		b64Config := base64.StdEncoding.EncodeToString(data)

		restConfig, err := factory.getKubeConfigFromHeader("test-cluster", b64Config)
		require.NoError(t, err)

		assert.Equal(t, "https://mycluster.example.com", restConfig.Host)
		assert.Equal(t, "my-token", restConfig.BearerToken)
		assert.Equal(t, []byte("fake-ca-data"), restConfig.TLSClientConfig.CAData)
		assert.Equal(t, 30*time.Second, restConfig.Timeout)
		assert.NotNil(t, restConfig.WrapTransport)
	})

	t.Run("invalid base64", func(t *testing.T) {
		_, err := factory.getKubeConfigFromHeader("test-cluster", "not-valid-base64!!!")
		require.Error(t, err)
	})

	t.Run("invalid json", func(t *testing.T) {
		b64Config := base64.StdEncoding.EncodeToString([]byte("not json"))

		_, err := factory.getKubeConfigFromHeader("test-cluster", b64Config)
		require.Error(t, err)
	})

	t.Run("with client certificate auth", func(t *testing.T) {
		tsuruKubeConfig := TsuruKubeConfig{
			Cluster: clientcmdapi.Cluster{
				Server: "https://mycluster.example.com",
			},
			AuthInfo: clientcmdapi.AuthInfo{
				ClientCertificateData: []byte("fake-cert"),
				ClientKeyData:         []byte("fake-key"),
			},
		}
		data, err := json.Marshal(tsuruKubeConfig)
		require.NoError(t, err)

		b64Config := base64.StdEncoding.EncodeToString(data)

		restConfig, err := factory.getKubeConfigFromHeader("test-cluster", b64Config)
		require.NoError(t, err)

		assert.Equal(t, "https://mycluster.example.com", restConfig.Host)
		assert.Equal(t, []byte("fake-cert"), restConfig.TLSClientConfig.CertData)
		assert.Equal(t, []byte("fake-key"), restConfig.TLSClientConfig.KeyData)
	})
}

func TestMultiClusterManagerWithKubeConfigHeader(t *testing.T) {
	t.Run("no error when only kube config header is provided", func(t *testing.T) {
		target := NewMultiClustersFactory(nil)

		tsuruKubeConfig := TsuruKubeConfig{
			Cluster: clientcmdapi.Cluster{
				Server: "https://mycluster.example.com",
			},
			AuthInfo: clientcmdapi.AuthInfo{
				Token: "my-token",
			},
		}
		data, err := json.Marshal(tsuruKubeConfig)
		require.NoError(t, err)

		b64Config := base64.StdEncoding.EncodeToString(data)

		headers := http.Header{}
		headers.Set("X-Tsuru-Cluster-Kube-Config", b64Config)
		headers.Set("X-Tsuru-Cluster-Name", "test-cluster")

		// Manager will fail when trying to create k8s client (no real cluster),
		// but it should NOT fail with ErrNoClusterProvided
		_, err = target.Manager(ctx, headers)
		assert.NotEqual(t, ErrNoClusterProvided, err)
	})

	t.Run("error when no address and no kube config", func(t *testing.T) {
		target := NewMultiClustersFactory(nil)

		headers := http.Header{}
		headers.Set("X-Tsuru-Cluster-Name", "test-cluster")

		_, err := target.Manager(ctx, headers)
		assert.Equal(t, ErrNoClusterProvided, err)
	})
}
