// Copyright 2019 tsuru authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package cmd

import (
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/olekukonko/tablewriter"
	"github.com/urfave/cli/v2"

	rpaasclient "github.com/tsuru/rpaas-operator/pkg/rpaas/client"
	clientTypes "github.com/tsuru/rpaas-operator/pkg/rpaas/client/types"
)

func NewCmdCertificates() *cli.Command {
	return &cli.Command{
		Name:    "certificates",
		Aliases: []string{"certificate"},
		Usage:   "Manages TLS certificates",
		Subcommands: []*cli.Command{
			NewCmdUpdateCertitifcate(),
			NewCmdDeleteCertitifcate(),
		},
	}
}

func NewCmdUpdateCertitifcate() *cli.Command {
	return &cli.Command{
		Name:    "update",
		Aliases: []string{"add"},
		Usage:   "Uploads a certificate and key or enables Cert Manager integration on an instance",
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
			},
			&cli.PathFlag{
				Name:    "certificate",
				Aliases: []string{"cert", "cert-file"},
				Usage:   "path in the system where the certificate (in PEM format) is located",
			},
			&cli.PathFlag{
				Name:    "key",
				Aliases: []string{"key-file"},
				Usage:   "path in the system where the key (in PEM format) is located",
			},
			&cli.BoolFlag{
				Name:  "cert-manager",
				Usage: "whether Cert Manager integration should be enabled",
			},
			&cli.StringSliceFlag{
				Name:  "dns",
				Usage: "a list of DNS names to be set on certificate as Subject Alternative Names (its usage requires --cert-manager)",
			},
			&cli.StringSliceFlag{
				Name:  "ip",
				Usage: "a list of IP addresses to be set on certificates as Subject Alternative Names (its usage requires --cert-manager)",
			},
			&cli.StringFlag{
				Name:  "issuer",
				Usage: "a Cert Manager Issuer name (its usage requires --cert-manager)",
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

	handled, err := updateCertManagerCertificate(c, client)
	if err != nil || handled {
		return err
	}

	certificate, err := os.ReadFile(c.Path("certificate"))
	if err != nil {
		return err
	}

	key, err := os.ReadFile(c.Path("key"))
	if err != nil {
		return err
	}

	name := c.String("name")
	if name == "" {
		name = "default"
	}

	args := rpaasclient.UpdateCertificateArgs{
		Instance:    c.String("instance"),
		Name:        name,
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

func updateCertManagerCertificate(c *cli.Context, client rpaasclient.Client) (bool, error) {
	if !c.Bool("cert-manager") {
		if c.String("issuer") != "" || len(c.StringSlice("dns")) > 0 || len(c.StringSlice("ip")) > 0 {
			return true, fmt.Errorf("issuer, DNS names and IP addresses require --cert-manager=true")
		}

		return false, nil
	}

	err := client.UpdateCertManager(c.Context, rpaasclient.UpdateCertManagerArgs{
		Instance: c.String("instance"),
		CertManager: clientTypes.CertManager{
			Name:        c.String("name"),
			Issuer:      c.String("issuer"),
			DNSNames:    c.StringSlice("dns"),
			IPAddresses: c.StringSlice("ip"),
		},
	})
	if err != nil {
		return true, err
	}

	fmt.Fprintln(c.App.Writer, "cert manager certificate was updated")
	return true, nil
}

func NewCmdDeleteCertitifcate() *cli.Command {
	return &cli.Command{
		Name:    "delete",
		Aliases: []string{"remove"},
		Usage:   "Deletes a certificate from a rpaas instance",
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
			&cli.BoolFlag{
				Name:  "cert-manager",
				Usage: "whether Cert Manager integration should be disabled",
			},
			&cli.StringFlag{
				Name:  "issuer",
				Usage: "a Cert Manager Issuer name (its usage requires --cert-manager)",
			},
		},
		Before: setupClient,
		Action: runDeleteCertificate,
	}
}

func runDeleteCertificate(c *cli.Context) error {
	client, err := getClient(c)
	if err != nil {
		return err
	}

	if c.Bool("cert-manager") {
		if err = client.DeleteCertManager(c.Context, c.String("instance"), c.String("issuer")); err != nil {
			return err
		}

		fmt.Fprintln(c.App.Writer, "cert manager integration was disabled")
		return nil
	}

	args := rpaasclient.DeleteCertificateArgs{
		Instance: c.String("instance"),
		Name:     c.String("name"),
	}
	err = client.DeleteCertificate(c.Context, args)
	if err != nil {
		return err
	}

	fmt.Fprintf(c.App.Writer, "certificate %q successfully deleted on %s\n", args.Name, formatInstanceName(c))
	return nil
}

func writeCertificatesInfoOnTableFormat(w io.Writer, certs []clientTypes.CertificateInfo) {
	var data [][]string
	for _, c := range certs {
		extraInfo := ""
		if c.IsManagedByCertManager {
			extraInfo = "\n  managed by: cert-manager\n  issuer: " + c.CertManagerIssuer
		}

		data = append(data, []string{c.Name + extraInfo, formatPublicKeyInfo(c), formatCertificateValidity(c), strings.Join(c.DNSNames, "\n")})
	}

	table := tablewriter.NewWriter(w)
	table.SetHeader([]string{"Name", "Public Key Info", "Validity", "DNS names"})
	table.SetColumnAlignment([]int{tablewriter.ALIGN_LEFT, tablewriter.ALIGN_CENTER, tablewriter.ALIGN_CENTER, tablewriter.ALIGN_CENTER})
	table.SetRowLine(true)
	table.SetAutoFormatHeaders(false)
	table.SetHeaderAlignment(tablewriter.ALIGN_LEFT)
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
