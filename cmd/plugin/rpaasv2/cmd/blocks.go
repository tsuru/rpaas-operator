// Copyright 2019 tsuru authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"

	"github.com/olekukonko/tablewriter"
	rpaasclient "github.com/tsuru/rpaas-operator/pkg/rpaas/client"
	"github.com/urfave/cli/v2"
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
		},
		Action: runUpdateBlock,
	}
}

func runUpdateBlock(c *cli.Context) error {
	client, err := getClient(c)
	if err != nil {
		return err
	}

	content, err := ioutil.ReadFile(c.Path("content"))
	if err != nil {
		return err
	}

	args := rpaasclient.UpdateBlockArgs{
		Instance: c.String("instance"),
		Name:     c.String("name"),
		Content:  string(content),
	}
	_, err = client.UpdateBlock(context.Background(), args)
	if err != nil {
		return err
	}

	fmt.Fprintf(c.App.Writer, "NGINX configuration fragment inserted at %q context\n", args.Name)
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
				Name:     "name",
				Aliases:  []string{"context", "n"},
				Usage:    "the NGINX context name wherein the fragment is (supported values: root, http, server, lua-server, lua-worker)",
				Required: true,
			},
		},
		Action: runDeleteBlock,
	}
}

func runDeleteBlock(c *cli.Context) error {
	client, err := getClient(c)
	if err != nil {
		return err
	}

	args := rpaasclient.DeleteBlockArgs{
		Instance: c.String("instance"),
		Name:     c.String("name"),
	}
	_, err = client.DeleteBlock(context.Background(), args)
	if err != nil {
		return err
	}

	fmt.Fprintf(c.App.Writer, "NGINX configuration at %q context removed\n", args.Name)
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
		Action: runListBlocks,
	}
}

func runListBlocks(c *cli.Context) error {
	client, err := getClient(c)
	if err != nil {
		return err
	}

	args := rpaasclient.ListBlocksArgs{Instance: c.String("instance")}
	blocks, _, err := client.ListBlocks(context.Background(), args)
	if err != nil {
		return err
	}

	if c.Bool("raw-output") {
		return writeBlocksOnJSONFormat(c.App.Writer, blocks)
	}

	writeBlocksOnTableFormat(c.App.Writer, blocks)
	return nil
}

func writeBlocksOnTableFormat(w io.Writer, blocks []rpaasclient.Block) {
	data := [][]string{}
	for _, block := range blocks {
		data = append(data, []string{block.Name, block.Content})
	}

	table := tablewriter.NewWriter(w)
	table.SetHeader([]string{"Context", "Configuration"})
	table.SetAutoWrapText(false)
	table.AppendBulk(data)
	table.Render()
}

func writeBlocksOnJSONFormat(w io.Writer, blocks []rpaasclient.Block) error {
	message, err := json.MarshalIndent(blocks, "", "\t")
	if err != nil {
		return err
	}

	fmt.Fprintln(w, string(message))
	return nil
}
