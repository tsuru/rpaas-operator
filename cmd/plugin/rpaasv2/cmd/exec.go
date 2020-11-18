// Copyright 2020 tsuru authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package cmd

import (
	"fmt"
	"os"

	"github.com/gorilla/websocket"
	rpaasclient "github.com/tsuru/rpaas-operator/pkg/rpaas/client"
	"github.com/urfave/cli/v2"
	"k8s.io/kubectl/pkg/util/term"
)

func NewCmdExec() *cli.Command {
	return &cli.Command{
		Name:      "exec",
		Usage:     "Run a command in an instance",
		ArgsUsage: "[-p POD] [-c CONTAINER] [--] COMMAND [args...]",
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:    "service",
				Aliases: []string{"tsuru-service", "s"},
				Usage:   "the Tsuru service name",
			},
			&cli.StringFlag{
				Name:     "instance",
				Aliases:  []string{"tsuru-service-instance", "i"},
				Usage:    "the reverse proxy instance name",
				Required: true,
			},
			&cli.StringFlag{
				Name:    "pod",
				Aliases: []string{"p"},
				Usage:   "pod name - if omitted, the first pod will be chosen",
			},
			&cli.StringFlag{
				Name:    "container",
				Aliases: []string{"c"},
				Usage:   "container name - if omitted, the \"nginx\" container will be chosen",
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
		Action: runExec,
	}
}

func runExec(c *cli.Context) error {
	client, err := getClient(c)
	if err != nil {
		return err
	}

	var width, height uint16
	if ts := term.GetSize(os.Stdin.Fd()); ts != nil {
		width, height = ts.Width, ts.Height
	}

	args := rpaasclient.ExecArgs{
		Command:        c.Args().Slice(),
		Instance:       c.String("instance"),
		Pod:            c.String("pod"),
		Container:      c.String("container"),
		Interactive:    c.Bool("interactive"),
		TTY:            c.Bool("tty"),
		TerminalWidth:  width,
		TerminalHeight: height,
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
		conn, err := client.Exec(c.Context, args)
		if err != nil {
			return err
		}
		defer conn.Close()

		done := make(chan error, 1)
		go func() {
			defer close(done)
			for {
				mtype, message, nerr := conn.ReadMessage()
				if nerr != nil {
					closeErr, ok := nerr.(*websocket.CloseError)
					if !ok {
						done <- fmt.Errorf("ERROR: receveid an unexpected error while reading messages: %w", err)
						return
					}

					switch closeErr.Code {
					case websocket.CloseNormalClosure:
					case websocket.CloseInternalServerErr:
						done <- fmt.Errorf("ERROR: the command may not be executed as expected - reason: %s", closeErr.Text)
					default:
						done <- fmt.Errorf("ERROR: unexpected close error: %s", closeErr.Error())
					}

					return
				}

				switch mtype {
				case websocket.TextMessage, websocket.BinaryMessage:
					c.App.Writer.Write(message)
				}
			}
		}()
		err = <-done
		conn.WriteMessage(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseNormalClosure, ""))
		return err
	})
}
