package target

import (
	"context"
	"encoding/base64"
	"errors"
	"net/http"
	"os"
	"sync"

	"github.com/opentracing/opentracing-go"
	"k8s.io/client-go/rest"
	sigsk8sclient "sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/tsuru/rpaas-operator/internal/config"
	"github.com/tsuru/rpaas-operator/internal/pkg/rpaas"
	"github.com/tsuru/rpaas-operator/internal/pkg/rpaas/validation"
	"github.com/tsuru/rpaas-operator/pkg/observability"
	extensionsruntime "github.com/tsuru/rpaas-operator/pkg/runtime"
)

var _ Factory = &multiClusterFactory{}

type missingParamsError struct {
	Msg           string   `json:"msg"`
	MissingParams []string `json:"missing_params"`
}

func (e *missingParamsError) Error() string {
	return e.Msg
}

func (e *missingParamsError) IsValidation() bool {
	return true
}

var ErrNoClusterProvided = &missingParamsError{
	Msg:           "No cluster address provided",
	MissingParams: []string{"cluster"},
}

type multiClusterFactory struct {
	tokens        sync.Map
	clusters      []config.ClusterConfig
	managersMutex sync.RWMutex
	managers      map[managersCacheKey]rpaas.RpaasManager
}

type managersCacheKey struct {
	clusterName    string
	poolName       string
	clusterAddress string
}

func NewMultiClustersFactory(clusters []config.ClusterConfig) Factory {
	return &multiClusterFactory{
		clusters:      clusters,
		tokens:        sync.Map{},
		managersMutex: sync.RWMutex{},
		managers:      map[managersCacheKey]rpaas.RpaasManager{},
	}
}

func (m *multiClusterFactory) Manager(ctx context.Context, headers http.Header) (rpaas.RpaasManager, error) {
	clusterName := headers.Get("X-Tsuru-Cluster-Name")
	address := headers.Get("X-Tsuru-Cluster-Addresses")
	disableValidation := headers.Get("X-RPaaS-Disable-Validation") != ""

	if address == "" {
		return nil, ErrNoClusterProvided
	}

	poolName := headers.Get("X-Tsuru-Pool-Name")
	cacheKey := managersCacheKey{clusterName, poolName, address}

	m.managersMutex.RLock()
	manager := m.managers[cacheKey]
	m.managersMutex.RUnlock()

	if manager != nil {
		return manager, nil
	}

	span := opentracing.SpanFromContext(ctx)
	if span != nil {
		span.SetTag("cluster.name", clusterName)
		span.SetTag("cluster.address", address)
		span.SetTag("pool.name", poolName)
	}

	kubernetesRestConfig, err := m.getKubeConfig(clusterName, address)
	if err != nil {
		return nil, err
	}

	clusterValidationDisabled := m.validationDisabled(clusterName)

	k8sClient, err := sigsk8sclient.New(kubernetesRestConfig, sigsk8sclient.Options{Scheme: extensionsruntime.NewScheme()})
	if err != nil {
		return nil, err
	}

	manager, err = rpaas.NewK8S(kubernetesRestConfig, k8sClient, clusterName, poolName)
	if err != nil {
		return nil, err
	}

	if !disableValidation && !clusterValidationDisabled {
		manager = validation.New(manager, k8sClient)
	}

	m.managersMutex.Lock()
	defer m.managersMutex.Unlock()

	m.managers[cacheKey] = manager
	return manager, nil
}

func (m *multiClusterFactory) validationDisabled(name string) bool {
	for _, cluster := range m.clusters {
		if cluster.Name == name {
			return cluster.DisableValidation
		}
	}

	return false
}

func (m *multiClusterFactory) getKubeConfig(name, address string) (*rest.Config, error) {
	selectedCluster := config.ClusterConfig{}

	for _, cluster := range m.clusters {
		if cluster.Default {
			selectedCluster = cluster
		}
		if cluster.Name == name {
			selectedCluster = cluster
			break
		}
	}

	if selectedCluster.Name == "" {
		return nil, errors.New("cluster not found")
	}

	if selectedCluster.Address != "" {
		address = selectedCluster.Address
	}

	restConfig := &rest.Config{
		Host:          address,
		BearerToken:   selectedCluster.Token,
		WrapTransport: observability.OpentracingTransport,
	}

	if selectedCluster.AuthProvider != nil {
		restConfig.AuthProvider = selectedCluster.AuthProvider
	}

	if selectedCluster.ExecProvider != nil {
		restConfig.ExecProvider = selectedCluster.ExecProvider
	}

	if selectedCluster.CA != "" {
		caData, err := base64.StdEncoding.DecodeString(selectedCluster.CA)
		if err != nil {
			return nil, err
		}
		restConfig.TLSClientConfig.CAData = caData
	}

	if selectedCluster.TokenFile != "" {
		var err error
		restConfig.BearerToken, err = m.readTokenFile(selectedCluster.TokenFile)
		if err != nil {
			return nil, err
		}
	}

	return restConfig, nil
}

func (m *multiClusterFactory) readTokenFile(tokenFile string) (string, error) {
	tokenInterface, ok := m.tokens.Load(tokenFile)

	if ok {
		return tokenInterface.(string), nil
	}

	token, err := os.ReadFile(tokenFile)
	if err != nil {
		return "", err
	}

	m.tokens.Store(tokenFile, string(token))

	return string(token), nil
}
