package target

import (
	"context"
	"net/http"

	"github.com/opentracing/opentracing-go"
	"github.com/tsuru/rpaas-operator/internal/config"
	"github.com/tsuru/rpaas-operator/internal/pkg/rpaas"
	"github.com/tsuru/rpaas-operator/pkg/observability"
	extensionsruntime "github.com/tsuru/rpaas-operator/pkg/runtime"
	"k8s.io/client-go/rest"
	sigsk8sclient "sigs.k8s.io/controller-runtime/pkg/client"
)

var _ Factory = &multiClusterFactory{}

var ErrNoClusterProvided = &rpaas.ValidationError{Msg: "No cluster address provided"}

type multiClusterFactory struct {
	clusters []config.ClusterConfig
}

func NewMultiClustersFactory(clusters []config.ClusterConfig) Factory {
	return &multiClusterFactory{clusters: clusters}
}

func (m *multiClusterFactory) Manager(ctx context.Context, headers http.Header) (rpaas.RpaasManager, error) {
	name := headers.Get("X-Tsuru-Cluster-Name")
	address := headers.Get("X-Tsuru-Cluster-Addresses")

	if address == "" {
		return nil, ErrNoClusterProvided
	}

	span := opentracing.SpanFromContext(ctx)
	if span != nil {
		span.SetTag("cluster.name", name)
		span.SetTag("cluster.address", address)
	}

	kubernetesRestConfig := &rest.Config{
		Host:          address,
		BearerToken:   m.getToken(name),
		WrapTransport: observability.OpentracingTransport,
	}
	k8sClient, err := sigsk8sclient.New(kubernetesRestConfig, sigsk8sclient.Options{Scheme: extensionsruntime.NewScheme()})
	if err != nil {
		return nil, err
	}

	manager, err := rpaas.NewK8S(kubernetesRestConfig, k8sClient)
	if err != nil {
		return nil, err
	}
	return manager, nil
}

func (m *multiClusterFactory) getToken(clusterName string) string {
	defaultToken := ""
	for _, cluster := range m.clusters {
		if cluster.Default {
			defaultToken = cluster.Token
		}
		if cluster.Name == clusterName {
			return cluster.Token
		}
	}
	return defaultToken
}
