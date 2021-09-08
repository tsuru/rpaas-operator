package cmd

import (
	rpaasclient "github.com/tsuru/rpaas-operator/pkg/rpaas/client"
	"github.com/urfave/cli/v2"
)

func NewCmdLogs() *cli.Command {
	return &cli.Command{
		Name:    "logs",
		Usage:   "Fetches logs from a rpaasv2 instance pod",
		Aliases: []string{"log"},
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
				Name:     "pod",
				Aliases:  []string{"p"},
				Usage:    "specific pod to log from",
				Required: false,
			},
			&cli.PathFlag{
				Name:     "container",
				Aliases:  []string{"c"},
				Usage:    "specific container to log from",
				Required: false,
			},
			&cli.IntFlag{
				Name:     "lines",
				Aliases:  []string{"l"},
				Usage:    "number of earlier log lines to show",
				Required: false,
			},
			&cli.IntFlag{
				Name:     "since",
				Usage:    "only return logs newer than a relative duration like 5s, 2m, or 3h. Defaults to all logs.",
				Required: false,
			},
			&cli.BoolFlag{
				Name:     "follow",
				Aliases:  []string{"f"},
				Usage:    "specify if the logs should be streamed",
				Required: false,
			},
			&cli.BoolFlag{
				Name:     "timestamp",
				Aliases:  []string{"with-timestamp"},
				Usage:    "include timestamps on each line in the log output",
				Required: false,
			},
		},
		Before: setupClient,
		Action: runLogRpaas,
	}
}

func runLogRpaas(c *cli.Context) error {
	client, err := getClient(c)
	if err != nil {
		return err
	}

	return client.Log(c.Context, rpaasclient.LogArgs{
		Instance:      c.String("instance"),
		Lines:         c.Int("lines"),
		Since:         c.Int("since"),
		Follow:        c.Bool("follow"),
		WithTimestamp: c.Bool("timestamp"),
		Pod:           c.String("pod"),
		Container:     c.String("container"),
	})
}
