package cmd

import (
	"fmt"
	"log"

	"github.com/tsuru/rpaas-operator/internal/web/target"
	rpaasclient "github.com/tsuru/rpaas-operator/pkg/rpaas/client"
	"github.com/urfave/cli/v2"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

type PortForward struct {
	Config          *rest.Config
	Clientset       kubernetes.Interface
	Name            string
	Labels          metav1.LabelSelector
	DestinationPort string
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

// func NewPortForwarder(namespace string, labels metav1.LabelSelector, port int) (*PortForward, error) {
// }

func runPortForward(c *cli.Context) error {
	ctx := c.Context
	var err error

	args := rpaasclient.PortForwardArgs{
		Pod:     c.String("pod"),
		Address: c.String("localhost"),
		Port:    c.Int("8888"),
	}
	Ports := []string{
		fmt.Sprintf("%d:%d", args.Port, 8888),
	}
	pf, err := target.NewPortForwarder("my-pod", Ports)
	if err != nil {
		return err
	}
	err = pf.Start(ctx)
	if err != nil {
		log.Fatal("Error starting port forward:", err)
	}
	return nil
}
