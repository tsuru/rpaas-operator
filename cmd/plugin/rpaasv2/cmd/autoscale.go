// Copyright 2019 tsuru authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package cmd

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"strconv"

	"github.com/olekukonko/tablewriter"
	rpaasclient "github.com/tsuru/rpaas-operator/pkg/rpaas/client"
	clientTypes "github.com/tsuru/rpaas-operator/pkg/rpaas/client/types"
	"github.com/urfave/cli/v2"
)

func NewCmdAutoscale() *cli.Command {
	return &cli.Command{
		Name:  "autoscale",
		Usage: "Manages autoscaling settings of an instance",
		Subcommands: []*cli.Command{
			NewCmdGetAutoscale(),
			NewCmdCreateAutoscale(),
			NewCmdUpdateAutoscale(),
			NewCmdRemoveAutoscale(),
		},
	}
}

func NewCmdCreateAutoscale() *cli.Command {
	return &cli.Command{
		Name:  "add",
		Usage: "Creates autoscale spec configuration of the desired instance",
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
			&cli.IntFlag{
				Name:     "min-replicas",
				Aliases:  []string{"min"},
				Usage:    "the lower limit for the number of replicas that can be set",
				Required: false,
			},
			&cli.IntFlag{
				Name:     "max-replicas",
				Aliases:  []string{"max"},
				Usage:    "the upper limit for the number of replicas that can be set",
				Required: true,
			},
			&cli.IntFlag{
				Name:     "cpu",
				Aliases:  []string{"cpu-utilization"},
				Usage:    "the target average CPU utilization over all the units. Represented as a percentage of requested CPU",
				Required: false,
			},
			&cli.IntFlag{
				Name:     "memory",
				Aliases:  []string{"memory-utilization"},
				Usage:    "the target average memory utilization over all the units. Represented as a percentage of requested memory.",
				Required: false,
			},
		},

		Before: setupClient,
		Action: runCreateAutoscale,
	}
}

func runCreateAutoscale(c *cli.Context) error {
	client, err := getClient(c)
	if err != nil {
		return err
	}

	createArgs := rpaasclient.CreateAutoscaleArgs{
		Instance:    c.String("instance"),
		MinReplicas: int32(c.Int("min")),
		MaxReplicas: int32(c.Int("max")),
		CPU:         int32(c.Int("cpu")),
		Memory:      int32(c.Int("memory")),
	}
	_, err = client.CreateAutoscale(c.Context, createArgs)
	if err != nil {
		return err
	}

	fmt.Fprintf(c.App.Writer, "Autoscale of %s successfully created\n", formatInstanceName(c))
	return nil
}

func NewCmdUpdateAutoscale() *cli.Command {
	return &cli.Command{
		Name:  "update",
		Usage: "Updates autoscale spec configuration of the desired instance",
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
			&cli.IntFlag{
				Name:     "minReplicas",
				Aliases:  []string{"min", "min-replicas", "minimal-replicas", "minimum"},
				Usage:    "the lower limit for the number of replicas that can be set",
				Required: false,
			},
			&cli.IntFlag{
				Name:     "maxReplicas",
				Aliases:  []string{"max", "max-replicas", "maximal-replicas", "maximum"},
				Usage:    "the upper limit for the number of replicas that can be set",
				Required: false,
			},
			&cli.IntFlag{
				Name:     "cpu",
				Aliases:  []string{"cpu-utilization"},
				Usage:    "the target average CPU utilization over all the units. Represented as a percentage of requested CPU",
				Required: false,
			},
			&cli.IntFlag{
				Name:     "memory",
				Aliases:  []string{"memory-utilization"},
				Usage:    "the target average memory utilization over all the units. Represented as a percentage of requested memory.",
				Required: false,
			},
		},

		Before: setupClient,
		Action: runUpdateAutoscale,
	}
}

