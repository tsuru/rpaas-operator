// Copyright 2019 tsuru authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

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
