// Copyright 2019 tsuru authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package cmd

import (
	"fmt"
	"io"
	"io/ioutil"
	"strings"

	"github.com/olekukonko/tablewriter"
	rpaasclient "github.com/tsuru/rpaas-operator/pkg/rpaas/client"
	clientTypes "github.com/tsuru/rpaas-operator/pkg/rpaas/client/types"
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
		Before: setupClient,
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
	err = client.UpdateCertificate(c.Context, args)
	if err != nil {
		return err
	}

	fmt.Fprintf(c.App.Writer, "certificate %q updated in %s\n", args.Name, formatInstanceName(c))
	return nil
}

func writeCertificatesInfoOnTableFormat(w io.Writer, certs []clientTypes.CertificateInfo) {
	var data [][]string
	for _, c := range certs {
		data = append(data, []string{c.Name, formatPublicKeyInfo(c), formatCertificateValidity(c), strings.Join(c.DNSNames, "\n")})
	}

	table := tablewriter.NewWriter(w)
	table.SetHeader([]string{"Name", "Public Key Info", "Validity", "DNS names"})
	table.SetColumnAlignment([]int{tablewriter.ALIGN_LEFT, tablewriter.ALIGN_CENTER, tablewriter.ALIGN_CENTER, tablewriter.ALIGN_CENTER})
	table.SetRowLine(true)
	table.SetAutoWrapText(false)
	table.AppendBulk(data)
	table.Render()
}

func formatPublicKeyInfo(c clientTypes.CertificateInfo) (pkInfo string) {
	if c.PublicKeyAlgorithm != "" {
		pkInfo += fmt.Sprintf("Algorithm\n%s\n\n", c.PublicKeyAlgorithm)
	}

	if c.PublicKeyBitSize > 0 {
		pkInfo += fmt.Sprintf("Key size (in bits)\n%d", c.PublicKeyBitSize)
	}

	return
}

func formatCertificateValidity(c clientTypes.CertificateInfo) string {
	return fmt.Sprintf("Not before\n%s\n\nNot after\n%s", formatTime(c.ValidFrom), formatTime(c.ValidUntil))
}
