// Copyright 2020 tsuru authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package cmd

import (
	"fmt"

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
		Usage: "removes all occurrences of the specified cache path",
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
				Usage:    "the route whose path shall be purged",
				Required: true,
			},
			&cli.BoolFlag{
				Name:    "preserve",
				Aliases: []string{"preserve-path"},
				Usage:   "specifies whether a request to purge/<protocol>/<purge Path> should be made",
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
		Instance: c.String("instance"),
		Path:     c.String("path"),
		Preserve: c.Bool("preserve"),
	}

	_, err = client.CachePurge(c.Context, purgeArgs)
	if err != nil {
		return err
	}

	fmt.Fprintf(c.App.Writer, "Cache of %s purged\n", formatInstanceName(c))
	return nil
}
