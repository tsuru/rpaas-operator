// Copyright 2024 tsuru authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package cmd

import (
	"fmt"
	"io"

	"github.com/tsuru/rpaas-operator/pkg/rpaas/client/types"
	"github.com/urfave/cli/v2"
)

func NewCmdMetadata() *cli.Command {
	return &cli.Command{
		Name:  "metadata",
		Usage: "Manages metadata information of rpaasv2 instances",
		Subcommands: []*cli.Command{
			NewCmdGetMetadata(),
			NewCmdSetMetadata(),
			NewCmdUnsetMetadata(),
		},
	}
}

func NewCmdGetMetadata() *cli.Command {
	return &cli.Command{
		Name:  "get",
		Usage: "Shows metadata information of an instance",
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
			&cli.BoolFlag{
				Name:  "json",
				Usage: "show as JSON instead of go template format",
				Value: false,
			},
		},
		Before: setupClient,
		Action: runGetMetadata,
	}
}

func writeMetadata(w io.Writer, metadata *types.Metadata) {
	if len(metadata.Labels) > 0 {
		fmt.Fprintf(w, "Labels:\n")
		for _, v := range metadata.Labels {
			fmt.Fprintf(w, "  %s: %s\n", v.Name, v.Value)
		}
	}

	if len(metadata.Annotations) > 0 {
		fmt.Fprintf(w, "Annotations:\n")
		for _, v := range metadata.Annotations {
			fmt.Fprintf(w, "  %s: %s\n", v.Name, v.Value)
		}
	}
}

func runGetMetadata(c *cli.Context) error {
	client, err := getClient(c)
	if err != nil {
		return err
	}

	metadata, err := client.GetMetadata(c.Context, c.String("instance"))
	if err != nil {
		return err
	}

	if outputAsJSON := c.Bool("json"); outputAsJSON {
		return writeJSON(c.App.Writer, metadata)
	}

	writeMetadata(c.App.Writer, metadata)
	return nil
}

func NewCmdSetMetadata() *cli.Command {
	return &cli.Command{
		Name:  "set",
		Usage: "Sets metadata information of an instance",
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
			&cli.BoolFlag{
				Name:  "json",
				Usage: "show as JSON instead of go template format",
				Value: false,
			},
		},
		Before: setupClient,
		Action: runSetMetadata,
	}
}

func NewCmdUnsetMetadata() *cli.Command {
	return &cli.Command{
		Name:  "unset",
		Usage: "Unsets metadata information of an instance",
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
			&cli.BoolFlag{
				Name:  "json",
				Usage: "show as JSON instead of go template format",
				Value: false,
			},
		},
		Before: setupClient,
		Action: runSetMetadata,
	}
}

func runSetMetadata(c *cli.Context) error {
	_, err := getClient(c)
	if err != nil {
		return err
	}

	return nil
}
