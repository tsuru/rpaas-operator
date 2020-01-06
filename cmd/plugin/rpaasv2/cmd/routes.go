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

func NewCmdRoutes() *cli.Command {
	return &cli.Command{
		Name:  "routes",
		Usage: "Manages specific locations",
		Subcommands: []*cli.Command{
			NewCmdDeleteRoute(),
			NewCmdListRoutes(),
			NewCmdUpdateRoute(),
		},
	}
}

func NewCmdDeleteRoute() *cli.Command {
	return &cli.Command{
		Name:    "delete",
		Aliases: []string{"remove"},
		Usage:   "Removes a route from an instance",
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
				Aliases:  []string{"p"},
				Usage:    "path name",
				Required: true,
			},
		},
		Action: runDeleteRoute,
	}
}

func runDeleteRoute(c *cli.Context) error {
	client, err := getRpaasClient(c)
	if err != nil {
		return err
	}

	args := rpaasclient.DeleteRouteArgs{
		Instance: c.String("instance"),
		Path:     c.String("path"),
	}
	_, err = client.DeleteRoute(context.Background(), args)
	if err != nil {
		return err
	}

	fmt.Fprintf(c.App.Writer, "Route %q deleted.\n", args.Path)
	return nil
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

func NewCmdUpdateRoute() *cli.Command {
	return &cli.Command{
		Name:    "update",
		Aliases: []string{"add"},
		Usage:   "Inserts a new location into instance",
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
				Aliases:  []string{"p"},
				Usage:    "path name",
				Required: true,
			},
			&cli.StringFlag{
				Name:    "destination",
				Aliases: []string{"d"},
				Usage:   "",
			},
			&cli.BoolFlag{
				Name:  "https-only",
				Usage: "indicates whether path should only be accessed over HTTPS",
			},
			&cli.PathFlag{
				Name:    "content",
				Aliases: []string{"content-file", "c"},
				Usage:   "path in the system where the NGINX configuration fragment is located",
			},
		},
		Action: runUpdateRoute,
	}
}

func runUpdateRoute(c *cli.Context) error {
	client, err := getRpaasClient(c)
	if err != nil {
		return err
	}

	var content []byte
	if contentFile := c.Path("content"); contentFile != "" {
		content, err = ioutil.ReadFile(contentFile)
		if err != nil {
			return err
		}
	}

	args := rpaasclient.UpdateRouteArgs{
		Instance:    c.String("instance"),
		Path:        c.String("path"),
		Destination: c.String("destination"),
		HTTPSOnly:   c.Bool("https-only"),
		Content:     string(content),
	}
	_, err = client.UpdateRoute(context.Background(), args)
	if err != nil {
		return err
	}

	fmt.Fprintf(c.App.Writer, "Route %q updated.\n", args.Path)
	return nil
}
