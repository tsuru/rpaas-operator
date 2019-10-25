// Copyright 2019 tsuru authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package cmd

import (
	"context"
	"fmt"

	"github.com/tsuru/rpaas-operator/pkg/rpaas/client"
	"github.com/urfave/cli"
)

func initScaleFlags() []cli.Flag {
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

func Scale() cli.Command {
	scale := cli.Command{
		Name:  "scale",
		Usage: "Scales the specified rpaas instance to [-q] units",
		Flags: initScaleFlags(),

		Action: func(ctx *cli.Context) error {
			quantity := int32(ctx.Int("quantity"))
			client, err := client.NewTsuruClient(ctx.GlobalString("target"), ctx.String("service"), ctx.GlobalString("token"))
			if err != nil {
				return err
			}

			err = client.Scale(context.TODO(), ctx.String("instance"), quantity)

			if err != nil {
				return err
			}

			fmt.Fprintf(ctx.App.Writer, "Instance successfully scaled to %d unit(s)\n", quantity)
			return nil
		},
	}

	return scale
}
