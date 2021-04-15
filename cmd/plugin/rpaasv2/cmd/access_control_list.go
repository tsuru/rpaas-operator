// Copyright 2021 tsuru authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package cmd

import (
	"fmt"
	"io"
	"strconv"

	"github.com/olekukonko/tablewriter"
	"github.com/tsuru/rpaas-operator/pkg/rpaas/client/types"
	"github.com/urfave/cli/v2"
)

func NewCmdAccessControlList() *cli.Command {
	return &cli.Command{
		Name:  "acl",
		Usage: "Manages ACL of rpaas instances",
		Subcommands: []*cli.Command{
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

			&cli.StringFlag{
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

			&cli.StringFlag{
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

func runAddAccessControlList(c *cli.Context) error {
	client, err := getClient(c)
	if err != nil {
		return err
	}

	instance := c.String("instance")
	host := c.String("host")
	port := c.Int("port")

	err = client.AddAccessControlList(c.Context, instance, host, port)
	if err != nil {
		return err
	}

	fmt.Fprintf(c.App.Writer, "Successfully added %s:%d to %s ACL.\n", host, port, formatInstanceName(c))
	return nil
}

func runListAccessControlList(c *cli.Context) error {
	client, err := getClient(c)
	if err != nil {
		return err
	}

	instance := c.String("instance")
	acl, err := client.ListAccessControlList(c.Context, instance)
	if err != nil {
		return err
	}

	writeAccessControlList(c.App.Writer, acl)
	return nil
}

func runRemoveAccessControlList(c *cli.Context) error {
	client, err := getClient(c)
	if err != nil {
		return err
	}

	instance := c.String("instance")
	host := c.String("host")
	port := c.Int("port")

	err = client.RemoveAccessControlList(c.Context, instance, host, port)
	if err != nil {
		return err
	}

	fmt.Fprintf(c.App.Writer, "Successfully removed %s:%d to %s ACL.\n", host, port, formatInstanceName(c))
	return nil
}

func writeAccessControlList(w io.Writer, acl *types.AccessControlList) {
	if acl == nil {
		return
	}
	table := tablewriter.NewWriter(w)
	table.SetHeader([]string{"Host", "Port"})
	table.SetAutoFormatHeaders(false)
	table.SetHeaderAlignment(tablewriter.ALIGN_LEFT)
	table.SetAutoWrapText(true)
	table.SetRowLine(false)

	for _, item := range acl.Items {
		port := ""
		if item.Port != nil {
			port = strconv.Itoa(*item.Port)
		}
		table.Append([]string{item.Host, port})
	}

	table.Render()
}
