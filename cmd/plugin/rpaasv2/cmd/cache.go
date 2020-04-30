// Copyright 2020 tsuru authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package cmd

import (
	"fmt"
	"io/ioutil"

	rpaasclient "github.com/tsuru/rpaas-operator/pkg/rpaas/client"
	"github.com/urfave/cli/v2"
)

func NewCmdCache() *cli.Command {
	return &cli.Command{
		Name:  "cache",
		Usage: "Manages cache settings of an instance",
		Subcommands: []*cli.Command{
			NewCmdCachePurge(),
		},
	}
}

func NewCmdCachePurge() *cli.Command {
	return &cli.Command{
		Name:  "purge",
		Usage: "Removes outdated cached files from the cache",
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
				Name:     "path",
				Usage:    "the resource path that is going to be removed",
				Required: true,
			},
			&cli.BoolFlag{
				Name:    "preserve-path",
				Aliases: []string{"p"},
				Usage:   "specifies whether the resource path should not be modified (read as the whole cache key)",
			},
		},

		Before: setupClient,
		Action: runCachePurge,
	}
}

func runCachePurge(c *cli.Context) error {
	client, err := getClient(c)
	if err != nil {
		return err
	}

	purgeArgs := rpaasclient.CachePurgeArgs{
		Instance:     c.String("instance"),
		Path:         c.String("path"),
		PreservePath: c.Bool("preserve"),
	}

	resp, err := client.CachePurge(c.Context, purgeArgs)
	if err != nil {
		return err
	}

	bodyBytes, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return err
	}
	fmt.Fprintf(c.App.Writer, "%s", string(bodyBytes))

	return nil
}
