import (
	"fmt"

	"github.com/urfave/cli/v2"
)

// Copyright 2020 tsuru authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

func NewCmdCache() *cli.Command {
	return &cli.Command{
		Name:  "cache",
		Usage: "TBI",
		Subcommands: []*cli.Command{
			NewCmdCachePurge(),
		},
	}
}

func NewCmdCachePurge() *cli.Command {
	return &cli.Command{
		Name:  "purge",
		Usage: "TBI",
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
				Usage:    "TBI",
				Required: true,
			},
			&cli.BoolFlag{
				Name:    "preserve",
				Aliases: "preserve-path",
				Usage:   "TBI",
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
	_, err = client.CachePurge(c.Context, updateArgs)
	if err != nil {
		return err
	}

	fmt.Fprintf(c.App.Writer, "Cache of %s purged\n", formatInstanceName(c))
	return nil
}

