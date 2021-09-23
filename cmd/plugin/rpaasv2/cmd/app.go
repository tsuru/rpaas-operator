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
		NewCmdAccessControlList(),
		NewCmdCertificates(),
		NewCmdBlocks(),
		NewCmdRoutes(),
		NewCmdInfo(),
		NewCmdAutoscale(),
		NewCmdExec(),
		NewCmdShell(),
		NewCmdLogs(),
	}
	app.Flags = []cli.Flag{
		&cli.StringFlag{
			Name:  "rpaas-url",
			Usage: "URL to RPaaS server",
		},
		&cli.StringFlag{
			Name:  "rpaas-user",
			Usage: "user name to authenticate on RPaaS server directly",
		},
		&cli.StringFlag{
			Name:  "rpaas-password",
			Usage: "password of user to authenticate on RPaaS server directly",
		},
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
	client, ok := c.Context.Value(rpaasClientKey).(rpaasclient.Client)
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
	opts := rpaasclient.ClientOptions{Timeout: c.Duration("timeout")}
	if rpaasURL := c.String("rpaas-url"); rpaasURL != "" {
		return rpaasclient.NewClientWithOptions(rpaasURL, c.String("rpaas-user"), c.String("rpaas-password"), opts)
	}

	return rpaasclient.NewClientThroughTsuruWithOptions(c.String("tsuru-target"), c.String("tsuru-token"), c.String("tsuru-service"), opts)
}
