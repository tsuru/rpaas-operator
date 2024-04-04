// Copyright 2024 tsuru authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package cmd

import (
	"fmt"

	"github.com/urfave/cli/v2"

	rpaasclient "github.com/tsuru/rpaas-operator/pkg/rpaas/client"
)

func NewCmdStart() *cli.Command {
	return &cli.Command{
		Name:  "start",
		Usage: "Starts instance if the current state is shutdown",
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
		Action: runStart,
	}
}

func runStart(c *cli.Context) error {
	client, err := getClient(c)
	if err != nil {
		return err
	}

	err = client.Start(c.Context, c.String("instance"))
	if err != nil {
		return err
	}

	fmt.Fprintf(c.App.Writer, "Started instance %s\n", formatInstanceName(c))
	return nil
}
