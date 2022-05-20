package cmd

import (
	rpaasclient "github.com/tsuru/rpaas-operator/pkg/rpaas/client"
	"github.com/urfave/cli/v2"
)

func NewCmdPortForward() *cli.Command {
	return &cli.Command{
		Name:      "port-forward",
		Usage:     "",
		ArgsUsage: "[-p POD] [-l LOCALHOST] [-dp LOCAL_PORT:][-rl REMOTE_PORT]",
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:    "instance",
				Aliases: []string{"tsuru-instance", "i"},
				Usage:   "the Tsuru instance name",
			},
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
				Aliases: []string{"localhost", "l"},
				Usage:   "Addresses to listen on (comma separated). Only accepts IP addresses or localhost as a value. When localhost is supplied, rpaas-operator will try to bind on both 127.0.0.1 and ::1 and will fail if neither of these addresses are available to bind.",
			},
			&cli.StringFlag{
				Name:    "destination-port",
				Aliases: []string{"dp"},
				Usage:   "specify a destined port",
			},
			&cli.StringFlag{
				Name:    "remote_port",
				Aliases: []string{"rp"},
				Usage:   "specify a remote port",
			},
		},
		Before: setupClient,
		Action: runPortForward,
	}
}

// func NewPortForwarder(namespace string, labels metav1.LabelSelector, port int) (*PortForward, error) {
// }

func runPortForward(c *cli.Context) error {
	client, err := getClient(c)
	if err != nil {
		return err
	}
	return client.StartPortForward(c.Context, rpaasclient.PortForwardArgs{
		Pod:             c.String("pod"),
		DestinationPort: c.Int("dp"),
		ListenPort:      c.Int("lp"),
		Instance:        c.String("instance"),
		Address:         c.String("localhost"),
	})
}
