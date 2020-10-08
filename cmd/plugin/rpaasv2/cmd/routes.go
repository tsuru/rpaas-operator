// Copyright 2019 tsuru authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package cmd

import (
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"strings"

	"github.com/olekukonko/tablewriter"
	rpaasclient "github.com/tsuru/rpaas-operator/pkg/rpaas/client"
	clientTypes "github.com/tsuru/rpaas-operator/pkg/rpaas/client/types"
	"github.com/urfave/cli/v2"
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

	args := rpaasclient.DeleteRouteArgs{
		Instance: c.String("instance"),
		Path:     c.String("path"),
	}
	_, err = client.DeleteRoute(c.Context, args)
	if err != nil {
		return err
	}

	fmt.Fprintf(c.App.Writer, "Route %q deleted.\n", args.Path)
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
	routes, _, err := client.ListRoutes(c.Context, args)
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
	for _, r := range routes {
		data = append(data, []string{r.Path, r.Destination, checkedChar(r.HTTPSOnly), r.Content})
	}

	table := tablewriter.NewWriter(w)
	table.SetHeader([]string{"Path", "Destination", "Force HTTPS?", "Configuration"})
	table.SetAutoWrapText(false)
	table.SetColumnAlignment([]int{tablewriter.ALIGN_LEFT, tablewriter.ALIGN_LEFT, tablewriter.ALIGN_CENTER, tablewriter.ALIGN_LEFT})
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

	args := rpaasclient.UpdateRouteArgs{
		Instance:    c.String("instance"),
		Path:        c.String("path"),
		Destination: c.String("destination"),
		HTTPSOnly:   c.Bool("https-only"),
		Content:     string(content),
	}
	_, err = client.UpdateRoute(c.Context, args)
	if err != nil {
		return err
	}

	fmt.Fprintf(c.App.Writer, "Route %q updated.\n", args.Path)
	return nil
}

func fetchContentFile(c *cli.Context) ([]byte, error) {
	contentFile := c.Path("content")
	if contentFile == "" {
		return nil, nil
	}
	content, err := ioutil.ReadFile(contentFile)
	if os.IsNotExist(err) &&
		strings.HasPrefix(contentFile, "@") {
		return ioutil.ReadFile(strings.TrimPrefix(contentFile, "@"))
	}

	return content, err
}
