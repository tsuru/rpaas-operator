// Copyright 2019 tsuru authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package cmd

import (
	"context"
	"fmt"
	"io/ioutil"

	rpaasclient "github.com/tsuru/rpaas-operator/pkg/rpaas/client"
	"github.com/urfave/cli/v2"
)

func NewCmdBlocks() *cli.Command {
	return &cli.Command{
		Name:  "blocks",
		Usage: "Manages fragments of NGINX configuration",
		Subcommands: []*cli.Command{
			NewCmdUpdateBlock(),
		},
	}
}

func NewCmdUpdateBlock() *cli.Command {
	return &cli.Command{
		Name:    "update",
		Aliases: []string{"add"},
		Usage:   "Inserts a NGINX configuration into instance's configuration",
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:    "service",
				Aliases: []string{"tsuru-service", "s"},
				Usage:   "the Tsuru service name",
			},
			&cli.StringFlag{
				Name:     "instance",
				Aliases:  []string{"tsuru-service-instance", "i"},
				Usage:    "the reverse proxy instance name",
				Required: true,
			},
			&cli.StringFlag{
				Name:     "name",
				Aliases:  []string{"context", "n"},
				Usage:    "the NGINX context name where this fragment will be inserted (supported values: root, http, server, lua-server, lua-worker)",
				Required: true,
			},
			&cli.PathFlag{
				Name:     "content",
				Aliases:  []string{"content-file", "c"},
				Usage:    "path in the system where the NGINX configuration fragment is located",
				Required: true,
			},
		},
		Action: runUpdateBlock,
	}
}

func runUpdateBlock(c *cli.Context) error {
	client, err := getRpaasClient(c)
	if err != nil {
		return err
	}

	content, err := ioutil.ReadFile(c.Path("content"))
	if err != nil {
		return err
	}

	args := rpaasclient.UpdateBlockArgs{
		Instance: c.String("instance"),
		Name:     c.String("name"),
		Content:  string(content),
	}
	_, err = client.UpdateBlock(context.Background(), args)
	if err != nil {
		return err
	}

	fmt.Fprintf(c.App.Writer, "NGINX configuration fragment inserted at %q context\n", args.Name)
	return nil
}
