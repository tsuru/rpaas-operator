package apis

import (
	"github.com/tsuru/rpaas-operator/pkg/apis/extensions/v1alpha1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/manager"
)

// AddToSchemes may be used to add all resources defined in the project to a Scheme
var AddToSchemes runtime.SchemeBuilder

// AddToScheme adds all Resources to the Scheme
func AddToScheme(s *runtime.Scheme) error {
	return AddToSchemes.AddToScheme(s)
}

// IndexRpaasInstanceName adds a FieldIndexer for indexing the RpaasInstance's
// name over the cache, since they can later be consumed via field selector
// from the client.
func IndexRpaasInstanceName(m manager.Manager) error {
	indexerFunc := func(o runtime.Object) []string {
		return []string{o.(*v1alpha1.RpaasInstance).Name}
	}
	return m.GetFieldIndexer().IndexField(&v1alpha1.RpaasInstance{}, "metadata.name", indexerFunc)
}
