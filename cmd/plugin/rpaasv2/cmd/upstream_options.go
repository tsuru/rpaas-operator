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

	"github.com/tsuru/rpaas-operator/api/v1alpha1"
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

func formatTrafficShapingPolicies(policy v1alpha1.TrafficShapingPolicy) []string {
	var policies []string

	// Add weight policy if present
	if policy.Weight > 0 {
		policies = append(policies, fmt.Sprintf("Weight: %d/%d;", policy.Weight, policy.WeightTotal))
	}

	// Add header policy if present
	if strings.TrimSpace(policy.Header) != "" {
		pattern := policy.HeaderPattern
		if pattern == "" {
			pattern = "exact"
		}
		policies = append(policies, fmt.Sprintf("Header: %s=%s (%s);", policy.Header, policy.HeaderValue, pattern))
	}

	// Add cookie policy if present
	if strings.TrimSpace(policy.Cookie) != "" {
		policies = append(policies, fmt.Sprintf("Cookie: %s", policy.Cookie))
	}

	// Return formatted policies or "None" if no policies
	if len(policies) == 0 {
		return []string{"None"}
	}

	// Remove trailing semicolon from last policy
	if len(policies) > 0 && strings.HasSuffix(policies[len(policies)-1], ";") {
		lastPolicy := policies[len(policies)-1]
		policies[len(policies)-1] = strings.TrimSuffix(lastPolicy, ";")
	}

	return policies
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

		// Get formatted traffic policies (can be multiple lines)
		trafficPolicies := formatTrafficShapingPolicies(uo.TrafficShapingPolicy)

		// Add the first row with all data
		row := []string{uo.PrimaryBind, canaryBinds, loadBalance, trafficPolicies[0]}
		data = append(data, row)

		// Add additional rows for extra traffic policies (with empty cells for other columns)
		for i := 1; i < len(trafficPolicies); i++ {
			additionalRow := []string{"", "", "", trafficPolicies[i]}
			data = append(data, additionalRow)
		}
	}

	table := tablewriter.NewWriter(w)
	table.SetHeader([]string{"Primary App", "Canary Apps", "Load Balance", "Traffic Policies"})
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
		Usage:   "Adds upstream options for an app",
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
				Name:     "app",
				Aliases:  []string{"bind", "primary-bind", "a"},
				Usage:    "the primary app name",
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
		PrimaryBind:          c.String("app"),
		CanaryBinds:          c.StringSlice("canary"),
		TrafficShapingPolicy: trafficShapingPolicy,
		LoadBalance:          c.String("load-balance"),
	}

	err = client.AddUpstreamOptions(c.Context, args)
	if err != nil {
		return err
	}

	fmt.Fprintf(c.App.Writer, "Upstream options added for app %q.\n", args.PrimaryBind)
	return nil
}

func NewCmdUpdateUpstreamOptions() *cli.Command {
	return &cli.Command{
		Name:  "update",
		Usage: "Updates upstream options for an app",
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
				Name:     "app",
				Aliases:  []string{"bind", "primary-bind", "a"},
				Usage:    "the primary app name",
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
		PrimaryBind:          c.String("app"),
		CanaryBinds:          c.StringSlice("canary"),
		TrafficShapingPolicy: trafficShapingPolicy,
		LoadBalance:          c.String("load-balance"),
	}

	err = client.UpdateUpstreamOptions(c.Context, args)
	if err != nil {
		return err
	}

	fmt.Fprintf(c.App.Writer, "Upstream options updated for app %q.\n", args.PrimaryBind)
	return nil
}

func NewCmdDeleteUpstreamOptions() *cli.Command {
	return &cli.Command{
		Name:    "delete",
		Aliases: []string{"remove"},
		Usage:   "Removes upstream options for an app",
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
				Name:     "app",
				Aliases:  []string{"bind", "primary-bind", "a"},
				Usage:    "the primary app name",
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
		PrimaryBind: c.String("app"),
	}

	err = client.DeleteUpstreamOptions(c.Context, args)
	if err != nil {
		return err
	}

	fmt.Fprintf(c.App.Writer, "Upstream options removed for app %q.\n", args.PrimaryBind)
	return nil
}
