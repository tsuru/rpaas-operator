// Copyright 2020 tsuru authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package cmd

import (
	"encoding/json"
	"fmt"
	"io"
	"text/template"

	"github.com/tsuru/rpaas-operator/internal/pkg/rpaas"
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
			&cli.BoolFlag{
				Name:    "raw-output",
				Aliases: []string{"r"},
				Usage:   "show as JSON instead of go template format",
				Value:   false,
			},
		},
		Before: setupClient,
		Action: runInfo,
	}
}

func prepareTemplate() (*template.Template, error) {
	tmp := `
{{- with .Name }}
Name: {{ . }}
{{- end }}
{{- with .Team }}{{ "\n" }}
Team: {{ . }}
{{- end }}
{{- with .Description }}{{ "\n" }}
Description: {{ . }}
{{ end }}
Binds:
{{- range $index, $bind:= .Binds }}{{- with not $index }}{{ end }}
{{- with $bind }}
    App: {{ .Name }}
    Host: {{ .Host }} 
{{ end }}
{{- end }}
Tags:
{{- with .Tags }}
{{- range $index, $tag := . }}{{- with not $index }}{{ end }}
    {{ $tag}}
{{- end }}
{{- end }}
{{- with .Address }}{{ "\n" }}
Address:
    Hostname: {{ .Hostname }}
    Ip: {{ .Ip }}
{{- end }}
{{- with .Replicas }}{{ "\n" }}
Replicas: {{ . }}
{{- end }}
{{- with .Plan }}{{ "\n" }}
Plan: {{ . }}
{{ end }}
Locations:
{{- with .Locations }}
{{- range $index, $location := . }}
{{- with $location }}
    Path: {{ .Path }}
    Destination: {{ .Destination }}
{{- end }}
{{- end }}
{{- end }}
Service: {{ .Service }}
Autoscale: {{ .Autoscale }}
`
	return template.New("root").Parse(tmp)
}

func writeInfoOnJSONFormat(w io.Writer, payload *rpaas.InfoBuilder) error {
	message, err := json.MarshalIndent(payload, "", "\t")
	if err != nil {
		return err
	}

	fmt.Fprintln(w, string(message))
	return nil
}

func runInfo(c *cli.Context) error {
	client, err := getClient(c)
	if err != nil {
		return err
	}

	info := rpaasclient.InfoArgs{
		Instance: c.String("instance"),
		Service:  c.String("service"),
		Raw:      c.Bool("raw-output"),
	}

	infoPayload, _, err := client.Info(c.Context, info)
	if err != nil {
		return err
	}

	if info.Raw {
		return writeInfoOnJSONFormat(c.App.Writer, infoPayload)
	}

	tmpl, err := prepareTemplate()
	if err != nil {
		return err
	}
	if infoPayload != nil {
		err = tmpl.Execute(c.App.Writer, infoPayload)
		if err != nil {
			return err
		}
	}

	return nil
}
