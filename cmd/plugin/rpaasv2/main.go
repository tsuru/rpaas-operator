package main

import (
	"log"
	"os"

	"github.com/tsuru/rpaas-operator/cmd/plugin/rpaasv2/app"
	"github.com/tsuru/rpaas-operator/cmd/plugin/rpaasv2/cmd"
	"github.com/urfave/cli"
)

func appendCmds(cliApp *cli.App) {
	cliApp.Commands = append(cliApp.Commands, cmd.Scale())
}

func main() {
	app, err := app.TsuruApp()
	if err != nil {
		log.Fatal(err)
	}

	appendCmds(app)

	err = app.Run(os.Args)
	if err != nil {
		log.Fatal(err)
	}
}
