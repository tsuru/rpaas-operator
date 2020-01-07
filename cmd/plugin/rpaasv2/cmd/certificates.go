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

func NewCmdCertificates() *cli.Command {
	return &cli.Command{
		Name:  "certificates",
		Usage: "Manages TLS certificates",
		Subcommands: []*cli.Command{
			NewCmdUpdateCertitifcate(),
		},
	}
}

func NewCmdUpdateCertitifcate() *cli.Command {
	return &cli.Command{
		Name:    "update",
		Aliases: []string{"add"},
		Usage:   "Uploads a certificate and key to an instance.",
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
				Name:  "name",
				Usage: "an identifier for the current certificate and key",
				Value: "default",
			},
			&cli.PathFlag{
				Name:     "certificate",
				Aliases:  []string{"cert", "cert-file"},
				Usage:    "path in the system where the certificate (in PEM format) is located",
				Required: true,
			},
			&cli.PathFlag{
				Name:     "key",
				Aliases:  []string{"key-file"},
				Usage:    "path in the system where the key (in PEM format) is located",
				Required: true,
			},
		},
		Action: runUpdateCertificate,
	}
}

func runUpdateCertificate(c *cli.Context) error {
	client, err := getClient(c)
	if err != nil {
		return err
	}

	certificate, err := ioutil.ReadFile(c.Path("certificate"))
	if err != nil {
		return err
	}

	key, err := ioutil.ReadFile(c.Path("key"))
	if err != nil {
		return err
	}

	args := rpaasclient.UpdateCertificateArgs{
		Instance:    c.String("instance"),
		Name:        c.String("name"),
		Certificate: string(certificate),
		Key:         string(key),
	}
	_, err = client.UpdateCertificate(context.Background(), args)
	if err != nil {
		return err
	}

	fmt.Fprintf(c.App.Writer, "certificate %q updated in %s\n", args.Name, formatInstanceName(c))
	return nil
}
