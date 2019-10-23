package main

import (
	"log"
	"os"

	cliApp "github.com/tsuru/rpaas-operator/cmd/plugin/rpaasv2/cli"
	"github.com/tsuru/rpaas-operator/cmd/plugin/rpaasv2/types"
	"github.com/urfave/cli"
)

func appendCmds(app *cli.App) {
	app.Commands = append(app.Commands, cliApp.CreateScale())
}

func createTsuruCLi() (*cli.App, error) {
	app := cliApp.AppInit()
	manager, err := types.NewTsuruManager()
	if err != nil {
		return nil, err
	}
	cliApp.SetBeforeFunc(app, manager)

	return app, nil
}

func main() {
	app, err := createTsuruCLi()
	appendCmds(app)

	if err != nil {
		log.Fatal(err)
	}
	err = app.Run(os.Args)
	if err != nil {
		log.Fatal(err)
	}
}
