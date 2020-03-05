// Copyright 2020 tsuru authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package cmd

import (
	"text/template"

	rpaasclient "github.com/tsuru/rpaas-operator/pkg/rpaas/client"
	"github.com/urfave/cli/v2"
)

func NewCmdInfo() *cli.Command {
	return &cli.Command{
		Name:  "info",
		Usage: "Retrieves information of the rpaas-operator instance given",
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
		Action: runInfo,
	}
}

func prepareTemplate() (*template.Template, error) {
	tmp := `
{{- with .Address }}{{ "\n" }}
Address:
    Hostname: {{ .Hostname }}
    Ip: {{ .Ip }}
{{ end }}
{{- with .Replicas }}{{ "\n" }}
Replicas: {{ . }}
{{- end }}
{{- with .Plan }}{{ "\n" }}
Plan: {{ . }}
{{- end }}
{{- with .Locations }}{{ "\n" }}
{{- range $index, $location := . }}{{ with not $index }}{{ "\n" }}{{ end }}
{{- end }}
{{- end }}
Locations: {{ .Locations }}
Service; {{ .Service }}
Autoscale: {{ .Autoscale }}
Binds: {{ .Binds }}
{{- with .Team }}{{ "\n" }}
Team: {{ . }}
{{ end }}
{{- with .Name }}{{ "\n" }}
Name: {{ . }}
{{ end }}
{{- with .Description }}{{ "\n" }}
Description: {{ . }}
{{ end }}
{{- range $index, $tag := . }}{{ with not $index }}{{ "\n" }}{{ end }}
  $index: {{ $tag}}
{{- end }}
`
	return template.New("root").Parse(tmp)
}

func runInfo(c *cli.Context) error {
	client, err := getClient(c)
	if err != nil {
		return err
	}

	info := rpaasclient.InfoArgs{
		Instance: c.String("instance"),
		Service:  c.String("service"),
	}

	infoPayload, _, err := client.Info(c.Context, info)
	if err != nil {
		return err
	}

	tmpl, err := prepareTemplate()
	if err != nil {
		return err
	}
	if infoPayload != nil {
		tmpl.Execute(c.App.Writer, infoPayload)
	}

	return nil
}
