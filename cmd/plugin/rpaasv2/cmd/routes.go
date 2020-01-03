// Copyright 2019 tsuru authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"io"

	"github.com/olekukonko/tablewriter"
	rpaasclient "github.com/tsuru/rpaas-operator/pkg/rpaas/client"
	"github.com/urfave/cli/v2"
)

func NewCmdRoutes() *cli.Command {
	return &cli.Command{
		Name:  "routes",
		Usage: "Manages specific locations",
		Subcommands: []*cli.Command{
			NewCmdListRoutes(),
		},
	}
}

func NewCmdListRoutes() *cli.Command {
	return &cli.Command{
		Name:  "list",
		Usage: "Shows the routes in the instance",
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
				Usage:   "writes routes on JSON format",
				Value:   false,
			},
		},
		Action: runListRoutes,
	}
}

func runListRoutes(c *cli.Context) error {
	client, err := getRpaasClient(c)
	if err != nil {
		return err
	}

	args := rpaasclient.ListRoutesArgs{Instance: c.String("instance")}
	routes, _, err := client.ListRoutes(context.Background(), args)
	if err != nil {
		return err
	}

	if c.Bool("raw-output") {
		return writeRoutesOnJSONFormat(c.App.Writer, routes)
	}

	writeRoutesOnTableFormat(c.App.Writer, routes)
	return nil
}

func writeRoutesOnTableFormat(w io.Writer, routes []rpaasclient.Route) {
	data := [][]string{}
	for _, r := range routes {
		data = append(data, []string{r.Path, r.Destination, checkedChar(r.HTTPSOnly), r.Content})
	}

	table := tablewriter.NewWriter(w)
	table.SetHeader([]string{"Path", "Destination", "Force HTTPS?", "Configuration"})
	table.SetRowLine(true)
	table.SetAutoWrapText(false)
	table.SetColumnAlignment([]int{tablewriter.ALIGN_LEFT, tablewriter.ALIGN_CENTER, tablewriter.ALIGN_CENTER, tablewriter.ALIGN_LEFT})
	table.AppendBulk(data)
	table.Render()
}

func writeRoutesOnJSONFormat(w io.Writer, routes []rpaasclient.Route) error {
	message, err := json.MarshalIndent(routes, "", "\t")
	if err != nil {
		return err
	}

	fmt.Fprintln(w, string(message))
	return nil
}

func checkedChar(b bool) string {
	if b {
		return "✓"
	}

	return ""
}
