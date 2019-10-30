// Copyright 2019 tsuru authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
package cmd

import (
	"context"
	"fmt"
	"io"
	"strconv"

	"github.com/olekukonko/tablewriter"
	"github.com/tsuru/rpaas-operator/pkg/rpaas/client"
	"github.com/tsuru/rpaas-operator/pkg/rpaas/client/types"
	"github.com/urfave/cli"
)

func info() cli.Command {
	return cli.Command{
		Name:  "info",
		Usage: "Display the available plan(s) and flavor(s) for the given instance",
		Flags: []cli.Flag{
			cli.StringFlag{
				Name:     "service, s",
				Usage:    "service name",
				Required: true,
			},
			cli.StringFlag{
				Name:     "instance, i",
				Usage:    "instance name",
				Required: true,
			},
		},

		Action: func(ctx *cli.Context) error {
			client, err := client.NewTsuruClient(ctx.GlobalString("target"), ctx.String("service"), ctx.GlobalString("token"))
			if err != nil {
				return err
			}

			instance := ctx.String("instance")

			plans, err := client.GetPlans(context.TODO(), &instance)

			writePlans("Plans", plans, ctx.App.Writer)

			flavors, err := client.GetFlavors(context.TODO(), &instance)

			writeFlavors("Flavors", flavors, ctx.App.Writer)

			if err != nil {
				return err
			}

			return nil
		},
	}
}

func writePlans(prefix string, plans []types.Plan, writer io.Writer) {
	// flushing stdout
	fmt.Fprintf(writer, "\n")

	table := tablewriter.NewWriter(writer)
	table.SetRowLine(true)
	table.SetHeader([]string{prefix, "Description", "Default"})
	for _, plan := range plans {
		table.Append([]string{plan.Name, plan.Description, strconv.FormatBool(plan.Default)})
	}

	table.Render()
}

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
