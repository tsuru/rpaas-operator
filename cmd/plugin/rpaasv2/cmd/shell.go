// Copyright 2020 tsuru authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package cmd

import (
	"context"
	"fmt"
	"os"

	"github.com/gorilla/websocket"
	"github.com/urfave/cli/v3"
	"k8s.io/kubectl/pkg/util/term"

	rpaasclient "github.com/tsuru/rpaas-operator/pkg/rpaas/client"
)

func NewCmdShell() *cli.Command {
	return &cli.Command{
		Name:      "shell",
		Usage:     "Opens a remote shell inside unit",
		ArgsUsage: "[-p POD] [-c CONTAINER]",
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
		},
		Before: setupClient,
		Action: runShell,
	}
}

func runShell(ctx context.Context, cmd *cli.Command) error {
	client, err := getClient(ctx)
	if err != nil {
		return err
	}

	var width, height uint16
	if ts := term.GetSize(os.Stdin.Fd()); ts != nil {
		width, height = ts.Width, ts.Height
	}

	args := rpaasclient.ExecArgs{
		Command:        []string{"bash"},
		Instance:       cmd.String("instance"),
		Pod:            cmd.String("pod"),
		Container:      cmd.String("container"),
		Interactive:    true,
		TTY:            true,
		TerminalWidth:  width,
		TerminalHeight: height,
	}

	if args.Interactive {
		args.In = os.Stdin
	}

	tty := &term.TTY{
		In:  args.In,
		Out: cmd.Root().Writer,
		Raw: args.TTY,
	}
	return tty.Safe(func() error {
		conn, err := client.Exec(ctx, args)
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
					cmd.Root().Writer.Write(message)
				}
			}
		}()
		err = <-done
		conn.WriteMessage(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseNormalClosure, ""))
		return err
	})
}
