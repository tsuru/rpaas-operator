// Copyright 2019 tsuru authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package cmd

import (
	"encoding/json"
	"fmt"
	"io"
	"os"

	"github.com/olekukonko/tablewriter"
	"github.com/urfave/cli/v2"

	rpaasclient "github.com/tsuru/rpaas-operator/pkg/rpaas/client"
	clientTypes "github.com/tsuru/rpaas-operator/pkg/rpaas/client/types"
)

func NewCmdBlocks() *cli.Command {
	return &cli.Command{
		Name:  "blocks",
		Usage: "Manages raw NGINX configuration fragments",
		Subcommands: []*cli.Command{
			NewCmdDeleteBlock(),
			NewCmdListBlocks(),
			NewCmdUpdateBlock(),
		},
	}
}

func NewCmdUpdateBlock() *cli.Command {
	return &cli.Command{
		Name:    "update",
		Aliases: []string{"add"},
		Usage:   "Inserts raw NGINX configuration on a context",
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
				Name:    "server-name",
				Aliases: []string{"sn"},
				Usage:   "Optional, indicates that block belongs to specific server_name, ignoring it this block will apply to all server_names",
			},
			&cli.StringFlag{
				Name:     "name",
				Aliases:  []string{"context", "n"},
				Usage:    "the NGINX context name wherein the fragment will be injected (supported values: root, http, server, lua-server, lua-worker)",
				Required: true,
			},
			&cli.PathFlag{
				Name:     "content",
				Aliases:  []string{"content-file", "c"},
				Usage:    "path in the system to the NGINX configuration",
				Required: true,
			},
			&cli.BoolFlag{
				Name:  "extend",
				Usage: "if set, the content will be appended to the default block, only valid for server context",
			},
		},
		Before: setupClient,
		Action: runUpdateBlock,
	}
}

func runUpdateBlock(c *cli.Context) error {
	client, err := getClient(c)
	if err != nil {
		return err
	}

	content, err := os.ReadFile(c.Path("content"))
	if err != nil {
		return err
	}

	serverName := c.String("server-name")

	args := rpaasclient.UpdateBlockArgs{
		Instance:   c.String("instance"),
		Name:       c.String("name"),
		ServerName: serverName,
		Extend:     c.Bool("extend"),
		Content:    string(content),
	}
	err = client.UpdateBlock(c.Context, args)
	if err != nil {
		return err
	}

	if serverName == "" {
		fmt.Fprintf(c.App.Writer, "NGINX configuration fragment inserted at %q context\n", args.Name)
	} else {
		fmt.Fprintf(c.App.Writer, "NGINX configuration fragment inserted at %q context for server name %q\n", args.Name, serverName)

	}
	return nil
}

func NewCmdDeleteBlock() *cli.Command {
	return &cli.Command{
		Name:    "delete",
		Aliases: []string{"remove"},
		Usage:   "Removes raw NGINX configuration from a context",
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
				Name:    "server-name",
				Aliases: []string{"sn"},
				Usage:   "Optional, indicates that block belongs to specific server_name, ignoring it this block will apply to all server_names",
			},
			&cli.StringFlag{
				Name:     "name",
				Aliases:  []string{"context", "n"},
				Usage:    "the NGINX context name wherein the fragment is (supported values: root, http, server, lua-server, lua-worker)",
				Required: true,
			},
		},
		Before: setupClient,
		Action: runDeleteBlock,
	}
}

func runDeleteBlock(c *cli.Context) error {
	client, err := getClient(c)
	if err != nil {
		return err
	}

	serverName := c.String("server-name")

	args := rpaasclient.DeleteBlockArgs{
		Instance:   c.String("instance"),
		Name:       c.String("name"),
		ServerName: serverName,
	}
	err = client.DeleteBlock(c.Context, args)
	if err != nil {
		return err
	}

	if serverName == "" {
		fmt.Fprintf(c.App.Writer, "NGINX configuration at %q context removed\n", args.Name)
	} else {
		fmt.Fprintf(c.App.Writer, "NGINX configuration at %q context for server name %q removed\n", args.Name, serverName)
	}
	return nil
}

func NewCmdListBlocks() *cli.Command {
	return &cli.Command{
		Name:  "list",
		Usage: "Shows the NGINX configuration fragments on the instance",
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
				Name:    "raw-output",
				Aliases: []string{"r"},
				Usage:   "show as JSON instead of table format",
				Value:   false,
			},
		},
		Before: setupClient,
		Action: runListBlocks,
	}
}

func runListBlocks(c *cli.Context) error {
	client, err := getClient(c)
	if err != nil {
		return err
	}

	args := rpaasclient.ListBlocksArgs{Instance: c.String("instance")}
	blocks, err := client.ListBlocks(c.Context, args)
	if err != nil {
		return err
	}

	if c.Bool("raw-output") {
		return writeBlocksOnJSONFormat(c.App.Writer, blocks)
	}

	writeBlocksOnTableFormat(c.App.Writer, blocks)
	return nil
}

func writeBlocksOnTableFormat(w io.Writer, blocks []clientTypes.Block) {
	hasServerName := false
	for _, block := range blocks {
		if block.ServerName != "" {
			hasServerName = true
			break
		}
	}

	rows := [][]string{}
	for _, block := range blocks {
		row := []string{block.Name, block.Content}
		if hasServerName {
			row = append([]string{block.ServerName}, row...)
			row = append(row, checkedChar(block.Extend))
		}
		rows = append(rows, row)
	}

	table := tablewriter.NewWriter(w)

	headers := []string{"Context", "Configuration"}
	alignment := []int{tablewriter.ALIGN_LEFT, tablewriter.ALIGN_LEFT}
	if hasServerName {
		headers = append([]string{"Server Name"}, headers...)
		headers = append(headers, "Extend")
		alignment = append(alignment, tablewriter.ALIGN_LEFT, tablewriter.ALIGN_CENTER)
	}

	table.SetHeader(headers)
	table.SetAutoWrapText(false)
	table.SetAutoFormatHeaders(false)
	table.SetHeaderAlignment(tablewriter.ALIGN_LEFT)
	table.SetColumnAlignment(alignment)
	table.AppendBulk(rows)
	table.Render()
}

func writeBlocksOnJSONFormat(w io.Writer, blocks []clientTypes.Block) error {
	message, err := json.MarshalIndent(blocks, "", "\t")
	if err != nil {
		return err
	}

	fmt.Fprintln(w, string(message))
	return nil
}
