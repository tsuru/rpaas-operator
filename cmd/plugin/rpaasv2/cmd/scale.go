// Copyright 2019 tsuru authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package cmd

import (
	"context"
	"fmt"

	"github.com/urfave/cli/v3"

	rpaasclient "github.com/tsuru/rpaas-operator/pkg/rpaas/client"
)

func NewCmdScale() *cli.Command {
	return &cli.Command{
		Name:  "scale",
		Usage: "Sets the number of replicas for an instance",
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
			&cli.IntFlag{
				Name:     "replicas",
				Aliases:  []string{"quantity", "q"},
				Usage:    "the desired replicas number",
				Value:    -1,
				Required: true,
			},
		},
		Before: setupClient,
		Action: runScale,
	}
}

func runScale(ctx context.Context, cmd *cli.Command) error {
	client, err := getClient(ctx)
	if err != nil {
		return err
	}

	scale := rpaasclient.ScaleArgs{
		Instance: cmd.String("instance"),
		Replicas: int32(cmd.Int("replicas")),
	}
	err = client.Scale(ctx, scale)
	if err != nil {
		return err
	}

	fmt.Fprintf(cmd.Root().Writer, "%s scaled to %d replica(s)\n", formatInstanceName(cmd), scale.Replicas)
	return nil
}

func formatInstanceName(cmd *cli.Command) string {
	var prefix string
	if service := cmd.String("service"); service != "" {
		prefix = fmt.Sprintf("%s/", service)
	}

	return fmt.Sprintf("%s%s", prefix, cmd.String("instance"))
}
