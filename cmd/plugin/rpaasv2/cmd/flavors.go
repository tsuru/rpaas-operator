// Copyright 2019 tsuru authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package cmd

import (
	"context"
	"fmt"
	"io"

	"github.com/olekukonko/tablewriter"
	"github.com/tsuru/rpaas-operator/pkg/rpaas/client"
	"github.com/tsuru/rpaas-operator/pkg/rpaas/client/types"
	"github.com/urfave/cli"
)

func writeFlavors(prefix string, flavors []types.Flavor, writer io.Writer) {
	// flushing stdout
	fmt.Fprintf(writer, "\n")

	table := tablewriter.NewWriter(writer)
	table.SetRowLine(true)
	table.SetHeader([]string{prefix, "Description"})
	for _, flavor := range flavors {
		table.Append([]string{flavor.Name, flavor.Description})
	}

	table.Render()
}

func flavors() cli.Command {
	return cli.Command{
		Name:        "flavors",
		Description: "Display the available flavor(s) for the given service",
		Flags: []cli.Flag{
			cli.StringFlag{
				Name:     "service, s",
				Usage:    "service name",
				Required: true,
			},
		},
		Action: func(ctx *cli.Context) error {
			client, err := client.NewTsuruClient(ctx.GlobalString("target"), ctx.String("service"), ctx.GlobalString("token"))
			if err != nil {
				return err
			}

			flavors, err := client.GetFlavors(context.TODO(), nil)

			writeFlavors("Flavors", flavors, ctx.App.Writer)

			if err != nil {
				return err
			}

			return nil
		},
	}
}
