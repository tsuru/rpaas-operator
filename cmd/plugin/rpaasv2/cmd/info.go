import (
	"fmt"

	rpaasclient "github.com/tsuru/rpaas-operator/pkg/rpaas/client"
	"github.com/urfave/cli/v2"
)

func NewCmdInfo() *cli.Command {
	return &cli.Command{
		Name:  "info",
		Usage: "Sets the number of replicas for an instance",
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
			&cli.IntFlag{
				Name:     "replicas",
				Aliases:  []string{"quantity", "q"},
				Usage:    "the desired replicas number",
				Value:    -1,
				Required: true,
			},
		},
		Before: setupClient,
		Action: runScale,
	}
}

func runInfo(c *cli.Context) error {
	client, err := getClient(c)
	if err != nil {
		return err
	}

	scale := rpaasclient.ScaleArgs{
		Instance: c.String("instance"),
		Replicas: int32(c.Int("replicas")),
	}
	_, err = client.Scale(c.Context, scale)
	if err != nil {
		return err
	}

	fmt.Fprintf(c.App.Writer, "%s scaled to %d replica(s)\n", formatInstanceName(c), scale.Replicas)
	return nil
}
