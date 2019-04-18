package apis

import (
	"github.com/tsuru/rpaas-operator/pkg/apis/extensions/v1alpha1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// AddToSchemes may be used to add all resources defined in the project to a Scheme
var AddToSchemes runtime.SchemeBuilder

// AddToScheme adds all Resources to the Scheme
func AddToScheme(s *runtime.Scheme) error {
	return AddToSchemes.AddToScheme(s)
}

// AddFieldIndexes adds some indexes on fieldIndexer so their objects can
// later be fetched by a field selector.
func AddFieldIndexes(fieldIndexer client.FieldIndexer) (err error) {
	err = fieldIndexer.IndexField(
		&v1alpha1.RpaasInstance{},
		"metadata.name",
		func(o runtime.Object) []string { return []string{o.(*v1alpha1.RpaasInstance).Name} },
	)

	if err != nil {
		return
	}

	err = fieldIndexer.IndexField(
		&v1alpha1.RpaasPlan{},
		"metadata.name",
		func(o runtime.Object) []string { return []string{o.(*v1alpha1.RpaasPlan).Name} },
	)

	return
}
