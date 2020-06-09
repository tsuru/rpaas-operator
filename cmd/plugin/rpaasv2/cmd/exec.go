package cmd

import (
	"bufio"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"os"

	rpaasclient "github.com/tsuru/rpaas-operator/pkg/rpaas/client"
	"github.com/urfave/cli/v2"
)

func NewCmdExec() *cli.Command {
	return &cli.Command{
		Name:  "exec",
		Usage: "Runs a shell inside a rpaasv2 instance or just a single command",
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
				Name:    "command",
				Aliases: []string{"c"},
				Usage:   "command that is supposed to be executed",
			},
			&cli.BoolFlag{
				Name:  "tty",
				Usage: "attaches input/output environment",
			},
		},
		Before: setupClient,
		Action: runExec,
	}
}

type bodyReader struct {
	body io.Reader
}

func (b *bodyReader) Read(arr []byte) (int, error) {
	reader := bufio.NewReader(b.body)
	line, err := reader.ReadBytes('\n')
	if err != nil {
		return 0, err
	}

	return copy(arr, line), nil
}

func runExec(c *cli.Context) error {
	client, err := getClient(c)
	if err != nil {
		return err
	}

	execArgs := rpaasclient.ExecArgs{
		Instance: c.String("instance"),
		Tty:      c.Bool("tty"),
		Reader:   os.Stdin,
	}
	if c.Bool("tty") {
		execArgs.Command = []string{"sh"}
	}
	if c.String("command") != "" {
		execArgs.Command = append(execArgs.Command, c.String("command"))
	}
	resp, err := client.Exec(c.Context, execArgs)
	if err != nil {
		return err
	}

	bodyReader := &bodyReader{
		body: resp.Body,
	}

	go func() {
		// _, err = io.Copy(os.Stdout, bodyReader)
		bb, err := ioutil.ReadAll(bodyReader)
		fmt.Printf("%s\n", string(bb))
		if err != nil {
			log.Fatal(err)
		}
	}()

	return nil
}
