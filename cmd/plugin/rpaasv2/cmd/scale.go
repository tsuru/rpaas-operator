// Copyright 2019 tsuru authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package cmd

import (
	"context"
	"fmt"

	rpaasclient "github.com/tsuru/rpaas-operator/pkg/rpaas/client"
	"github.com/urfave/cli/v2"
)

func NewCmdScale() *cli.Command {
	return &cli.Command{
		Name:  "scale",
		Usage: "Sets a new amount of desired replicas for an instance",
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
				Usage:    "the new desired number of replicas",
				Value:    -1,
				Required: true,
			},
		},
		Action: runScale,
	}
}

func runScale(c *cli.Context) error {
	client, err := getRpaasClient(c)
	if err != nil {
		return err
	}

	scale := rpaasclient.ScaleArgs{
		Instance: c.String("instance"),
		Replicas: int32(c.Int("replicas")),
	}
	_, err = client.Scale(context.Background(), scale)
	if err != nil {
		return fmt.Errorf("could not scale the instance on server: %w", err)
	}

	fmt.Fprintf(c.App.Writer, "%s scaled to %d replica(s)\n", formatInstanceName(c), scale.Replicas)
	return nil
}

func formatInstanceName(c *cli.Context) string {
	var prefix string
	if service := c.String("service"); service != "" {
		prefix = fmt.Sprintf("%s/", service)
	}

	return fmt.Sprintf("%s%s", prefix, c.String("instance"))
}
