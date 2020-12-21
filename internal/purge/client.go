package purge

import (
	"os"

	"github.com/tsuru/rpaas-operator/pkg/observability"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
)

func NewK8S() (kubernetes.Interface, error) {
	kubeconfig := os.Getenv("KUBECONFIG")

	config, err := clientcmd.BuildConfigFromFlags("", kubeconfig)
	if err != nil {
		return nil, err
	}
	config.WrapTransport = observability.OpentracingTransport

	return kubernetes.NewForConfig(config)
}
