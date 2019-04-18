package api

import (
	"github.com/tsuru/rpaas-operator/pkg/apis"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/config"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/runtime/signals"
)

var cli client.Client

func setup() error {
	cfg, err := config.GetConfig()
	if err != nil {
		return err
	}
	m, err := manager.New(cfg, manager.Options{})
	if err != nil {
		return err
	}
	if err = apis.AddToScheme(m.GetScheme()); err != nil {
		return err
	}
	if err = apis.AddFieldIndexes(m.GetFieldIndexer()); err != nil {
		return err
	}
	cli = m.GetClient()
	go m.Start(signals.SetupSignalHandler())
	return nil
}
