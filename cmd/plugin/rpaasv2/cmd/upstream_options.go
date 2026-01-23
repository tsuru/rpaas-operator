// Copyright 2025 tsuru authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"strings"

	"github.com/olekukonko/tablewriter"
	"github.com/urfave/cli/v3"

	"github.com/tsuru/rpaas-operator/api/v1alpha1"
	rpaasclient "github.com/tsuru/rpaas-operator/pkg/rpaas/client"
	clientTypes "github.com/tsuru/rpaas-operator/pkg/rpaas/client/types"
)

func NewCmdUpstreamOptions() *cli.Command {
	return &cli.Command{
		Name:  "upstream",
		Usage: "Manages upstream options with traffic shaping and canary deployments",
		Commands: []*cli.Command{
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

func runListUpstreamOptions(ctx context.Context, cmd *cli.Command) error {
	client, err := getClient(ctx)
	if err != nil {
		return err
	}

	args := rpaasclient.ListUpstreamOptionsArgs{Instance: cmd.String("instance")}
	upstreamOptions, err := client.ListUpstreamOptions(ctx, args)
	if err != nil {
		return err
	}

	if cmd.Bool("raw-output") {
		return writeUpstreamOptionsOnJSONFormat(cmd.Root().Writer, upstreamOptions)
	}

	writeUpstreamOptionsOnTableFormat(cmd.Root().Writer, upstreamOptions)
	return nil
}

func formatTrafficShapingPolicies(policy v1alpha1.TrafficShapingPolicy) []string {
	var policies []string

	// Add header policy if present (highest precedence)
	if strings.TrimSpace(policy.Header) != "" {
		var headerDisplay string
		if strings.TrimSpace(policy.HeaderValue) != "" {
			// Exact match using header-value
			headerDisplay = fmt.Sprintf("Header: %s=%s (exact);", policy.Header, policy.HeaderValue)
		} else if strings.TrimSpace(policy.HeaderPattern) != "" {
			// Pattern/regex match using header-pattern
			headerDisplay = fmt.Sprintf("Header: %s=%s (regex);", policy.Header, policy.HeaderPattern)
		} else {
			// Header without value or pattern (shouldn't happen, but handle gracefully)
			headerDisplay = fmt.Sprintf("Header: %s (exact);", policy.Header)
		}
		policies = append(policies, headerDisplay)
	}

	// Add cookie policy if present (medium precedence)
	if strings.TrimSpace(policy.Cookie) != "" {
		policies = append(policies, fmt.Sprintf("Cookie: %s;", policy.Cookie))
	}

	// Add weight policy if present (lowest precedence)
	if policy.Weight > 0 {
		policies = append(policies, fmt.Sprintf("Weight: %d/%d", policy.Weight, policy.WeightTotal))
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

func formatLoadBalanceWithHashKey(loadBalance v1alpha1.LoadBalanceAlgorithm, hashKey string) string {
	algorithm := string(loadBalance)
	if algorithm == "" {
		algorithm = "round_robin"
	}

	// If it's chash and a hash key is provided, show it in a readable format
	if algorithm == "chash" && strings.TrimSpace(hashKey) != "" {
		return fmt.Sprintf("%s (key: %s)", algorithm, hashKey)
	}

	return algorithm
}

func writeUpstreamOptionsOnTableFormat(w io.Writer, upstreamOptions []clientTypes.UpstreamOptions) {
	table := tablewriter.NewWriter(w)
	table.SetHeader([]string{"Primary App", "Canary App", "Load Balance", "Traffic Policies"})
	table.SetAutoWrapText(false)
	table.SetAutoFormatHeaders(false)
	table.SetHeaderAlignment(tablewriter.ALIGN_LEFT)
	table.SetColumnAlignment([]int{tablewriter.ALIGN_LEFT, tablewriter.ALIGN_LEFT, tablewriter.ALIGN_LEFT, tablewriter.ALIGN_LEFT})
	table.SetRowSeparator("-")

	for i, uo := range upstreamOptions {
		canaryBinds := strings.Join(uo.CanaryBinds, ", ")
		if canaryBinds == "" {
			canaryBinds = "-"
		}

		// Format load balance with hash key if applicable
		loadBalance := formatLoadBalanceWithHashKey(uo.LoadBalance, uo.LoadBalanceHashKey)

		// Get formatted traffic policies (can be multiple lines)
		trafficPolicies := formatTrafficShapingPolicies(uo.TrafficShapingPolicy)

		// Add the first row with all data
		row := []string{uo.PrimaryBind, canaryBinds, loadBalance, trafficPolicies[0]}
		table.Append(row)

		// Add additional rows for extra traffic policies (with empty cells for other columns)
		for j := 1; j < len(trafficPolicies); j++ {
			additionalRow := []string{"", "", "", trafficPolicies[j]}
			table.Append(additionalRow)
		}

		// Add separator between different upstream options (except for the last one)
		if i < len(upstreamOptions)-1 {
			table.Append([]string{"", "", "", ""})
		}
	}

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
				Usage:   "canary bind name (only one canary per upstream is allowed)",
			},
			&cli.StringFlag{
				Name:    "load-balance",
				Aliases: []string{"lb"},
				Usage:   "load balancing algorithm (round_robin, chash, ewma)",
			},
			&cli.StringFlag{
				Name:  "load-balance-hash-key",
				Usage: "nginx variable, text value or combination for consistent hashing (required when load-balance is chash)",
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
				Usage: "exact header value for header-based routing (mutually exclusive with header-pattern)",
			},
			&cli.StringFlag{
				Name:  "header-pattern",
				Usage: "regex header pattern for header-based routing (mutually exclusive with header-value)",
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

func runAddUpstreamOptions(ctx context.Context, cmd *cli.Command) error {
	client, err := getClient(ctx)
	if err != nil {
		return err
	}

	// Validate that header-value and header-pattern are mutually exclusive
	headerValue := cmd.String("header-value")
	headerPattern := cmd.String("header-pattern")
	if headerValue != "" && headerPattern != "" {
		return fmt.Errorf("header-value and header-pattern are mutually exclusive, please specify only one")
	}

	trafficShapingPolicy := rpaasclient.TrafficShapingPolicy{
		Weight:        int(cmd.Int("weight")),
		WeightTotal:   int(cmd.Int("weight-total")),
		Header:        cmd.String("header"),
		HeaderValue:   headerValue,
		HeaderPattern: headerPattern,
		Cookie:        cmd.String("cookie"),
	}

	args := rpaasclient.UpstreamOptionsArgs{
		Instance:             cmd.String("instance"),
		PrimaryBind:          cmd.String("app"),
		CanaryBinds:          cmd.StringSlice("canary"),
		TrafficShapingPolicy: trafficShapingPolicy,
		LoadBalance:          cmd.String("load-balance"),
		LoadBalanceHashKey:   cmd.String("load-balance-hash-key"),
	}

	err = client.AddUpstreamOptions(ctx, args)
	if err != nil {
		return err
	}

	fmt.Fprintf(cmd.Root().Writer, "Upstream options added for app %q.\n", args.PrimaryBind)
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
				Usage:   "canary bind name (only one canary per upstream is allowed)",
			},
			&cli.StringFlag{
				Name:    "load-balance",
				Aliases: []string{"lb"},
				Usage:   "load balancing algorithm (round_robin, chash, ewma)",
			},
			&cli.StringFlag{
				Name:  "load-balance-hash-key",
				Usage: "nginx variable, text value or combination for consistent hashing (required when load-balance is chash)",
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
				Usage: "exact header value for header-based routing (mutually exclusive with header-pattern)",
			},
			&cli.StringFlag{
				Name:  "header-pattern",
				Usage: "regex header pattern for header-based routing (mutually exclusive with header-value)",
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

func runUpdateUpstreamOptions(ctx context.Context, cmd *cli.Command) error {
	client, err := getClient(ctx)
	if err != nil {
		return err
	}

	// Handle mutually exclusive header-value and header-pattern
	headerValue := cmd.String("header-value")
	headerPattern := cmd.String("header-pattern")

	// If both are provided, reject the request
	if headerValue != "" && headerPattern != "" {
		return fmt.Errorf("header-value and header-pattern are mutually exclusive, please specify only one")
	}

	// For update operations: if one is provided, the other should be cleared
	// This logic will be handled in the API layer to ensure the other field is set to empty

	trafficShapingPolicy := rpaasclient.TrafficShapingPolicy{
		Weight:        int(cmd.Int("weight")),
		WeightTotal:   int(cmd.Int("weight-total")),
		Header:        cmd.String("header"),
		HeaderValue:   headerValue,
		HeaderPattern: headerPattern,
		Cookie:        cmd.String("cookie"),
	}

	args := rpaasclient.UpstreamOptionsArgs{
		Instance:             cmd.String("instance"),
		PrimaryBind:          cmd.String("app"),
		CanaryBinds:          cmd.StringSlice("canary"),
		TrafficShapingPolicy: trafficShapingPolicy,
		LoadBalance:          cmd.String("load-balance"),
		LoadBalanceHashKey:   cmd.String("load-balance-hash-key"),
	}

	err = client.UpdateUpstreamOptions(ctx, args)
	if err != nil {
		return err
	}

	fmt.Fprintf(cmd.Root().Writer, "Upstream options updated for app %q.\n", args.PrimaryBind)
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

func runDeleteUpstreamOptions(ctx context.Context, cmd *cli.Command) error {
	client, err := getClient(ctx)
	if err != nil {
		return err
	}

	args := rpaasclient.DeleteUpstreamOptionsArgs{
		Instance:    cmd.String("instance"),
		PrimaryBind: cmd.String("app"),
	}

	err = client.DeleteUpstreamOptions(ctx, args)
	if err != nil {
		return err
	}

	fmt.Fprintf(cmd.Root().Writer, "Upstream options removed for app %q.\n", args.PrimaryBind)
	return nil
}
