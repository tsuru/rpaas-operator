package api

import (
	"github.com/tsuru/rpaas-operator/pkg/apis"
	"k8s.io/apimachinery/pkg/runtime"
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
	scheme := runtime.NewScheme()
	if err = apis.AddToScheme(scheme); err != nil {
		return err
	}
	m, err := manager.New(cfg, manager.Options{Scheme: scheme})
	if err != nil {
		return err
	}
	if err = apis.IndexRpaasInstanceName(m); err != nil {
		return err
	}
	cli = m.GetClient()
	go m.Start(signals.SetupSignalHandler())
	return nil
}
