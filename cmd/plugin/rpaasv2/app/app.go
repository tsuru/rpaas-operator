package app

import (
	"github.com/tsuru/rpaas-operator/cmd/plugin/rpaasv2/types"
	"github.com/urfave/cli"
)

func Init() *cli.App {
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

func TsuruApp() (*cli.App, error) {
	app := Init()
	manager, err := types.NewTsuruManager()
	if err != nil {
		return nil, err
	}
	SetContext(app, manager)

	return app, nil
}

func SetContext(app *cli.App, manager *types.Manager) {
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
