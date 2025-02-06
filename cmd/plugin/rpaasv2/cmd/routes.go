// Copyright 2019 tsuru authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package cmd

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/olekukonko/tablewriter"
	"github.com/urfave/cli/v2"

	rpaasclient "github.com/tsuru/rpaas-operator/pkg/rpaas/client"
	clientTypes "github.com/tsuru/rpaas-operator/pkg/rpaas/client/types"
)

func NewCmdRoutes() *cli.Command {
	return &cli.Command{
		Name:  "routes",
		Usage: "Manages application-layer routing (NGINX locations)",
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
		Usage:   "Removes a route from a path",
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
				Usage:   "Optional. Indicates this route belongs to a specific server_name. Not setting it will apply this block to all server_names",
			},
			&cli.StringFlag{
				Name:     "path",
				Aliases:  []string{"p"},
				Usage:    "path name",
				Required: true,
			},
		},
		Before: setupClient,
		Action: runDeleteRoute,
	}
}

func runDeleteRoute(c *cli.Context) error {
	client, err := getClient(c)
	if err != nil {
		return err
	}

	serverName := c.String("server-name")
	args := rpaasclient.DeleteRouteArgs{
		Instance:   c.String("instance"),
		ServerName: serverName,
		Path:       c.String("path"),
	}
	err = client.DeleteRoute(c.Context, args)
	if err != nil {
		return err
	}

	if serverName == "" {
		fmt.Fprintf(c.App.Writer, "Route %q deleted.\n", args.Path)
	} else {
		fmt.Fprintf(c.App.Writer, "Route %q deleted for server name %q.\n", args.Path, serverName)
	}

	return nil
}

func NewCmdListRoutes() *cli.Command {
	return &cli.Command{
		Name:  "list",
		Usage: "Shows the routes on the instance",
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
		Action: runListRoutes,
	}
}

func runListRoutes(c *cli.Context) error {
	client, err := getClient(c)
	if err != nil {
		return err
	}

	args := rpaasclient.ListRoutesArgs{Instance: c.String("instance")}
	routes, err := client.ListRoutes(c.Context, args)
	if err != nil {
		return err
	}

	if c.Bool("raw-output") {
		return writeRoutesOnJSONFormat(c.App.Writer, routes)
	}

	writeRoutesOnTableFormat(c.App.Writer, routes)
	return nil
}

func writeRoutesOnTableFormat(w io.Writer, routes []clientTypes.Route) {
	data := [][]string{}
	hasServerName := false

	for _, r := range routes {
		if r.ServerName != "" {
			hasServerName = true
			break
		}
	}
	for _, r := range routes {
		row := []string{r.Path, r.Destination, checkedChar(r.HTTPSOnly), r.Content}
		if hasServerName {
			row = append([]string{r.ServerName}, row...)
		}
		data = append(data, row)
	}

	table := tablewriter.NewWriter(w)
	headers := []string{"Path", "Destination", "Force HTTPS?", "Configuration"}

	if hasServerName {
		headers = append([]string{"Server Name"}, headers...)
	}
	table.SetHeader(headers)
	table.SetAutoWrapText(false)
	table.SetAutoFormatHeaders(false)
	table.SetHeaderAlignment(tablewriter.ALIGN_LEFT)
	alignments := []int{tablewriter.ALIGN_LEFT, tablewriter.ALIGN_LEFT, tablewriter.ALIGN_CENTER, tablewriter.ALIGN_LEFT}
	if hasServerName {
		alignments = append([]int{tablewriter.ALIGN_LEFT}, alignments...)
	}
	table.SetColumnAlignment(alignments)
	table.AppendBulk(data)
	table.Render()
}

func writeRoutesOnJSONFormat(w io.Writer, routes []clientTypes.Route) error {
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
		Usage:   "Inserts a NGINX location on a path",
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
				Usage:   "Optional. Indicates this route belongs to a specific server_name. Not setting it will apply this block to all server_names",
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
				Usage:   "host address that all request will be forwarded for",
			},
			&cli.BoolFlag{
				Name:  "https-only",
				Usage: "indicates whether should only be accessed over TLS (requires that destination be set)",
			},
			&cli.PathFlag{
				Name:    "content",
				Aliases: []string{"content-file", "c"},
				Usage:   "path in the system to the NGINX configuration (should not be combined with destination)",
			},
		},
		Before: setupClient,
		Action: runUpdateRoute,
	}
}

func runUpdateRoute(c *cli.Context) error {
	client, err := getClient(c)
	if err != nil {
		return err
	}

	content, err := fetchContentFile(c)
	if err != nil {
		return err
	}

	serverName := c.String("server-name")
	args := rpaasclient.UpdateRouteArgs{
		Instance:    c.String("instance"),
		ServerName:  serverName,
		Path:        c.String("path"),
		Destination: c.String("destination"),
		HTTPSOnly:   c.Bool("https-only"),
		Content:     string(content),
	}
	err = client.UpdateRoute(c.Context, args)
	if err != nil {
		return err
	}

	if serverName == "" {
		fmt.Fprintf(c.App.Writer, "Route %q updated.\n", args.Path)
	} else {
		fmt.Fprintf(c.App.Writer, "Route %q updated for server name %q.\n", args.Path, serverName)
	}
	return nil
}

func fetchContentFile(c *cli.Context) ([]byte, error) {
	contentFile := c.Path("content")
	if contentFile == "" {
		return nil, nil
	}
	content, err := os.ReadFile(contentFile)
	if os.IsNotExist(err) &&
		strings.HasPrefix(contentFile, "@") {
		return os.ReadFile(strings.TrimPrefix(contentFile, "@"))
	}

	return content, err
}
