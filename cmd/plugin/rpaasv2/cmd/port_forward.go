package cmd

import (
	"log"
	"os"
	"time"

	rpaasclient "github.com/tsuru/rpaas-operator/pkg/rpaas/client"
	"github.com/urfave/cli/v2"
	"k8s.io/kubectl/pkg/util/term"
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
			&cli.BoolFlag{
				Name:    "interactive",
				Aliases: []string{"I", "stdin"},
				Usage:   "pass STDIN to container",
			},
			&cli.BoolFlag{
				Name:    "tty",
				Aliases: []string{"t"},
				Usage:   "allocate a pseudo-TTY",
			},
		},
		Before: setupClient,
		Action: runPortForward,
	}
}

func runPortForward(c *cli.Context) error {
	client, err := getClient(c)
	if err != nil {
		return err
	}
	var width, height uint16
	if ts := term.GetSize(os.Stdin.Fd()); ts != nil {
		width, height = ts.Width, ts.Height
	}
	args := rpaasclient.PortForwardArgs{
		Pod:             c.String("pod"),
		DestinationPort: 8080,
		ListenPort:      80,
		Instance:        c.String("instance"),
		Address:         c.String("localhost"),
		Interactive:     c.Bool("interactive"),
		TTY:             c.Bool("TTY"),
		TerminalWidth:   width,
		TerminalHeight:  height,
	}
	if args.Interactive {
		args.In = os.Stdin
	}

	tty := &term.TTY{
		In:  args.In,
		Out: c.App.Writer,
		Raw: args.TTY,
	}
	return tty.Safe(func() error {
		pf, err := client.StartPortForward(c.Context, args)

		if err != nil {
			return err
		}
		err = pf.Start(c.Context)
		if err != nil {
			log.Fatal("Error starting port forward:", err)
		}
		log.Printf("Started tunnel on %d\n", pf.ListenPort)
		time.Sleep(60 * time.Second)
		return nil
	})
}
