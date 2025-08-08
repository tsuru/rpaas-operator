// Copyright 2025 tsuru authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package cmd

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"

	"github.com/olekukonko/tablewriter"
	"github.com/urfave/cli/v2"

	rpaasclient "github.com/tsuru/rpaas-operator/pkg/rpaas/client"
	clientTypes "github.com/tsuru/rpaas-operator/pkg/rpaas/client/types"
)

func NewCmdUpstreamOptions() *cli.Command {
	return &cli.Command{
		Name:  "upstream",
		Usage: "Manages upstream options with traffic shaping and canary deployments",
		Subcommands: []*cli.Command{
			NewCmdListUpstreamOptions(),
			NewCmdAddUpstreamOptions(),
			NewCmdUpdateUpstreamOptions(),
			NewCmdDeleteUpstreamOptions(),
		},
	}
}

func NewCmdListUpstreamOptions() *cli.Command {
	return &cli.Command{
		Name:  "list",
		Usage: "Shows the upstream options on the instance",
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
				Aliases: []string{"r"},
				Usage:   "show as JSON instead of table format",
				Value:   false,
			},
		},
		Before: setupClient,
		Action: runListUpstreamOptions,
	}
}

func runListUpstreamOptions(c *cli.Context) error {
	client, err := getClient(c)
	if err != nil {
		return err
	}

	args := rpaasclient.ListUpstreamOptionsArgs{Instance: c.String("instance")}
	upstreamOptions, err := client.ListUpstreamOptions(c.Context, args)
	if err != nil {
		return err
	}

	if c.Bool("raw-output") {
		return writeUpstreamOptionsOnJSONFormat(c.App.Writer, upstreamOptions)
	}

	writeUpstreamOptionsOnTableFormat(c.App.Writer, upstreamOptions)
	return nil
}

func writeUpstreamOptionsOnTableFormat(w io.Writer, upstreamOptions []clientTypes.UpstreamOptions) {
	data := [][]string{}

	for _, uo := range upstreamOptions {
		canaryBinds := strings.Join(uo.CanaryBinds, ", ")
		if canaryBinds == "" {
			canaryBinds = "-"
		}

		loadBalance := string(uo.LoadBalance)
		if loadBalance == "" {
			loadBalance = "-"
		}

		var trafficPolicy string
		if uo.TrafficShapingPolicy.Weight > 0 {
			trafficPolicy = fmt.Sprintf("Weight: %d/%d", uo.TrafficShapingPolicy.Weight, uo.TrafficShapingPolicy.WeightTotal)
		} else if strings.TrimSpace(uo.TrafficShapingPolicy.Header) != "" {
			trafficPolicy = fmt.Sprintf("Header: %s=%s (%s)", uo.TrafficShapingPolicy.Header, uo.TrafficShapingPolicy.HeaderValue, uo.TrafficShapingPolicy.HeaderPattern)
		} else if strings.TrimSpace(uo.TrafficShapingPolicy.Cookie) != "" {
			trafficPolicy = fmt.Sprintf("Cookie: %s", uo.TrafficShapingPolicy.Cookie)
		} else {
			trafficPolicy = "-"
		}

		row := []string{uo.PrimaryBind, canaryBinds, loadBalance, trafficPolicy}
		data = append(data, row)
	}

	table := tablewriter.NewWriter(w)
	table.SetHeader([]string{"Primary Bind", "Canary Binds", "Load Balance", "Traffic Policy"})
	table.SetAutoWrapText(false)
	table.SetAutoFormatHeaders(false)
	table.SetHeaderAlignment(tablewriter.ALIGN_LEFT)
	table.SetColumnAlignment([]int{tablewriter.ALIGN_LEFT, tablewriter.ALIGN_LEFT, tablewriter.ALIGN_LEFT, tablewriter.ALIGN_LEFT})
	table.AppendBulk(data)
	table.Render()
}

func writeUpstreamOptionsOnJSONFormat(w io.Writer, upstreamOptions []clientTypes.UpstreamOptions) error {
	message, err := json.MarshalIndent(upstreamOptions, "", "\t")
	if err != nil {
		return err
	}

	fmt.Fprintln(w, string(message))
	return nil
}

func NewCmdAddUpstreamOptions() *cli.Command {
	return &cli.Command{
		Name:    "add",
		Aliases: []string{"create"},
		Usage:   "Adds upstream options for a bind",
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
				Name:     "bind",
				Aliases:  []string{"primary-bind", "b"},
				Usage:    "the primary bind name",
				Required: true,
			},
			&cli.StringSliceFlag{
				Name:    "canary",
				Aliases: []string{"canary-binds", "c"},
				Usage:   "canary bind names (can be specified multiple times)",
			},
			&cli.StringFlag{
				Name:    "load-balance",
				Aliases: []string{"lb"},
				Usage:   "load balancing algorithm (round_robin, least_conn, ip_hash, random, hash)",
			},
			&cli.IntFlag{
				Name:  "weight",
				Usage: "weight for weight-based routing (only for canary leaf nodes)",
			},
			&cli.IntFlag{
				Name:  "weight-total",
				Usage: "total weight for weight-based routing (auto-calculated if not provided)",
			},
			&cli.StringFlag{
				Name:  "header",
				Usage: "header name for header-based routing",
			},
			&cli.StringFlag{
				Name:  "header-value",
				Usage: "header value for header-based routing",
			},
			&cli.StringFlag{
				Name:  "header-pattern",
				Usage: "header pattern (exact, regex) for header-based routing",
			},
			&cli.StringFlag{
				Name:  "cookie",
				Usage: "cookie name for cookie-based routing",
			},
		},
		Before: setupClient,
		Action: runAddUpstreamOptions,
	}
}

