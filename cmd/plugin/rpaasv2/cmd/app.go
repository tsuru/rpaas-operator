// Copyright 2019 tsuru authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package cmd

import (
	"context"
	"fmt"
	"io"
	"os"
	"time"

	rpaasclient "github.com/tsuru/rpaas-operator/pkg/rpaas/client"
	"github.com/tsuru/rpaas-operator/version"
	"github.com/urfave/cli/v2"
)

func NewDefaultApp() *cli.App {
	return NewApp(os.Stdout, os.Stderr, nil)
}

func NewApp(o, e io.Writer, client rpaasclient.Client) (app *cli.App) {
	app = cli.NewApp()
	app.Usage = "Manipulates reverse proxy instances running on Reverse Proxy as a Service."
	app.Version = version.Version
	app.ErrWriter = e
	app.Writer = o
	app.Commands = []*cli.Command{
		NewCmdScale(),
		NewCmdCertificates(),
		NewCmdBlocks(),
		NewCmdRoutes(),
		NewCmdInfo(),
	}
	app.Flags = []cli.Flag{
		&cli.StringFlag{
			Name:    "tsuru-target",
			Usage:   "address of Tsuru server",
			EnvVars: []string{"TSURU_TARGET"},
		},
		&cli.StringFlag{
			Name:        "tsuru-token",
			Usage:       "authentication credential to Tsuru server",
			EnvVars:     []string{"TSURU_TOKEN"},
			DefaultText: "-",
		},
		&cli.DurationFlag{
			Name:  "timeout",
			Usage: "time limit that a remote operation (HTTP request) can take",
			Value: 60 * time.Second,
		},
	}
	app.Before = func(c *cli.Context) error {
		setClient(c, client)
		return nil
	}
	return
}

type contextKey string

const rpaasClientKey = contextKey("rpaas.client")

var errClientNotFoundAtContext = fmt.Errorf("rpaas client not found at context")

func setClient(c *cli.Context, client rpaasclient.Client) {
	c.Context = context.WithValue(c.Context, rpaasClientKey, client)
}

func getClient(c *cli.Context) (rpaasclient.Client, error) {
	client, ok := c.Value(rpaasClientKey).(rpaasclient.Client)
	if !ok {
		return nil, errClientNotFoundAtContext
	}

	return client, nil
}

func setupClient(c *cli.Context) error {
	client, err := getClient(c)
	if err != nil && err != errClientNotFoundAtContext {
		return err
	}

	if client != nil {
		return nil
	}

	client, err = newClient(c)
	if err != nil {
		return err
	}

	setClient(c, client)
	return nil
}

func newClient(c *cli.Context) (rpaasclient.Client, error) {
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
