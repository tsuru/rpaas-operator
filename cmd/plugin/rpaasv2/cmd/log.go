// Copyright 2021 tsuru authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package cmd

import (
	rpaasclient "github.com/tsuru/rpaas-operator/pkg/rpaas/client"
	"github.com/urfave/cli/v2"
)

func NewCmdLogs() *cli.Command {
	return &cli.Command{
		Name:    "logs",
		Usage:   "Fetches and prints logs from instance pods",
		Aliases: []string{"log"},
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
				Name:     "pod",
				Aliases:  []string{"p"},
				Usage:    "specific pod to log from",
				Required: false,
			},
			&cli.PathFlag{
				Name:     "container",
				Aliases:  []string{"c"},
				Usage:    "specific container to log from",
				Required: false,
			},
			&cli.IntFlag{
				Name:     "lines",
				Aliases:  []string{"l"},
				Usage:    "number of earlier log lines to show",
				Required: false,
			},
			&cli.DurationFlag{
				Name:     "since",
				Usage:    "only return logs newer than a relative duration like 5s, 2m, or 3h. Defaults to last 24 hours.",
				Required: false,
			},
			&cli.BoolFlag{
				Name:     "follow",
				Aliases:  []string{"f"},
				Usage:    "specify if the logs should be streamed",
				Required: false,
			},
			&cli.BoolFlag{
				Name:     "timestamp",
				Aliases:  []string{"with-timestamp"},
				Usage:    "include timestamps on each line in the log output",
				Required: false,
				Value:    true,
			},
			&cli.BoolFlag{
				Name:     "color",
				Usage:    "defines whether or not to display colorful output. Defaults to true.",
				Required: false,
				Value:    true,
			},
		},
		Before: setupClient,
		Action: runLogRpaas,
	}
}

func runLogRpaas(c *cli.Context) error {
	client, err := getClient(c)
	if err != nil {
		return err
	}

	return client.Log(c.Context, rpaasclient.LogArgs{
		Instance:      c.String("instance"),
		Lines:         c.Int("lines"),
		Since:         c.Duration("since"),
		Follow:        c.Bool("follow"),
		WithTimestamp: c.Bool("timestamp"),
		Pod:           c.String("pod"),
		Container:     c.String("container"),
		Color:         c.Bool("color"),
	})
}
