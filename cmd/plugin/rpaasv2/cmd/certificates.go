// Copyright 2019 tsuru authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package cmd

import (
	"context"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/olekukonko/tablewriter"
	"github.com/olekukonko/tablewriter/tw"
	"github.com/urfave/cli/v3"

	rpaasclient "github.com/tsuru/rpaas-operator/pkg/rpaas/client"
	clientTypes "github.com/tsuru/rpaas-operator/pkg/rpaas/client/types"
)

func NewCmdCertificates() *cli.Command {
	return &cli.Command{
		Name:    "certificates",
		Aliases: []string{"certificate"},
		Usage:   "Manages TLS certificates",
		Commands: []*cli.Command{
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
			&cli.StringFlag{
				Name:      "certificate",
				Aliases:   []string{"cert", "cert-file"},
				Usage:     "path in the system where the certificate (in PEM format) is located",
				TakesFile: true,
			},
			&cli.StringFlag{
				Name:      "key",
				Aliases:   []string{"key-file"},
				Usage:     "path in the system where the key (in PEM format) is located",
				TakesFile: true,
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

func runUpdateCertificate(ctx context.Context, cmd *cli.Command) error {
	client, err := getClient(ctx)
	if err != nil {
		return err
	}

	handled, err := updateCertManagerCertificate(ctx, cmd, client)
	if err != nil || handled {
		return err
	}

	certificate, err := os.ReadFile(cmd.String("certificate"))
	if err != nil {
		return err
	}

	key, err := os.ReadFile(cmd.String("key"))
	if err != nil {
		return err
	}

	name := cmd.String("name")
	if name == "" {
		name = "default"
	}

	args := rpaasclient.UpdateCertificateArgs{
		Instance:    cmd.String("instance"),
		Name:        name,
		Certificate: string(certificate),
		Key:         string(key),
	}
	err = client.UpdateCertificate(ctx, args)
	if err != nil {
		return err
	}

	fmt.Fprintf(cmd.Root().Writer, "certificate %q updated in %s\n", args.Name, formatInstanceName(cmd))
	return nil
}

func updateCertManagerCertificate(ctx context.Context, cmd *cli.Command, client rpaasclient.Client) (bool, error) {
	if !cmd.Bool("cert-manager") {
		if cmd.String("issuer") != "" || len(cmd.StringSlice("dns")) > 0 || len(cmd.StringSlice("ip")) > 0 {
			return true, fmt.Errorf("issuer, DNS names and IP addresses require --cert-manager=true")
		}

		return false, nil
	}

	err := client.UpdateCertManager(ctx, rpaasclient.UpdateCertManagerArgs{
		Instance: cmd.String("instance"),
		CertManager: clientTypes.CertManager{
			Name:        cmd.String("name"),
			Issuer:      cmd.String("issuer"),
			DNSNames:    cmd.StringSlice("dns"),
			IPAddresses: cmd.StringSlice("ip"),
		},
	})
	if err != nil {
		return true, err
	}

	fmt.Fprintln(cmd.Root().Writer, "cert manager certificate was updated")
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

func runDeleteCertificate(ctx context.Context, cmd *cli.Command) error {
	client, err := getClient(ctx)
	if err != nil {
		return err
	}

	if cmd.Bool("cert-manager") {
		if cmd.String("name") != "" {
			err = client.DeleteCertManagerByName(ctx, cmd.String("instance"), cmd.String("name"))
		} else {
			err = client.DeleteCertManagerByIssuer(ctx, cmd.String("instance"), cmd.String("issuer"))
		}

		if err != nil {
			return err
		}

		fmt.Fprintln(cmd.Root().Writer, "cert manager integration was disabled")
		return nil
	}

	args := rpaasclient.DeleteCertificateArgs{
		Instance: cmd.String("instance"),
		Name:     cmd.String("name"),
	}
	err = client.DeleteCertificate(ctx, args)
	if err != nil {
		return err
	}

	fmt.Fprintf(cmd.Root().Writer, "certificate %q successfully deleted on %s\n", args.Name, formatInstanceName(cmd))
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

	table := newTable(w,
		tablewriter.WithRowAlignmentConfig(tw.CellAlignment{
			PerColumn: []tw.Align{tw.AlignLeft, tw.AlignCenter, tw.AlignCenter, tw.AlignCenter},
		}),
		tablewriter.WithRendition(tw.Rendition{
			Settings: tw.Settings{Separators: tw.Separators{BetweenRows: tw.On}},
		}),
	)
	table.Header("Name", "Public Key Info", "Validity", "DNS names")
	table.Bulk(data)
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
