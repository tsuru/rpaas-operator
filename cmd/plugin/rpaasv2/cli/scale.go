package cli

import (
	"context"
	"fmt"

	"github.com/tsuru/rpaas-operator/pkg/rpaas/client"
	"github.com/urfave/cli"
)

func createFlags() []cli.Flag {
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
		cli.IntFlag{
			Name:     "quantity, q",
			Usage:    "amount of units to scale to",
			Required: true,
		},
	}
}

func CreateScale() cli.Command {
	scale := cli.Command{
		Name:  "scale",
		Usage: "Scales the specified rpaas instance to [-q] units",
		Flags: createFlags(),

		Action: func(ctx *cli.Context) error {
			tsuruTarget := ctx.GlobalString("target")
			tsuruToken := ctx.GlobalString("token")
			serviceName := ctx.String("service")
			instanceName := ctx.String("instance")
			quantity := ctx.Int("quantity")
			client, err := client.NewTsuruClient(tsuruTarget, serviceName, tsuruToken)
			if err != nil {
				return err
			}
			err = client.Scale(context.TODO(), instanceName, int32(quantity))
			if err != nil {
				return err
			}

			fmt.Fprintf(ctx.App.Writer, "Instance successfully scaled to %d unit(s)\n", quantity)
			return nil
		},
	}

	return scale
}
