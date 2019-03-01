package api

import (
	"time"

	"github.com/tsuru/rpaas-operator/pkg/apis"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/config"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/runtime/signals"
)

const (
	apiTimeout   = 10 * time.Second
	dialTimeout  = 30 * time.Second
	tcpKeepAlive = 30 * time.Second
	NAMESPACE    = "default"
)

type kubeConfig struct {
	Addr       string
	CaCert     []byte
	ClientCert []byte
	ClientKey  []byte
}

var cli client.Client

func setup() error {
	cfg, err := config.GetConfig()
	if err != nil {
		return err
	}
	m, err := manager.New(cfg, manager.Options{Namespace: NAMESPACE})
	if err != nil {
		return err
	}
	cli = m.GetClient()
	err = apis.AddToScheme(m.GetScheme())
	if err != nil {
		return err
	}
	go m.Start(signals.SetupSignalHandler())
	return nil
}
