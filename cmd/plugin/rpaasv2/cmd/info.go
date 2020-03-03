// Copyright 2020 tsuru authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package cmd

import (
	"fmt"

	rpaasclient "github.com/tsuru/rpaas-operator/pkg/rpaas/client"
	"github.com/urfave/cli/v2"
)

func NewCmdInfo() *cli.Command {
	return &cli.Command{
		Name:  "info",
		Usage: "Retrieves information of the rpaas-operator instance given",
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
		Action: runInfo,
	}
}

func runInfo(c *cli.Context) error {
	client, err := getClient(c)
	if err != nil {
		return err
	}

	info := rpaasclient.InfoArgs{
		Instance: c.String("instance"),
		Service:  c.String("service"),
	}
	resp, err = client.Info(c.Context, info)
	if err != nil {
		return err
	}

	fmt.Sprintf("%v", resp.Body)

	return nil
}
