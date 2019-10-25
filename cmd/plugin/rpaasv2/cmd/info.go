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

func initInfoFlags() []cli.Flag {
	return []cli.Flag{
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
	}
}

type respData struct {
	Name        string `json:"name"`
	Description string `json:"description"`
}

func Info() cli.Command {
	info := cli.Command{
		Name:  "info",
		Usage: "Displays tables of plans and flavors from the respective instance passed",
		Flags: initInfoFlags(),

		Action: func(ctx *cli.Context) error {
			client, err := client.NewTsuruClient(ctx.GlobalString("target"), ctx.String("service"), ctx.GlobalString("token"))
			if err != nil {
				return err
			}

			instance := ctx.String("instance")

			plans, err := client.GetPlans(context.TODO(), &instance)

			WritePlans("Plans", plans, ctx.App.Writer)

			flavors, err := client.GetFlavors(context.TODO(), &instance)

			WriteFlavors("Flavors", flavors, ctx.App.Writer)

			if err != nil {
				return err
			}

			return nil
		},
	}

	return info
}

func WritePlans(prefix string, plans []types.Plan, writer io.Writer) {
	// flushing stdout
	fmt.Println()

	table := tablewriter.NewWriter(writer)
	table.SetRowLine(true)
	table.SetHeader([]string{prefix, "Description"})
	for _, plan := range plans {
		table.Append([]string{plan.Name, plan.Description, "Default: " + strconv.FormatBool(plan.Default)})
	}

	table.Render()
}

func WriteFlavors(prefix string, flavors []types.Flavor, writer io.Writer) {
	// flushing stdout
	fmt.Println()

	table := tablewriter.NewWriter(writer)
	table.SetRowLine(true)
	table.SetHeader([]string{prefix, "Description"})
	for _, flavor := range flavors {
		table.Append([]string{flavor.Name, flavor.Description})
	}

	table.Render()
}
