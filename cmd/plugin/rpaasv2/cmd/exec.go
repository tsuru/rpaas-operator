package cmd

import (
	"io"
	"log"
	"os"
	"strconv"

	rpaasclient "github.com/tsuru/rpaas-operator/pkg/rpaas/client"
	"github.com/urfave/cli/v2"
	"golang.org/x/crypto/ssh/terminal"
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

	fd := int(os.Stdin.Fd())
	var width, height int
	if terminal.IsTerminal(fd) {
		width, height, _ = terminal.GetSize(fd)
		oldState, terminalErr := terminal.MakeRaw(fd)
		if terminalErr != nil {
			return err
		}
		defer terminal.Restore(fd, oldState)
	}
	execArgs.TerminalHeight = strconv.Itoa(height)
	execArgs.TerminalWidth = strconv.Itoa(width)

	resp, err := client.Exec(c.Context, execArgs)
	if err != nil {
		return err
	}

	_, err = io.Copy(os.Stdout, resp.Body)
	if err != nil {
		log.Fatal(err)
	}

	return nil
}
