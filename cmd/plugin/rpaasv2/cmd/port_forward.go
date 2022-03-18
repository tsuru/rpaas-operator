package cmd

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"os/signal"

	rpaasclient "github.com/tsuru/rpaas-operator/pkg/rpaas/client"
	"github.com/urfave/cli/v2"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	corev1client "k8s.io/client-go/kubernetes/typed/core/v1"
	restclient "k8s.io/client-go/rest"
	"k8s.io/client-go/tools/portforward"
	"k8s.io/client-go/transport/spdy"
	cmdutil "k8s.io/kubectl/pkg/cmd/util"
	"k8s.io/kubectl/pkg/polymorphichelpers"
	"k8s.io/kubectl/pkg/scheme"
)

type PortForwardOptions struct {
	Namespace     string
	PodName       string
	RESTClient    *restclient.RESTClient
	Config        *restclient.Config
	PodClient     corev1client.PodsGetter
	Address       []string
	Ports         []string
	PortForwarder portForwarder
	StopChannel   chan struct{}
	ReadyChannel  chan struct{}
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

type portForwarder interface {
	ForwardPorts(method string, url *url.URL, opts PortForwardOptions) error
}
type defaultPortForwarder struct {
	genericclioptions.IOStreams
}

func (f *defaultPortForwarder) ForwardPorts(method string, url *url.URL, opts PortForwardOptions) error {
	transport, upgrader, err := spdy.RoundTripperFor(opts.Config)
	if err != nil {
		return err
	}
	dialer := spdy.NewDialer(upgrader, &http.Client{Transport: transport}, method, url)
	fw, err := portforward.NewOnAddresses(dialer, opts.Address, opts.Ports, opts.StopChannel, opts.ReadyChannel, f.Out, f.ErrOut)
	if err != nil {
		return err
	}
	return fw.ForwardPorts()
}

func (o *PortForwardOptions) Complete(f cmdutil.Factory, cmd *cli.Command, args rpaasclient.PortForwardArgs) error {
	var err error
	o.PodName = args.Pod
	if len(o.PodName) == 0 && len(args.Pod) == 0 {
		println("POD is required for port-forward")
		return err
	}
	o.Ports = args.Port

	o.Address = append(o.Address, args.Address)

	builder := f.NewBuilder().WithScheme(scheme.Scheme, scheme.Scheme.PreferredVersionAllGroups()...).ContinueOnError()

	resourceName := o.PodName
	builder.ResourceNames("pod", resourceName)

	obj, err := builder.Do().Object()
	if err != nil {
		return err
	}

	forwablePod, err := polymorphichelpers.AttachablePodForObjectFn(f, obj, 0)

	o.PodName = forwablePod.Name

	clientset, err := f.KubernetesClientSet()
	if err != nil {
		return err
	}
	o.PodClient = clientset.CoreV1()

	o.Config, err = f.ToRESTConfig()
	if err != nil {
		return err
	}
	o.RESTClient, err = f.RESTClient()
	if err != nil {
		return err
	}

	o.StopChannel = make(chan struct{}, 1)
	o.ReadyChannel = make(chan struct{})
	return nil
}

func (o PortForwardOptions) Validate() error {
	if len(o.PodName) == 0 {
		return fmt.Errorf("pod name must be specified")
	}

	if len(o.Ports) < 1 {
		return fmt.Errorf("at least 1 PORT is required for port-forward")
	}

	if o.PortForwarder == nil || o.PodClient == nil || o.RESTClient == nil || o.Config == nil {
		return fmt.Errorf("client, client config, restClient, and portforwarder must be provided")
	}
	return nil
}

func runPortForward(c *cli.Context) error {
	var streams genericclioptions.IOStreams
	var cmd *cli.Command
	var f cmdutil.Factory
	opts := &PortForwardOptions{PortForwarder: &defaultPortForwarder{
		IOStreams: streams},
	}
	client, err := getClient(c)
	if err != nil {
		return err
	}

	println(client)

	args := rpaasclient.PortForwardArgs{
		Pod:     c.String("pod"),
		Address: c.String("Address"),
		Port:    c.StringSlice("8888"),
	}

	opts.Complete(f, cmd, args)
	opts.portForwa()

	return nil
}

func (o PortForwardOptions) portForwa() error {
	pod, err := o.PodClient.Pods("").Get(context.TODO(), o.PodName, metav1.GetOptions{})
	if err != nil {
		return err
	}
	if pod.Status.Phase != corev1.PodRunning {
		return fmt.Errorf("unable to forward port because pod is not running. Current status=%v", pod.Status.Phase)
	}

	signals := make(chan os.Signal, 1)
	signal.Notify(signals, os.Interrupt)
	defer signal.Stop(signals)

	go func() {
		<-signals
		if o.StopChannel != nil {
			close(o.StopChannel)
		}
	}()

	req := o.RESTClient.Post().
		Resource("pods").
		Name(pod.Name).
		SubResource("portforward")

	return o.PortForwarder.ForwardPorts("POST", req.URL(), o)
}
