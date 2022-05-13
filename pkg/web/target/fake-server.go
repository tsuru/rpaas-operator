package target

import (
	"context"
	"net/http"

	"github.com/tsuru/rpaas-operator/api/v1alpha1"
	"github.com/tsuru/rpaas-operator/internal/pkg/rpaas"
	extensionsruntime "github.com/tsuru/rpaas-operator/pkg/runtime"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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

func NewFakeServerFactory() (Factory, error) {

	scheme := extensionsruntime.NewScheme()
	k8sClient := fake.NewClientBuilder().WithScheme(scheme).WithRuntimeObjects(fakeRuntimeObjects()...).Build()

	manager, err := rpaas.NewK8S(nil, k8sClient, "", "")
	if err != nil {
		return nil, err
	}

	return &fakeServerFactory{manager: manager}, nil
}

func NewFakeFactory(manager rpaas.RpaasManager) Factory {
	return &fakeServerFactory{manager: manager}
}

func fakeRuntimeObjects() []runtime.Object {
	return []runtime.Object{
		&v1alpha1.RpaasPlan{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "my-plan",
				Namespace: "rpaasv2",
			},
		},
		&v1alpha1.RpaasInstance{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "my-rpaas",
				Namespace: "rpaasv2",
			},
		},
	}
}
