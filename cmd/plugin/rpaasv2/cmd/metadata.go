// Copyright 2024 tsuru authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package cmd

import (
	"fmt"
	"io"
	"strings"

	"github.com/urfave/cli/v2"

	"github.com/tsuru/rpaas-operator/pkg/rpaas/client/types"
)

func NewCmdMetadata() *cli.Command {
	return &cli.Command{
		Name:  "metadata",
		Usage: "Manages metadata information of rpaasv2 instances",
		Subcommands: []*cli.Command{
			NewCmdGetMetadata(),
			NewCmdSetMetadata(),
			NewCmdUnsetMetadata(),
		},
	}
}

func NewCmdGetMetadata() *cli.Command {
	return &cli.Command{
		Name:  "get",
		Usage: "Shows metadata information of an instance",
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
			&cli.BoolFlag{
				Name:  "json",
				Usage: "show as JSON instead of go template format",
				Value: false,
			},
		},
		Before: setupClient,
		Action: runGetMetadata,
	}
}

func writeMetadata(w io.Writer, metadata *types.Metadata) {
	if len(metadata.Labels) > 0 {
		fmt.Fprintf(w, "Labels:\n")
		for _, v := range metadata.Labels {
			fmt.Fprintf(w, "  %s: %s\n", v.Name, v.Value)
		}
	}

	if len(metadata.Annotations) > 0 {
		fmt.Fprintf(w, "Annotations:\n")
		for _, v := range metadata.Annotations {
			fmt.Fprintf(w, "  %s: %s\n", v.Name, v.Value)
		}
	}
}

func runGetMetadata(c *cli.Context) error {
	client, err := getClient(c)
	if err != nil {
		return err
	}

	metadata, err := client.GetMetadata(c.Context, c.String("instance"))
	if err != nil {
		return err
	}

	if outputAsJSON := c.Bool("json"); outputAsJSON {
		return writeJSON(c.App.Writer, metadata)
	}

	writeMetadata(c.App.Writer, metadata)
	return nil
}

func NewCmdSetMetadata() *cli.Command {
	return &cli.Command{
		Name:      "set",
		Usage:     "Sets metadata information of an instance",
		ArgsUsage: "<NAME=value> [NAME=value] ...",
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
				Name:     "type",
				Aliases:  []string{"t"},
				Usage:    "the type of metadata (label or annotation)",
				Required: true,
			},
		},
		Before: setupClient,
		Action: runSetMetadata,
	}
}

func isValidMetadataType(metaType string) bool {
	return metaType == "label" || metaType == "annotation"
}

func createMetadata(meta []string, metaType string, isSet bool) (*types.Metadata, error) {
	metadata := &types.Metadata{}

	for _, kv := range meta {
		var item types.MetadataItem
		if isSet {
			if !strings.Contains(kv, "=") {
				return nil, fmt.Errorf("invalid NAME=value pair: %q", kv)
			}
			item.Name = strings.Split(kv, "=")[0]
			item.Value = strings.Split(kv, "=")[1]
		} else {
			item.Name = kv
			item.Delete = true
		}

		if metaType == "label" {
			metadata.Labels = append(metadata.Labels, item)
		} else {
			metadata.Annotations = append(metadata.Annotations, item)
		}
	}

	return metadata, nil
}

func runSetMetadata(c *cli.Context) error {
	keyValues := c.Args().Slice()
	metaType := c.String("type")

	if len(keyValues) == 0 {
		return fmt.Errorf("at least one NAME=value pair is required")
	}

	if !isValidMetadataType(metaType) {
		return fmt.Errorf("invalid metadata type: %q", metaType)
	}

	metadata, err := createMetadata(keyValues, metaType, true)
	if err != nil {
		return err
	}

	client, err := getClient(c)
	if err != nil {
		return err
	}

	err = client.SetMetadata(c.Context, c.String("instance"), metadata)
	if err != nil {
		return err
	}

	fmt.Fprintln(c.App.Writer, metadata)
	fmt.Fprintln(c.App.Writer, "Metadata updated successfully")

	return nil
}

func NewCmdUnsetMetadata() *cli.Command {
	return &cli.Command{
		Name:      "unset",
		Usage:     "Unsets metadata information of an instance",
		ArgsUsage: "NAME [NAME] ...",
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
				Name:     "type",
				Aliases:  []string{"t"},
				Usage:    "the type of metadata (label or annotation)",
				Required: true,
			},
		},
		Before: setupClient,
		Action: runUnsetMetadata,
	}
}

func runUnsetMetadata(c *cli.Context) error {
	keys := c.Args().Slice()
	metaType := c.String("type")

	if len(keys) == 0 {
		return fmt.Errorf("at least one NAME is required")
	}

	if !isValidMetadataType(metaType) {
		return fmt.Errorf("invalid metadata type: %q", metaType)
	}

	metadata, err := createMetadata(keys, metaType, false)
	if err != nil {
		return err
	}

	client, err := getClient(c)
	if err != nil {
		return err
	}

	err = client.UnsetMetadata(c.Context, c.String("instance"), metadata)
	if err != nil {
		return err
	}

	fmt.Fprintln(c.App.Writer, metadata)
	fmt.Fprintln(c.App.Writer, "Metadata removed successfully")

	return nil
}
