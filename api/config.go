package api

import (
	"errors"
	"io/ioutil"
	"net"
	"os"
	"time"

	"github.com/tsuru/rpaas-operator/pkg/apis"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
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

func getKubeConfig() (kubeConfig, error) {
	k := kubeConfig{
		Addr: os.Getenv("KUBERNETES_ADDRESS"),
	}
	if len(k.Addr) == 0 {
		return k, errors.New("Missing env var KUBERNETES_ADDRESS")
	}
	caCertFilename := os.Getenv("KUBERNETES_CA_CERT")
	if len(caCertFilename) == 0 {
		return k, errors.New("Missing env var KUBERNETES_CA_CERT")
	}
	var err error
	k.CaCert, err = ioutil.ReadFile(caCertFilename)
	if err != nil {
		return k, err
	}
	clientCertFilename := os.Getenv("KUBERNETES_CLIENT_CERT")
	if len(clientCertFilename) == 0 {
		return k, errors.New("Missing env var KUBERNETES_CLIENT_CERT")
	}
	k.ClientCert, err = ioutil.ReadFile(clientCertFilename)
	if err != nil {
		return k, err
	}
	clientKeyFilename := os.Getenv("KUBERNETES_CLIENT_KEY")
	if len(clientKeyFilename) == 0 {
		return k, errors.New("Missing env var KUBERNETES_CLIENT_KEY")
	}
	k.ClientKey, err = ioutil.ReadFile(clientKeyFilename)
	return k, err
}

func getClient() (client.Client, error) {
	config, err := rest.InClusterConfig()
	if err != nil {
		var kConf kubeConfig
		kConf, err = getKubeConfig()
		if err != nil {
			return nil, err
		}
		config, err = getRestConfig(kConf)
		if err != nil {
			return nil, err
		}
	}
	m, err := manager.New(config, manager.Options{})
	if err != nil {
		return nil, err
	}
	return m.GetClient(), nil
}

func getRestConfig(c kubeConfig) (*rest.Config, error) {
	cfg, err := getRestBaseConfig()
	if err != nil {
		return nil, err
	}
	cfg.Host = c.Addr
	cfg.TLSClientConfig = rest.TLSClientConfig{
		CAData:   c.CaCert,
		CertData: c.ClientCert,
		KeyData:  c.ClientKey,
	}
	cfg.Dial = (&net.Dialer{
		Timeout:   dialTimeout,
		KeepAlive: tcpKeepAlive,
	}).DialContext
	return cfg, nil
}

func getRestBaseConfig() (*rest.Config, error) {
	gv, err := schema.ParseGroupVersion("/v1")
	if err != nil {
		return nil, err
	}
	return &rest.Config{
		APIPath: "/api",
		ContentConfig: rest.ContentConfig{
			GroupVersion:         &gv,
			NegotiatedSerializer: serializer.DirectCodecFactory{CodecFactory: scheme.Codecs},
		},
		Timeout: apiTimeout,
	}, nil
}
