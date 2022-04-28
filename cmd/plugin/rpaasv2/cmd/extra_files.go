// Copyright 2022 tsuru authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package cmd

import (
	"fmt"
	"io"
	"io/ioutil"
	"path/filepath"
	"strings"

	"github.com/olekukonko/tablewriter"
	rpaasclient "github.com/tsuru/rpaas-operator/pkg/rpaas/client"
	"github.com/urfave/cli/v2"
)

func NewCmdExtraFiles() *cli.Command {
	return &cli.Command{
		Name:    "extra-files",
		Aliases: []string{"files"},
		Usage:   "Add extra files to the RpaaS filesystem, they will be mounted on: etc/ngnix/extra-files",
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
				Name:     "files",
				Aliases:  []string{"filepaths", "paths", "names"},
				Usage:    "file path of each file",
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
				Name:     "files",
				Aliases:  []string{"filepaths", "paths", "names"},
				Usage:    "file path of each file",
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
				Name:     "files",
				Aliases:  []string{"filepaths", "paths", "names"},
				Usage:    "file path of each file",
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

func prepareFiles(filePathList []string) (map[string][]byte, error) {
	files := map[string][]byte{}
	var err error
	for _, fp := range filePathList {
		path := fp
		if !filepath.IsAbs(fp) {
			path, err = filepath.Abs(fp)
			if err != nil {
				return nil, err
			}
		}
		fileContent, err := ioutil.ReadFile(path)
		if err != nil {
			return nil, err
		}
		files[fp] = fileContent
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

	files, err := prepareFiles(c.StringSlice("files"))
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
	for name := range files {
		fNames = append(fNames, name)
	}
	extraFilesSuccessMessage(c, "Added", "to", instance, fNames)
	return nil
}

func runUpdateExtraFiles(c *cli.Context) error {
	client, err := getClient(c)
	if err != nil {
		return err
	}

	files, err := prepareFiles(c.StringSlice("files"))
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
	for name := range files {
		fNames = append(fNames, name)
	}
	extraFilesSuccessMessage(c, "Updated", "on", instance, fNames)
	return nil
}

func runDeleteExtraFiles(c *cli.Context) error {
	client, err := getClient(c)
	if err != nil {
		return err
	}

	files := c.StringSlice("files")
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

func writeExtraFilesOnTableFormat(writer io.Writer, files map[string]string) {
	data := [][]string{}
	for name, content := range files {
		data = append(data, []string{name, content})
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

	instance := c.String("instance")

	files, err := client.ListExtraFiles(c.Context, instance)
	if err != nil {
		return err
	}
	switch c.Bool("show-content") {
	default:
		for _, file := range files {
			fmt.Fprintln(c.App.Writer, file)
		}
	case true:
		fileMap := map[string]string{}
		for _, name := range files {
			f, err := client.GetExtraFile(c.Context, rpaasclient.GetExtraFileArgs{
				Instance: instance,
				FileName: name,
			})
			if err != nil {
				fileMap[name] = err.Error()
			} else {
				fileMap[name] = strings.TrimSuffix(string(f.Content), "\n")
			}
		}
		writeExtraFilesOnTableFormat(c.App.Writer, fileMap)
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
