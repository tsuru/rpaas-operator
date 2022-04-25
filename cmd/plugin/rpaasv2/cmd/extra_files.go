// Copyright 2022 tsuru authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package cmd

import (
	"io/ioutil"
	"path/filepath"

	rpaasclient "github.com/tsuru/rpaas-operator/pkg/rpaas/client"
	"github.com/urfave/cli/v2"
)

func NewCmdExtraFiles() *cli.Command {
	return &cli.Command{
		Name:    "extra-files",
		Aliases: []string{"files"},
		Usage:   "Extra files add into nginx filesystem",
		Subcommands: []*cli.Command{
			NewCmdAddExtraFile(),
		},
	}
}

func NewCmdAddExtraFile() *cli.Command {
	return &cli.Command{
		Name:    "add",
		Aliases: []string{"update"},
		Usage:   "Uploads a new file to the instance",
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
				Usage:    "file path of the file",
				Required: true,
			},
		},
		Before: setupClient,
		Action: runAddExtraFiles,
	}
}

func runAddExtraFiles(c *cli.Context) error {
	client, err := getClient(c)
	if err != nil {
		return err
	}

	files := map[string][]byte{}
	for _, fp := range c.StringSlice("files") {
		path := fp
		if !filepath.IsAbs(fp) {
			path, err = filepath.Abs(fp)
			if err != nil {
				return err
			}
		}
		fileContent, err := ioutil.ReadFile(path)
		if err != nil {
			return err
		}
		files[fp] = fileContent
	}

	err = client.ExtraFiles(c.Context, rpaasclient.ExtraFilesArgs{
		Instance: c.String("instance"),
		Files:    files,
	})
	if err != nil {
		return err
	}

	return nil
}
