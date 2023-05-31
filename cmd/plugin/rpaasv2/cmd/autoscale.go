// Copyright 2019 tsuru authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package cmd

import (
	"encoding/json"
	"fmt"
	"io"

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
			NewCmdUpdateAutoscale(),
			NewCmdRemoveAutoscale(),
		},
	}
}

func NewCmdUpdateAutoscale() *cli.Command {
	return &cli.Command{
		Name:    "add",
		Aliases: []string{"update"},
		Usage:   "Creates autoscale spec configuration of the desired instance",
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
				Name:     "min",
				Usage:    "the lower limit of replicas that can be reached",
				Required: false,
			},
			&cli.IntFlag{
				Name:     "max",
				Usage:    "the upper limit of replicas that can be reached",
				Required: true,
			},
			&cli.IntFlag{
				Name:        "cpu",
				Aliases:     []string{"cpu-utilization"},
				Usage:       "the target average CPU utilization on all replicas (in percentage format, e.g. 80 equals to 80%)",
				DefaultText: "N/A",
			},
			&cli.IntFlag{
				Name:        "memory",
				Aliases:     []string{"memory-utilization"},
				Usage:       "the target average memory utilization on all the replicas (in percentage format, e.g. 80 equals to 80%)",
				DefaultText: "N/A",
			},
			&cli.IntFlag{
				Name:        "rps",
				Aliases:     []string{"requests-per-second"},
				Usage:       "the target average of HTTP requests per seconds between replicas (e.g. 100, means 100 req/s)",
				DefaultText: "N/A",
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
		MaxReplicas: pointerToInt32(int32(c.Int("max"))),
	}

	if c.IsSet("min") {
		updateArgs.MinReplicas = pointerToInt32(int32(c.Int("min")))
	} else {
		updateArgs.MinReplicas = pointerToInt32(1)
	}

	if c.IsSet("cpu") {
		updateArgs.CPU = pointerToInt32(int32(c.Int("cpu")))
	}

	if c.IsSet("memory") {
		updateArgs.Memory = pointerToInt32(int32(c.Int("memory")))
	}

	if c.IsSet("rps") {
		updateArgs.RPS = pointerToInt32(int32(c.Int("rps")))
	}

	err = client.UpdateAutoscale(c.Context, updateArgs)
	if err != nil {
		return err
	}

	fmt.Fprintf(c.App.Writer, "Autoscale of %s successfully updated!\n", formatInstanceName(c))
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

	spec, err := client.GetAutoscale(c.Context, args)
	if err != nil {
		return err
	}

	if args.Raw {
		return writeAutoscaleJSON(c.App.Writer, spec)
	}

	if spec != nil {
		writeAutoscale(c.App.Writer, spec)
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

	err = client.RemoveAutoscale(c.Context, args)
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
	table.SetHeader([]string{"Replicas", "Target"})
	table.SetAutoFormatHeaders(false)
	table.SetHeaderAlignment(tablewriter.ALIGN_LEFT)
	table.SetAutoWrapText(true)
	table.SetRowLine(false)

	max := "Max: N/A"
	if autoscale.MaxReplicas != nil {
		max = fmt.Sprintf("Max: %d", *autoscale.MaxReplicas)
	}

	min := "Min: N/A"
	if autoscale.MinReplicas != nil {
		min = fmt.Sprintf("Min: %d", *autoscale.MinReplicas)
	}

	cpuPercentage := "CPU: N/A"
	if autoscale.CPU != nil {
		cpuPercentage = fmt.Sprintf("CPU: %d%%", *autoscale.CPU)
	}

	memPercentage := "Memory: N/A"
	if autoscale.Memory != nil {
		memPercentage = fmt.Sprintf("Memory: %d%%", *autoscale.Memory)
	}

	rps := "RPS: N/A"
	if autoscale.RPS != nil {
		rps = fmt.Sprintf("RPS: %d req/s", *autoscale.RPS)
	}

	data := [][]string{
		{max, cpuPercentage},
		{min, memPercentage},
		{"", rps},
	}
	table.AppendBulk(data)
	table.Render()
}

func pointerToInt32(x int32) *int32 {
	return &x
}
