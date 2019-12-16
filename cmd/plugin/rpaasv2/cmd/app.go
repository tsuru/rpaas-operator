// Copyright 2019 tsuru authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package cmd

import (
	"sync"

	rpaasclient "github.com/tsuru/rpaas-operator/pkg/rpaas/client"
	"github.com/urfave/cli"
)

type globalArgs struct {
	sync.Mutex
	rpaasClient rpaasclient.Client
}

var global = globalArgs{}

func appendCmds(cliApp *cli.App) {
	cliApp.Commands = []cli.Command{
		Scale(),
		info(),
	}
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

func setRpaasClient(client rpaasclient.Client) {
	global.Lock()
	defer global.Unlock()

	global.rpaasClient = client
}

func getRpaasClient(c *cli.Context) (rpaasclient.Client, error) {
	global.Lock()
	defer global.Unlock()

	if global.rpaasClient != nil {
		return global.rpaasClient, nil
	}

	client, err := newRpaasClient(c)
	if err != nil {
		return nil, err
	}

	return client, err
}

func newRpaasClient(c *cli.Context) (rpaasclient.Client, error) {
	tsuruTarget := c.GlobalString("target")
	tsuruToken := c.GlobalString("token")
	tsuruService := c.String("service")

	client, err := rpaasclient.NewClientThroughTsuru(tsuruTarget, tsuruToken, tsuruService)
	if err != nil {
		return nil, err
	}

	return client, nil
}
