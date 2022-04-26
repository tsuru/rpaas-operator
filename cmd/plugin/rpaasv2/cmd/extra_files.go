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
		Usage:   "Add extra files to the RpaaS filesystem, they will be mounted on: etc/ngnix/extra-files",
		Subcommands: []*cli.Command{
			NewCmdAddExtraFiles(),
			NewCmdUpdateExtraFiles(),
			NewCmdDeleteExtraFiles(),
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

func runAddExtraFiles(c *cli.Context) error {
	client, err := getClient(c)
	if err != nil {
		return err
	}

	files, err := prepareFiles(c.StringSlice("files"))
	if err != nil {
		return err
	}

	err = client.AddExtraFiles(c.Context, rpaasclient.ExtraFilesArgs{
		Instance: c.String("instance"),
		Files:    files,
	})
	if err != nil {
		return err
	}

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

	err = client.UpdateExtraFiles(c.Context, rpaasclient.ExtraFilesArgs{
		Instance: c.String("instance"),
		Files:    files,
	})
	if err != nil {
		return err
	}

	return nil
}

func runDeleteExtraFiles(c *cli.Context) error {
	client, err := getClient(c)
	if err != nil {
		return err
	}

	files := c.StringSlice("files")

	err = client.DeleteExtraFiles(c.Context, rpaasclient.DeleteExtraFilesArgs{
		Instance: c.String("instance"),
		Files:    files,
	})
	if err != nil {
		return err
	}

	return nil
}
