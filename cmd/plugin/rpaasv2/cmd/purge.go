// Copyright 2025 tsuru authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/urfave/cli/v3"

	rpaasclient "github.com/tsuru/rpaas-operator/pkg/rpaas/client"
)

func NewCmdPurge() *cli.Command {
	return &cli.Command{
		Name:  "purge",
		Usage: "Purges objects from rpaas cache",
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
				Name:    "path",
				Aliases: []string{"p"},
				Usage:   "path to be purged (ignored if --file is provided)",
			},
			&cli.BoolFlag{
				Name:    "preserve-path",
				Aliases: []string{"preserve"},
				Usage:   "whether should preserve the given path (only for single path purge)",
			},
			&cli.StringSliceFlag{
				Name:    "header",
				Aliases: []string{"H"},
				Usage:   "extra headers to be sent in the purge request (format: \"Key: Value\", only for single path purge)",
			},
			&cli.StringFlag{
				Name:      "file",
				Aliases:   []string{"f"},
				Usage:     "path to JSON file containing purge items (enables bulk mode)",
				TakesFile: true,
			},
		},
		Before: setupClient,
		Action: runPurge,
		Description: `Purges objects from rpaas cache.

Single path purge:
  rpaasv2 purge -s my-service -i my-instance -p /path/to/purge
  rpaasv2 purge -s my-service -i my-instance -p /path/to/purge -H "Accept: text/html" -H "Accept: application/json"

Bulk purge from JSON file:
  rpaasv2 purge -s my-service -i my-instance -f purge.json

The JSON file should contain an array of objects with the following structure:
[
  {
    "path": "/path/to/purge",
    "preserve_path": false,
    "extra_headers": {
      "Accept": ["text/html", "application/json"],
      "X-Custom-Header": ["value"]
    }
  }
]

Note: extra_headers values must be arrays, even for single values.`,
	}
}

func runPurge(ctx context.Context, cmd *cli.Command) error {
	filePath := cmd.String("file")

	if filePath != "" {
		return runPurgeBulk(ctx, cmd, filePath)
	}

	return runPurgePath(ctx, cmd)
}

func runPurgePath(ctx context.Context, cmd *cli.Command) error {
	path := cmd.String("path")
	if path == "" {
		return fmt.Errorf("either --path or --file must be provided")
	}

	client, err := getClient(ctx)
	if err != nil {
		return err
	}

	var extraHeaders map[string][]string
	if headers := cmd.StringSlice("header"); len(headers) > 0 {
		extraHeaders = make(map[string][]string)
		for _, header := range headers {
			parts := strings.SplitN(header, ":", 2)
			if len(parts) != 2 {
				return fmt.Errorf("invalid header format: %q (expected \"Key: Value\")", header)
			}
			key := strings.TrimSpace(parts[0])
			value := strings.TrimSpace(parts[1])
			extraHeaders[key] = append(extraHeaders[key], value)
		}
	}

	args := rpaasclient.PurgeCacheArgs{
		Instance:     cmd.String("instance"),
		Path:         path,
		PreservePath: cmd.Bool("preserve-path"),
		ExtraHeaders: extraHeaders,
	}

	count, err := client.PurgeCache(ctx, args)
	if err != nil {
		return err
	}

	fmt.Fprintf(cmd.Root().Writer, "Object purged on %d servers\n", count)
	return nil
}

func runPurgeBulk(ctx context.Context, cmd *cli.Command, filePath string) error {
	client, err := getClient(ctx)
	if err != nil {
		return err
	}

	fileContent, err := os.ReadFile(filePath)
	if err != nil {
		return fmt.Errorf("failed to read file: %w", err)
	}

	type purgeItemJSON struct {
		Path         string              `json:"path"`
		PreservePath bool                `json:"preserve_path"`
		ExtraHeaders map[string][]string `json:"extra_headers"`
	}

	var items []purgeItemJSON
	if err := json.Unmarshal(fileContent, &items); err != nil {
		return fmt.Errorf("failed to parse JSON: %w", err)
	}

	if len(items) == 0 {
		return fmt.Errorf("no purge items found in file")
	}

	var purgeItems []rpaasclient.PurgeCacheItem
	for _, item := range items {
		purgeItems = append(purgeItems, rpaasclient.PurgeCacheItem{
			Path:         item.Path,
			PreservePath: item.PreservePath,
			ExtraHeaders: item.ExtraHeaders,
		})
	}

	args := rpaasclient.PurgeCacheBulkArgs{
		Instance: cmd.String("instance"),
		Items:    purgeItems,
	}

	results, err := client.PurgeCacheBulk(ctx, args)
	if err != nil {
		return err
	}

	hasErrors := false
	for _, result := range results {
		if result.Error != "" {
			hasErrors = true
			fmt.Fprintf(cmd.Root().Writer, "Path %q: ERROR - %s\n", result.Path, result.Error)
		} else {
			fmt.Fprintf(cmd.Root().Writer, "Path %q: purged on %d servers\n", result.Path, result.InstancesPurged)
		}
	}

	if hasErrors {
		return fmt.Errorf("some purge operations failed")
	}

	return nil
}
