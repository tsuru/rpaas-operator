// Copyright 2019 tsuru authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package cmd

import (
	"io"
	"os"
	"sync"
	"time"

	rpaasclient "github.com/tsuru/rpaas-operator/pkg/rpaas/client"
	"github.com/tsuru/rpaas-operator/version"
	"github.com/urfave/cli/v2"
)

type globalArgs struct {
	sync.Mutex
	rpaasClient rpaasclient.Client
}

var global = globalArgs{}

func NewDefaultApp() *cli.App {
	return NewApp(os.Stdout, os.Stderr)
}

func NewApp(o, e io.Writer) (app *cli.App) {
	app = cli.NewApp()
	app.Usage = "Manipulates reverse proxy instances running on Reverse Proxy as a Service."
	app.Version = version.Version
	app.ErrWriter = e
	app.Writer = o
	app.Commands = []*cli.Command{
		NewCmdScale(),
	}
	app.Flags = []cli.Flag{
		&cli.StringFlag{
			Name:    "tsuru-target",
			Usage:   "address of Tsuru server",
			EnvVars: []string{"TSURU_TARGET"},
		},
		&cli.StringFlag{
			Name:    "tsuru-token",
			Usage:   "authentication credential to Tsuru server",
			EnvVars: []string{"TSURU_TOKEN"},
		},
		&cli.DurationFlag{
			Name:  "timeout",
			Usage: "time limit that a remote operation (HTTP request) can take",
			Value: 10 * time.Second,
		},
	}
	return
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

	return client, nil
}

func newRpaasClient(c *cli.Context) (rpaasclient.Client, error) {
	tsuruTarget := c.String("tsuru-target")
	tsuruToken := c.String("tsuru-token")
	tsuruService := c.String("tsuru-service")

	opts := rpaasclient.ClientOptions{Timeout: c.Duration("timeout")}
	client, err := rpaasclient.NewClientThroughTsuruWithOptions(tsuruTarget, tsuruToken, tsuruService, opts)
	if err != nil {
		return nil, err
	}

	return client, nil
}
