// Copyright 2021 tsuru authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package cmd

import (
	"bytes"
	"context"
	"fmt"
	"strconv"

	"github.com/olekukonko/tablewriter"
	"github.com/olekukonko/tablewriter/tw"
	"github.com/urfave/cli/v3"

	"github.com/tsuru/rpaas-operator/pkg/rpaas/client/types"
)

func NewCmdAccessControlList() *cli.Command {
	return &cli.Command{
		Name:  "acl",
		Usage: "Manages ACL of rpaas instances",
		Commands: []*cli.Command{
			NewCmdAddAccessControlList(),
			NewCmdListAccessControlList(),
			NewCmdRemoveAccessControlList(),
		},
	}
}

func NewCmdAddAccessControlList() *cli.Command {
	return &cli.Command{
		Name:    "add",
		Aliases: []string{"set"},
		Usage:   "Add host and port to rpaas instance ACL",
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
				Name:     "host",
				Aliases:  []string{"hostname", "H"},
				Usage:    "The hostname or IP of destination target",
				Required: true,
			},

			&cli.IntFlag{
				Name:     "port",
				Aliases:  []string{"p"},
				Usage:    "The number of destination port",
				Required: true,
			},
		},
		Before: setupClient,
		Action: runAddAccessControlList,
	}
}

func NewCmdListAccessControlList() *cli.Command {
	return &cli.Command{
		Name:    "list",
		Aliases: []string{"get"},
		Usage:   "Get hosts and ports from rpaas instance ACL",
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
		Action: runListAccessControlList,
	}
}

func NewCmdRemoveAccessControlList() *cli.Command {
	return &cli.Command{
		Name:    "remove",
		Aliases: []string{"delete"},
		Usage:   "Remove host and port from rpaas instance ACL",
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
				Name:     "host",
				Aliases:  []string{"hostname", "H"},
				Usage:    "The hostname or IP of destination target",
				Required: true,
			},

			&cli.IntFlag{
				Name:     "port",
				Aliases:  []string{"p"},
				Usage:    "The number of destination port",
				Required: true,
			},
		},
		Before: setupClient,
		Action: runRemoveAccessControlList,
	}
}

func runAddAccessControlList(ctx context.Context, cmd *cli.Command) error {
	client, err := getClient(ctx)
	if err != nil {
		return err
	}

	instance := cmd.String("instance")
	host := cmd.String("host")
	port := int(cmd.Int("port"))

	err = client.AddAccessControlList(ctx, instance, host, port)
	if err != nil {
		return err
	}

	fmt.Fprintf(cmd.Root().Writer, "Successfully added %s:%d to %s ACL.\n", host, port, formatInstanceName(cmd))
	return nil
}

func runListAccessControlList(ctx context.Context, cmd *cli.Command) error {
	client, err := getClient(ctx)
	if err != nil {
		return err
	}

	instance := cmd.String("instance")
	acls, err := client.ListAccessControlList(ctx, instance)
	if err != nil {
		return err
	}

	fmt.Fprint(cmd.Root().Writer, writeAccessControlListOnTableFormat(acls))
	return nil
}

func runRemoveAccessControlList(ctx context.Context, cmd *cli.Command) error {
	client, err := getClient(ctx)
	if err != nil {
		return err
	}

	instance := cmd.String("instance")
	host := cmd.String("host")
	port := int(cmd.Int("port"))

	err = client.RemoveAccessControlList(ctx, instance, host, port)
	if err != nil {
		return err
	}

	fmt.Fprintf(cmd.Root().Writer, "Successfully removed %s:%d from %s ACL.\n", host, port, formatInstanceName(cmd))
	return nil
}

func writeAccessControlListOnTableFormat(acls []types.AllowedUpstream) string {
	if len(acls) == 0 {
		return ""
	}

	var buffer bytes.Buffer
	table := newTable(&buffer,
		tablewriter.WithRowAutoWrap(tw.WrapNormal),
		tablewriter.WithRowAlignmentConfig(tw.CellAlignment{
			PerColumn: []tw.Align{tw.AlignLeft, tw.AlignRight},
		}),
	)
	table.Header("Host", "Port")

	for _, acl := range acls {
		var port string
		if acl.Port > 0 {
			port = strconv.Itoa(acl.Port)
		}

		table.Append(acl.Host, port)
	}

	table.Render()

	return buffer.String()
}
