// Copyright 2022 tsuru authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package cmd

import (
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/olekukonko/tablewriter"
	rpaasclient "github.com/tsuru/rpaas-operator/pkg/rpaas/client"
	"github.com/tsuru/rpaas-operator/pkg/rpaas/client/types"
	"github.com/urfave/cli/v2"
)

func NewCmdExtraFiles() *cli.Command {
	return &cli.Command{
		Name:    "extra-files",
		Aliases: []string{"files"},
		Usage:   "Manages persistent files in the instance filesystem",
		Subcommands: []*cli.Command{
			NewCmdAddExtraFiles(),
			NewCmdUpdateExtraFiles(),
			NewCmdDeleteExtraFiles(),
			NewCmdListExtraFiles(),
			NewCmdGetExtraFile(),
		},
	}
}

func NewCmdAddExtraFiles() *cli.Command {
	return &cli.Command{
		Name:  "add",
		Usage: "Uploads new files",
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
			&cli.StringSliceFlag{
				Name:     "file",
				Usage:    "path in the local filesystem to the file",
				Required: true,
			},
		},
		Before: setupClient,
		Action: runAddExtraFiles,
	}
}

func NewCmdUpdateExtraFiles() *cli.Command {
	return &cli.Command{
		Name:  "update",
		Usage: "Uploads existing files",
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
			&cli.StringSliceFlag{
				Name:     "file",
				Usage:    "path in the local filesystem to the file",
				Required: true,
			},
		},
		Before: setupClient,
		Action: runUpdateExtraFiles,
	}
}

func NewCmdDeleteExtraFiles() *cli.Command {
	return &cli.Command{
		Name:    "delete",
		Aliases: []string{"remove"},
		Usage:   "Deletes files",
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
			&cli.StringSliceFlag{
				Name:     "file",
				Usage:    "the name of the file",
				Required: true,
			},
		},
		Before: setupClient,
		Action: runDeleteExtraFiles,
	}
}

func NewCmdListExtraFiles() *cli.Command {
	return &cli.Command{
		Name:  "list",
		Usage: "Shows all extra-files inside the instance and it's contents",
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
				Name:     "show-content",
				Usage:    "shows the content of each file on plain text format",
				Required: false,
			},
		},
		Before: setupClient,
		Action: runListExtraFiles,
	}
}

func NewCmdGetExtraFile() *cli.Command {
	return &cli.Command{
		Name:    "get",
		Aliases: []string{"show"},
		Usage:   "Displays the content of the specified file in plain text",
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
				Name:     "file",
				Usage:    "the name of the file",
				Required: true,
			},
		},
		Before: setupClient,
		Action: runGetExtraFile,
	}
}

func prepareFiles(filePathList []string) ([]types.RpaasFile, error) {
	files := []types.RpaasFile{}
	for _, fp := range filePathList {
		fileContent, err := os.ReadFile(fp)
		if err != nil {
			return nil, err
		}
		files = append(files, types.RpaasFile{
			Name:    fp,
			Content: fileContent,
		})
	}

	return files, nil
}

func extraFilesSuccessMessage(c *cli.Context, prefix, suffix, instance string, files []string) {
	fmt.Fprintf(c.App.Writer, "%s ", prefix)
	fmt.Fprintf(c.App.Writer, "[%s]", strings.Join(files, ", "))
	fmt.Fprintf(c.App.Writer, " %s %s\n", suffix, instance)
}

func runAddExtraFiles(c *cli.Context) error {
	client, err := getClient(c)
	if err != nil {
		return err
	}

	files, err := prepareFiles(c.StringSlice("file"))
	if err != nil {
		return err
	}

	instance := c.String("instance")
	err = client.AddExtraFiles(c.Context, rpaasclient.ExtraFilesArgs{
		Instance: instance,
		Files:    files,
	})
	if err != nil {
		return err
	}

	fNames := []string{}
	for _, file := range files {
		fNames = append(fNames, file.Name)
	}
	extraFilesSuccessMessage(c, "Added", "to", instance, fNames)
	return nil
}

func runUpdateExtraFiles(c *cli.Context) error {
	client, err := getClient(c)
	if err != nil {
		return err
	}

	files, err := prepareFiles(c.StringSlice("file"))
	if err != nil {
		return err
	}

	instance := c.String("instance")
	err = client.UpdateExtraFiles(c.Context, rpaasclient.ExtraFilesArgs{
		Instance: c.String("instance"),
		Files:    files,
	})
	if err != nil {
		return err
	}

	fNames := []string{}
	for _, file := range files {
		fNames = append(fNames, file.Name)
	}
	extraFilesSuccessMessage(c, "Updated", "on", instance, fNames)
	return nil
}

func runDeleteExtraFiles(c *cli.Context) error {
	client, err := getClient(c)
	if err != nil {
		return err
	}

	files := c.StringSlice("file")
	instance := c.String("instance")
	err = client.DeleteExtraFiles(c.Context, rpaasclient.DeleteExtraFilesArgs{
		Instance: instance,
		Files:    files,
	})
	if err != nil {
		return err
	}

	extraFilesSuccessMessage(c, "Removed", "from", instance, files)
	return nil
}

func writeExtraFilesOnTableFormat(writer io.Writer, files []types.RpaasFile) {
	data := [][]string{}
	for _, file := range files {
		data = append(data, []string{file.Name, string(file.Content)})
	}

	table := tablewriter.NewWriter(writer)
	table.SetHeader([]string{"Name", "Content"})
	table.SetAutoWrapText(false)
	table.SetRowLine(true)
	table.SetAutoFormatHeaders(false)
	table.SetAlignment(tablewriter.ALIGN_LEFT)
	table.AppendBulk(data)
	table.Render()
}

func runListExtraFiles(c *cli.Context) error {
	client, err := getClient(c)
	if err != nil {
		return err
	}

	showContent := c.Bool("show-content")
	args := rpaasclient.ListExtraFilesArgs{
		Instance:    c.String("instance"),
		ShowContent: showContent,
	}
	files, err := client.ListExtraFiles(c.Context, args)
	if err != nil {
		return err
	}
	switch showContent {
	default:
		for _, file := range files {
			fmt.Fprintln(c.App.Writer, file.Name)
		}
	case true:
		writeExtraFilesOnTableFormat(c.App.Writer, files)
	}
	return nil
}

func runGetExtraFile(c *cli.Context) error {
	client, err := getClient(c)
	if err != nil {
		return err
	}

	file, err := client.GetExtraFile(c.Context, rpaasclient.GetExtraFileArgs{
		Instance: c.String("instance"),
		FileName: c.String("file"),
	})
	if err != nil {
		return err
	}

	fmt.Fprintln(c.App.Writer, strings.TrimSuffix(string(file.Content), "\n"))
	return nil
}