func runAddUpstreamOptions(c *cli.Context) error {
	client, err := getClient(c)
	if err != nil {
		return err
	}

	trafficShapingPolicy := rpaasclient.TrafficShapingPolicy{
		Weight:        c.Int("weight"),
		WeightTotal:   c.Int("weight-total"),
		Header:        c.String("header"),
		HeaderValue:   c.String("header-value"),
		HeaderPattern: c.String("header-pattern"),
		Cookie:        c.String("cookie"),
	}

	args := rpaasclient.AddUpstreamOptionsArgs{
		Instance:             c.String("instance"),
		PrimaryBind:          c.String("bind"),
		CanaryBinds:          c.StringSlice("canary"),
		TrafficShapingPolicy: trafficShapingPolicy,
		LoadBalance:          c.String("load-balance"),
	}

	err = client.AddUpstreamOptions(c.Context, args)
	if err != nil {
		return err
	}

	fmt.Fprintf(c.App.Writer, "Upstream options added for bind %q.\n", args.PrimaryBind)
	return nil
}

func NewCmdUpdateUpstreamOptions() *cli.Command {
	return &cli.Command{
		Name:  "update",
		Usage: "Updates upstream options for a bind",
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
				Name:     "bind",
				Aliases:  []string{"primary-bind", "b"},
				Usage:    "the primary bind name",
				Required: true,
			},
			&cli.StringSliceFlag{
				Name:    "canary",
				Aliases: []string{"canary-binds", "c"},
				Usage:   "canary bind names (can be specified multiple times)",
			},
			&cli.StringFlag{
				Name:    "load-balance",
				Aliases: []string{"lb"},
				Usage:   "load balancing algorithm (round_robin, least_conn, ip_hash, random, hash)",
			},
			&cli.IntFlag{
				Name:  "weight",
				Usage: "weight for weight-based routing (only for canary leaf nodes)",
			},
			&cli.IntFlag{
				Name:  "weight-total",
				Usage: "total weight for weight-based routing (auto-calculated if not provided)",
			},
			&cli.StringFlag{
				Name:  "header",
				Usage: "header name for header-based routing",
			},
			&cli.StringFlag{
				Name:  "header-value",
				Usage: "header value for header-based routing",
			},
			&cli.StringFlag{
				Name:  "header-pattern",
				Usage: "header pattern (exact, regex) for header-based routing",
			},
			&cli.StringFlag{
				Name:  "cookie",
				Usage: "cookie name for cookie-based routing",
			},
		},
		Before: setupClient,
		Action: runUpdateUpstreamOptions,
	}
}

func runUpdateUpstreamOptions(c *cli.Context) error {
	client, err := getClient(c)
	if err != nil {
		return err
	}

	trafficShapingPolicy := rpaasclient.TrafficShapingPolicy{
		Weight:        c.Int("weight"),
		WeightTotal:   c.Int("weight-total"),
		Header:        c.String("header"),
		HeaderValue:   c.String("header-value"),
		HeaderPattern: c.String("header-pattern"),
		Cookie:        c.String("cookie"),
	}

	args := rpaasclient.UpdateUpstreamOptionsArgs{
		Instance:             c.String("instance"),
		PrimaryBind:          c.String("bind"),
		CanaryBinds:          c.StringSlice("canary"),
		TrafficShapingPolicy: trafficShapingPolicy,
		LoadBalance:          c.String("load-balance"),
	}

	err = client.UpdateUpstreamOptions(c.Context, args)
	if err != nil {
		return err
	}

	fmt.Fprintf(c.App.Writer, "Upstream options updated for bind %q.\n", args.PrimaryBind)
	return nil
}

func NewCmdDeleteUpstreamOptions() *cli.Command {
	return &cli.Command{
		Name:    "delete",
		Aliases: []string{"remove"},
		Usage:   "Removes upstream options for a bind",
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
				Name:     "bind",
				Aliases:  []string{"primary-bind", "b"},
				Usage:    "the primary bind name",
				Required: true,
			},
		},
		Before: setupClient,
		Action: runDeleteUpstreamOptions,
	}
}

func runDeleteUpstreamOptions(c *cli.Context) error {
	client, err := getClient(c)
	if err != nil {
		return err
	}

	args := rpaasclient.DeleteUpstreamOptionsArgs{
		Instance:    c.String("instance"),
		PrimaryBind: c.String("bind"),
	}

	err = client.DeleteUpstreamOptions(c.Context, args)
	if err != nil {
		return err
	}

	fmt.Fprintf(c.App.Writer, "Upstream options removed for bind %q.\n", args.PrimaryBind)
	return nil
}
