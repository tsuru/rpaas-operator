package cmd

import (
	"context"
	"fmt"
	"io/ioutil"
	"log"
	"net"
	"net/http"

	"github.com/pkg/errors"
	rpaasclient "github.com/tsuru/rpaas-operator/pkg/rpaas/client"
	"github.com/urfave/cli/v2"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/util/httpstream"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/tools/portforward"
	"k8s.io/client-go/transport/spdy"
)

type PortForward struct {
	Config          *rest.Config
	Clientset       kubernetes.Interface
	Name            string
	Labels          metav1.LabelSelector
	DestinationPort int
	ListenPort      int
	Namespace       string
	Ports           []string
	StopChan        chan struct{}
	ReadyChan       chan struct{}
}

func NewCmdPortForward() *cli.Command {
	return &cli.Command{
		Name:      "port-forward",
		Usage:     "",
		ArgsUsage: "[-s SERVICE][-p POD] [LOCALHOST] [-l LOCAL_PORT:]REMOTE_PORT",
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:    "service",
				Aliases: []string{"tsuru-service", "s"},
				Usage:   "the Tsuru service name",
			},
			&cli.StringFlag{
				Name:    "pod",
				Aliases: []string{"p"},
				Usage:   "pod name - if omitted, the first pod will be chosen",
			},
			&cli.StringFlag{
				Name:    "address",
				Aliases: []string{"localhost"},
				Usage:   "Addresses to listen on (comma separated). Only accepts IP addresses or localhost as a value. When localhost is supplied, rpaas-operator will try to bind on both 127.0.0.1 and ::1 and will fail if neither of these addresses are available to bind.",
			},
			&cli.StringFlag{
				Name:    "local_port",
				Aliases: []string{"l"},
				Usage:   "specify a local port",
			},
		},
		Before: setupClient,
		Action: runPortForward,
	}
}
func NewPortForwarder(namespace string, labels metav1.LabelSelector, port int) (*PortForward, error) {
	pf := &PortForward{
		Namespace:       namespace,
		Labels:          labels,
		DestinationPort: port,
	}

	var err error
	pf.Config, err = clientcmd.NewNonInteractiveDeferredLoadingClientConfig(
		clientcmd.NewDefaultClientConfigLoadingRules(),
		&clientcmd.ConfigOverrides{},
	).ClientConfig()
	if err != nil {
		return pf, errors.Wrap(err, "Could not load kubernetes configuration file")
	}

	pf.Clientset, err = kubernetes.NewForConfig(pf.Config)
	if err != nil {
		return pf, errors.Wrap(err, "Could not create kubernetes client")
	}

	return pf, nil
}

// Start a port forward to a pod - blocks until the tunnel is ready for use.
func (p *PortForward) Start(ctx context.Context) error {
	p.StopChan = make(chan struct{}, 1)
	readyChan := make(chan struct{}, 1)
	errChan := make(chan error, 1)

	listenPort, err := p.getListenPort()
	if err != nil {
		return errors.Wrap(err, "Could not find a port to bind to")
	}

	dialer, err := p.dialer(ctx)
	if err != nil {
		return errors.Wrap(err, "Could not create a dialer")
	}

	ports := []string{
		fmt.Sprintf("%d:%d", listenPort, 8888),
	}

	discard := ioutil.Discard
	pf, err := portforward.New(dialer, ports, p.StopChan, readyChan, discard, discard)
	if err != nil {
		return errors.Wrap(err, "Could not port forward into pod")
	}

	go func() {
		errChan <- pf.ForwardPorts()
	}()

	select {
	case err = <-errChan:
		return errors.Wrap(err, "Could not create port forward")
	case <-readyChan:
		return nil
	}

	return nil
}

// Stop a port forward.
func (p *PortForward) Stop() {
	p.StopChan <- struct{}{}
}

// Returns the port that the port forward should listen on.
// If ListenPort is set, then it returns ListenPort.
// Otherwise, it will call getFreePort() to find an open port.
func (p *PortForward) getListenPort() (int, error) {
	var err error

	if p.ListenPort == 0 {
		p.ListenPort, err = p.getFreePort()
	}

	return p.ListenPort, err
}

// Get a free port on the system by binding to port 0, checking
// the bound port number, and then closing the socket.
func (p *PortForward) getFreePort() (int, error) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return 0, err
	}

	port := listener.Addr().(*net.TCPAddr).Port
	err = listener.Close()
	if err != nil {
		return 0, err
	}

	return port, nil
}

// Create an httpstream.Dialer for use with portforward.New
func (p *PortForward) dialer(ctx context.Context) (httpstream.Dialer, error) {
	pod, err := p.getPodName(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "Could not get pod name")
	}

	url := p.Clientset.CoreV1().RESTClient().Post().
		Resource("pods").
		Name(pod).
		SubResource("portforward").URL()

	transport, upgrader, err := spdy.RoundTripperFor(p.Config)
	if err != nil {
		return nil, errors.Wrap(err, "Could not create round tripper")
	}

	dialer := spdy.NewDialer(upgrader, &http.Client{Transport: transport}, "POST", url)
	return dialer, nil
}

// Gets the pod name to port forward to, if Name is set, Name is returned. Otherwise,
// it will call findPodByLabels().
func (p *PortForward) getPodName(ctx context.Context) (string, error) {
	var err error
	if p.Name == "" {
		p.Name, err = p.findPodByLabels(ctx)
	}
	return p.Name, err
}

// Find the name of a pod by label, returns an error if the label returns
// more or less than one pod.
// It searches for the labels specified by labels.
func (p *PortForward) findPodByLabels(ctx context.Context) (string, error) {
	if len(p.Labels.MatchLabels) == 0 && len(p.Labels.MatchExpressions) == 0 {
		return "", errors.New("No pod labels specified")
	}

	pods, err := p.Clientset.CoreV1().Pods("").List(ctx, metav1.ListOptions{
		LabelSelector: metav1.FormatLabelSelector(&p.Labels),
		FieldSelector: fields.OneTermEqualSelector("status.phase", string(v1.PodRunning)).String(),
	})

	if err != nil {
		return "", errors.Wrap(err, "Listing pods in kubernetes")
	}

	formatted := metav1.FormatLabelSelector(&p.Labels)

	if len(pods.Items) == 0 {
		return "", errors.New(fmt.Sprintf("Could not find running pod for selector: labels \"%s\"", formatted))
	}

	if len(pods.Items) != 1 {
		return "", errors.New(fmt.Sprintf("Ambiguous pod: found more than one pod for selector: labels \"%s\"", formatted))
	}

	return pods.Items[0].ObjectMeta.Name, nil
}

func runPortForward(c *cli.Context) error {
	var ctx context.Context

	args := rpaasclient.PortForwardArgs{
		Pod:     c.String("pod"),
		Address: c.String("Address"),
		Port:    c.Int("8888"),
	}

	pf, err := NewPortForwarder("test", metav1.LabelSelector{
		MatchLabels: map[string]string{},
	}, args.Port)

	if err != nil {
		log.Fatal("error setting up port forwarder: ", err)
	}

	pf.Name = args.Pod

	err = pf.Start(ctx)
	if err != nil {
		log.Fatal("Error starting port forward:", err)
	}

	return nil
}
