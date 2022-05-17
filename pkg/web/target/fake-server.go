package target

import (
	"context"
	"net/http"

	"github.com/tsuru/rpaas-operator/internal/pkg/rpaas"
	extensionsruntime "github.com/tsuru/rpaas-operator/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

var _ Factory = &fakeServerFactory{}

type fakeServerFactory struct {
	manager rpaas.RpaasManager
}

func (f *fakeServerFactory) Manager(ctx context.Context, header http.Header) (rpaas.RpaasManager, error) {
	return f.manager, nil
}

func NewFakeServerFactory(runtimeObjects []runtime.Object) (Factory, error) {

	scheme := extensionsruntime.NewScheme()
	k8sClient := fake.NewClientBuilder().WithScheme(scheme).WithRuntimeObjects(runtimeObjects...).Build()

	manager, err := rpaas.NewK8S(nil, k8sClient, "", "")
	if err != nil {
		return nil, err
	}

	return &fakeServerFactory{manager: manager}, nil
}

func NewFakeFactory(manager rpaas.RpaasManager) Factory {
	return &fakeServerFactory{manager: manager}
}
