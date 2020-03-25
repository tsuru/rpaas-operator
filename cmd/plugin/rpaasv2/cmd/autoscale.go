package cmd

import (
	"encoding/json"
	"fmt"
	"io"
	"strconv"

	"github.com/olekukonko/tablewriter"
	rpaasclient "github.com/tsuru/rpaas-operator/pkg/rpaas/client"
	clientTypes "github.com/tsuru/rpaas-operator/pkg/rpaas/client/types"
	"github.com/urfave/cli/v2"
)

func NewCmdAutoscale() *cli.Command {
	return &cli.Command{
		Name:  "autoscale",
		Usage: "Manages the autoscale spec of the defined instance",
		Subcommands: []*cli.Command{
			NewCmdGetAutoscale(),
		},
	}
}

func NewCmdGetAutoscale() *cli.Command {
	return &cli.Command{
		Name:  "get",
		Usage: "Retrieves autoscale configuration of the desired instance",
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
				Aliases: []string{"r", "raw"},
				Usage:   "show as JSON instead of go template format",
				Value:   false,
			},
		},
		Before: setupClient,
		Action: runGetAutoscale,
	}
}

func writeAutoscaleJSON(w io.Writer, spec *clientTypes.Autoscale) error {
	message, err := json.MarshalIndent(spec, "", "\t")
	if err != nil {
		return err
	}

	fmt.Fprintln(w, string(message))
	return nil
}

func writeAutoscale(w io.Writer, autoscale *clientTypes.Autoscale) {
	if autoscale == nil {
		return
	}
	table := tablewriter.NewWriter(w)
	table.SetHeader([]string{"Replicas", "Target Utilization"})
	table.SetAutoWrapText(true)
	table.SetRowLine(false)
	var max, min, cpuPercentage, memPercentage string

	if autoscale.MaxReplicas != nil {
		max = fmt.Sprintf("Max: %s", strconv.Itoa(int(*autoscale.MaxReplicas)))
	} else {
		max = "Max: N/A"
	}
	if autoscale.MinReplicas != nil {
		min = fmt.Sprintf("Min: %s", strconv.Itoa(int(*autoscale.MinReplicas)))
	} else {
		min = "Min: N/A"
	}
	if autoscale.CPU != nil {
		cpuPercentage = fmt.Sprintf("CPU: %s%%", strconv.Itoa(int(*autoscale.CPU)))
	} else {
		cpuPercentage = "CPU: N/A"
	}
	if autoscale.Memory != nil {
		memPercentage = fmt.Sprintf("Memory: %s%%", strconv.Itoa(int(*autoscale.Memory)))
	} else {
		memPercentage = "Memory: N/A"
	}
	data := [][]string{
		{max, cpuPercentage},
		{min, memPercentage},
	}
	table.AppendBulk(data)
	table.Render()
}

func runGetAutoscale(c *cli.Context) error {
	client, err := getClient(c)
	if err != nil {
		return err
	}

	args := rpaasclient.GetAutoscaleArgs{
		Instance: c.String("instance"),
		Raw:      c.Bool("raw-output"),
	}

	spec, _, err := client.GetAutoscale(c.Context, args)
	if err != nil {
		return err
	}

	if args.Raw {
		return writeAutoscaleJSON(c.App.Writer, spec)
	}

	if spec != nil {
		writeAutoscale(c.App.Writer, spec)
	}

	return nil
}
