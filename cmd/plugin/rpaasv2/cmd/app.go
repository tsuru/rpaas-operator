package cmd

import (
	"github.com/urfave/cli"
)

func appendCmds(cliApp *cli.App) {
	cliApp.Commands = append(cliApp.Commands, Scale())
}

func NewApp() *cli.App {
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

	appendCmds(app)

	return app
}
