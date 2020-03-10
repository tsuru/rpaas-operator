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
{{- with .Team }}
Team: {{ . }}
{{- end }}
{{- with .Description }}
Description: {{ . }}
{{- end }}
{{- with .Binds }}
Binds:
{{- range $index, $bind:= . }}{{- with not $index }}{{ end }}
{{- with $bind }}
    #Bind {{ $index }}
    App: {{ .Name }}
    Host: {{ .Host }} 
{{- end }}
{{- end }}
{{- end }}
{{- with .Tags }}
Tags:
{{- range $index, $tag := . }}{{- with not $index }}{{ end }}
    {{ $tag }}
{{- end }}
{{- end }}
{{- with .Address }}
Adresses:
{{- range $index, $address := . }}
    #Address {{ $index }}:
{{- with $address }}
        Hostname: {{ .Hostname }}
        Ip: {{ .Ip }}
{{- end }}
{{- end }}
{{- end }}
{{- with .Replicas }}
Replicas: {{ . }}
{{- end }}
{{- with .Plan }}
Plan: {{ . }}
{{- end }}
{{- with .Locations }}
Locations:
{{- range $index, $location := . }}
    #Location {{ $index }}
{{- with $location }}
    Path: {{ .Path }}
    Destination: {{ .Destination }}
{{- end }}
{{- end }}
{{- end }}
{{- with .Autoscale }}{{ "\n" }}
Autoscale: {{ .Autoscale }}
{{- end }}
`
	return template.New("root").Parse(tmp)
}

func writeInfoOnJSONFormat(w io.Writer, payload *rpaas.InstanceInfo) error {
	message, err := json.Marshal(payload)
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
