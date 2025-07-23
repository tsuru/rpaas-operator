// Copyright 2025 tsuru authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package cmd

import (
	"fmt"

	"github.com/urfave/cli/v2"
)

func NewCmdRestart() *cli.Command {
	return &cli.Command{
		Name:  "restart",
		Usage: "Restarts instance",
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
		},
		Before: setupClient,
		Action: runRestart,
	}
}

func runRestart(c *cli.Context) error {
	client, err := getClient(c)
	if err != nil {
		return err
	}

	err = client.Restart(c.Context, c.String("instance"))
	if err != nil {
		return err
	}

	fmt.Fprintf(c.App.Writer, "Restarting instance %s\n", formatInstanceName(c))
	return nil
}
