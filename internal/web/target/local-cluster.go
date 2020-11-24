package target

import (
	"context"
	"net/http"

	"github.com/tsuru/rpaas-operator/internal/pkg/rpaas"
	"github.com/tsuru/rpaas-operator/pkg/observability"
	extensionsruntime "github.com/tsuru/rpaas-operator/pkg/runtime"
	sigsk8sclient "sigs.k8s.io/controller-runtime/pkg/client"
	sigsk8sconfig "sigs.k8s.io/controller-runtime/pkg/client/config"
)

var _ Factory = &localClusterFactory{}

type localClusterFactory struct {
	manager rpaas.RpaasManager
}

func (l *localClusterFactory) Manager(ctx context.Context, header http.Header) (rpaas.RpaasManager, error) {
	return l.manager, nil
}

func NewKubeConfigFactory() (Factory, error) {
	restConfig, err := sigsk8sconfig.GetConfig()
	if err != nil {
		return nil, err
	}
	restConfig.WrapTransport = observability.OpentracingTransport

	k8sClient, err := sigsk8sclient.New(restConfig, sigsk8sclient.Options{Scheme: extensionsruntime.NewScheme()})
	if err != nil {
		return nil, err
	}

	manager, err := rpaas.NewK8S(restConfig, k8sClient)
	if err != nil {
		return nil, err
	}

	return &localClusterFactory{manager: manager}, nil
}

func NewLocalFactory(manager rpaas.RpaasManager) Factory {
	return &localClusterFactory{manager: manager}
}