func runUpdateAutoscale(c *cli.Context) error {
	client, err := getClient(c)
	if err != nil {
		return err
	}

	updateArgs := rpaasclient.UpdateAutoscaleArgs{
		Instance:    c.String("instance"),
		MinReplicas: int32(c.Int("minReplicas")),
		MaxReplicas: int32(c.Int("maxReplicas")),
		CPU:         int32(c.Int("cpu")),
		Memory:      int32(c.Int("memory")),
	}
	_, err = client.UpdateAutoscale(c.Context, updateArgs)
	if err != nil {
		return err
	}

	fmt.Fprintf(c.App.Writer, "Autoscale of %s successfully updated\n", formatInstanceName(c))
	return nil
}

func NewCmdGetAutoscale() *cli.Command {
	return &cli.Command{
		Name:  "info",
		Usage: "Shows  the autoscaling settings",
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
				Name:    "raw-output",
				Aliases: []string{"r", "raw"},
				Usage:   "show as JSON instead of go template format",
				Value:   false,
			},
		},
		Before: setupClient,
		Action: runGetAutoscale,
	}
}

func runGetAutoscale(c *cli.Context) error {
	client, err := getClient(c)
	if err != nil {
		return err
	}

	args := rpaasclient.GetAutoscaleArgs{
		Instance: c.String("instance"),
		Raw:      c.Bool("raw-output"),
	}

	spec, _, err := client.GetAutoscale(c.Context, args)
	if err != nil {
		return err
	}

	if args.Raw {
		return writeAutoscaleJSON(c.App.Writer, spec)
	}

	if spec != nil {
		// writeAutoscale(c.App.Writer, spec)
		var buffer bytes.Buffer
		_, err := buffer.WriteString(writeAutoscaleOnTableFormat(spec))
		if err != nil {
			return err
		}
		buffer.WriteTo(c.App.Writer)
	}

	return nil
}

func NewCmdRemoveAutoscale() *cli.Command {
	return &cli.Command{
		Name:    "remove",
		Usage:   "Removes autoscale of the desired instance",
		Aliases: []string{"delete"},
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
		},
		Before: setupClient,
		Action: runRemoveAutoscale,
	}
}

func runRemoveAutoscale(c *cli.Context) error {
	client, err := getClient(c)
	if err != nil {
		return err
	}

	args := rpaasclient.RemoveAutoscaleArgs{
		Instance: c.String("instance"),
	}

	_, err = client.RemoveAutoscale(c.Context, args)
	if err != nil {
		return err
	}

	fmt.Fprintf(c.App.Writer, "Autoscale of %s successfully removed\n", formatInstanceName(c))

	return nil
}

func writeAutoscaleJSON(w io.Writer, spec *clientTypes.Autoscale) error {
	message, err := json.MarshalIndent(spec, "", "\t")
	if err != nil {
		return err
	}

	fmt.Fprintln(w, string(message))
	return nil
}

func writeAutoscale(w io.Writer, autoscale *clientTypes.Autoscale) {
	if autoscale == nil {
		return
	}
	table := tablewriter.NewWriter(w)
	table.SetHeader([]string{"Replicas", "Target Utilization"})
	table.SetAutoWrapText(true)
	table.SetRowLine(false)
	var max, min, cpuPercentage, memPercentage string

	if autoscale.MaxReplicas != nil {
		max = fmt.Sprintf("Max: %s", strconv.Itoa(int(*autoscale.MaxReplicas)))
	} else {
		max = "Max: N/A"
	}
	if autoscale.MinReplicas != nil {
		min = fmt.Sprintf("Min: %s", strconv.Itoa(int(*autoscale.MinReplicas)))
	} else {
		min = "Min: N/A"
	}
	if autoscale.CPU != nil {
		cpuPercentage = fmt.Sprintf("CPU: %s%%", strconv.Itoa(int(*autoscale.CPU)))
	} else {
		cpuPercentage = "CPU: N/A"
	}
	if autoscale.Memory != nil {
		memPercentage = fmt.Sprintf("Memory: %s%%", strconv.Itoa(int(*autoscale.Memory)))
	} else {
		memPercentage = "Memory: N/A"
	}
	data := [][]string{
		{max, cpuPercentage},
		{min, memPercentage},
	}
	table.AppendBulk(data)
	table.Render()
}
