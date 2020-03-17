// Copyright 2020 tsuru authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package cmd

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"strconv"
	"text/template"

	"github.com/olekukonko/tablewriter"
	"github.com/tsuru/rpaas-operator/pkg/apis/extensions/v1alpha1"
	rpaasclient "github.com/tsuru/rpaas-operator/pkg/rpaas/client"
	clientTypes "github.com/tsuru/rpaas-operator/pkg/rpaas/client/types"
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
{{- with .Replicas }}
Replicas: {{ . }}
{{- end }}
{{- with .Plan }}
Plan: {{ . }}
{{- end }}
{{- with .Binds }}

Binds:
{{ formatBinds . }}
{{- end }}
{{- with .Tags }}
Tags:
{{ formatTags . }}
{{- end }}
{{- with .Address }}

Adresses:
{{ formatAdresses . }}
{{- end }}

{{- with .Locations }}
Locations:
{{ formatLocations . }}
{{- end }}
{{- with .Autoscale }}
Autoscale:
{{ formatAutoscale . }}
{{- end }}
`

	return template.New("root").Funcs(template.FuncMap{
		"formatTags":      writeTagsOnTableFormat,
		"formatLocations": writeLocationsOnTableFormat,
		"formatAdresses":  writeAdressesOnTableFormat,
		"formatBinds":     writeBindsOnTableFormat,
		"formatAutoscale": writeAutoscaleOnTableFormat,
	}).Parse(tmp)
}

func writeAutoscaleOnTableFormat(autoscale *v1alpha1.RpaasInstanceAutoscaleSpec) string {
	var buffer bytes.Buffer
	table := tablewriter.NewWriter(&buffer)
	table.SetHeader([]string{"Replicas", "Target Utilization"})
	table.SetAutoWrapText(true)
	table.SetRowLine(false)
	max := fmt.Sprintf("Max: %s", strconv.Itoa(int(autoscale.MaxReplicas)))
	min := fmt.Sprintf("Min: %s", strconv.Itoa(int(*autoscale.MinReplicas)))
	cpuPercentage := fmt.Sprintf("CPU: %s%%", strconv.Itoa(int(*autoscale.TargetCPUUtilizationPercentage)))
	memPercentage := fmt.Sprintf("Memory: %s%%", strconv.Itoa(int(*autoscale.TargetMemoryUtilizationPercentage)))
	data := [][]string{
		{max, cpuPercentage},
		{min, memPercentage},
	}
	table.AppendBulk(data)
	table.Render()

	return buffer.String()
}

func writeAdressesOnTableFormat(adresses []clientTypes.InstanceAddress) string {
	data := [][]string{}
	for _, address := range adresses {
		data = append(data, []string{address.Hostname, address.IP})
	}
	var buffer bytes.Buffer
	table := tablewriter.NewWriter(&buffer)
	table.SetHeader([]string{"Hostname", "IP"})
	table.SetRowLine(true)
	table.SetAutoWrapText(true)
	table.SetColumnAlignment([]int{tablewriter.ALIGN_LEFT, tablewriter.ALIGN_CENTER, tablewriter.ALIGN_CENTER, tablewriter.ALIGN_LEFT})
	table.AppendBulk(data)
	table.Render()

	return buffer.String()
}

func writeLocationsOnTableFormat(locations []v1alpha1.Location) string {
	data := [][]string{}
	for _, location := range locations {
		data = append(data, []string{location.Path, location.Destination})
	}
	var buffer bytes.Buffer
	table := tablewriter.NewWriter(&buffer)
	table.SetHeader([]string{"Path", "Destination"})
	table.SetRowLine(true)
	table.SetAutoWrapText(true)
	table.SetColumnAlignment([]int{tablewriter.ALIGN_LEFT, tablewriter.ALIGN_CENTER, tablewriter.ALIGN_CENTER, tablewriter.ALIGN_LEFT})
	table.AppendBulk(data)
	table.Render()

	return buffer.String()
}

func writeTagsOnTableFormat(tags []string) string {
	data := [][]string{}
	for _, tag := range tags {
		data = append(data, []string{tag})
	}
	var buffer bytes.Buffer
	table := tablewriter.NewWriter(&buffer)
	table.SetRowLine(true)
	table.SetAutoWrapText(true)
	table.SetColumnAlignment([]int{tablewriter.ALIGN_LEFT, tablewriter.ALIGN_CENTER, tablewriter.ALIGN_CENTER, tablewriter.ALIGN_LEFT})
	table.AppendBulk(data)
	table.Render()

	return buffer.String()
}

func writeBindsOnTableFormat(binds []v1alpha1.Bind) string {
	data := [][]string{}
	for _, bind := range binds {
		data = append(data, []string{bind.Name, bind.Host})
	}
	var buffer bytes.Buffer
	table := tablewriter.NewWriter(&buffer)
	table.SetHeader([]string{"App", "Address"})
	table.SetRowLine(true)
	table.SetAutoWrapText(true)
	table.SetColumnAlignment([]int{tablewriter.ALIGN_LEFT, tablewriter.ALIGN_CENTER, tablewriter.ALIGN_CENTER, tablewriter.ALIGN_LEFT})
	table.AppendBulk(data)
	table.Render()

	return buffer.String()
}

func writeInfoOnJSONFormat(w io.Writer, payload *clientTypes.InstanceInfo) error {
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
