package cli

import (
	"github.com/tsuru/rpaas-operator/cmd/plugin/rpaasv2/types"
	"github.com/urfave/cli"
)

func AppInit() *cli.App {
	app := cli.NewApp()
	app.Flags = append(app.Flags, cli.StringFlag{
		Name:   "target",
		Hidden: true,
		EnvVar: "TSURU_TARGET",
	})

	app.Flags = append(app.Flags, cli.StringFlag{
		Name:   "token",
		Hidden: true,
		EnvVar: "TSURU_TOKEN",
	})

	return app
}

func SetBeforeFunc(app *cli.App, manager *types.Manager) {
	app.Before = func(ctx *cli.Context) error {
		if err := ctx.GlobalSet("target", manager.Target); err != nil {
			return err
		}

		if err := ctx.GlobalSet("token", manager.Token); err != nil {
			return err
		}

		ctx.App.Writer = manager.Writer

		return nil
	}
}
