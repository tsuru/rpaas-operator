// Copyright 2024 tsuru authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package cmd

import (
	"context"
	"fmt"

	"github.com/urfave/cli/v3"
)

func NewCmdStop() *cli.Command {
	return &cli.Command{
		Name:  "stop",
		Usage: "Shutdown instance (halt autoscale and scale in all replicas)",
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
		Action: runStop,
	}
}

func runStop(ctx context.Context, cmd *cli.Command) error {
	client, err := getClient(ctx)
	if err != nil {
		return err
	}

	err = client.Stop(ctx, cmd.String("instance"))
	if err != nil {
		return err
	}

	fmt.Fprintf(cmd.Root().Writer, "Shutting down instance %s\n", formatInstanceName(cmd))
	return nil
}
