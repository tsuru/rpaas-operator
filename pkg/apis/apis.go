// Copyright 2019 tsuru authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package apis

import (
	nginxApis "github.com/tsuru/nginx-operator/pkg/apis"
	"github.com/tsuru/rpaas-operator/pkg/apis/extensions/v1alpha1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/config"
	"sigs.k8s.io/controller-runtime/pkg/manager"
)

// AddToSchemes may be used to add all resources defined in the project to a Scheme
var AddToSchemes runtime.SchemeBuilder

// AddToScheme adds all Resources to the Scheme
func AddToScheme(s *runtime.Scheme) error {
	return AddToSchemes.AddToScheme(s)
}

// AddFieldIndexes adds some indexes on fieldIndexer so their objects can
// later be fetched by a field selector.
func AddFieldIndexes(fieldIndexer client.FieldIndexer) error {
	return fieldIndexer.IndexField(
		&v1alpha1.RpaasInstance{},
		"metadata.name",
		func(o runtime.Object) []string { return []string{o.(*v1alpha1.RpaasInstance).Name} },
	)
}

func NewManager() (manager.Manager, error) {
	cfg, err := config.GetConfig()
	if err != nil {
		return nil, err
	}
	mgr, err := manager.New(cfg, manager.Options{})
	if err != nil {
		return nil, err
	}
	if err = AddToScheme(mgr.GetScheme()); err != nil {
		return nil, err
	}
	if err = nginxApis.AddToScheme(mgr.GetScheme()); err != nil {
		return nil, err
	}
	if err = AddFieldIndexes(mgr.GetFieldIndexer()); err != nil {
		return nil, err
	}
	return mgr, nil
}
