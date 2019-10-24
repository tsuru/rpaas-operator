package types

import (
	"io"
	"os"

	tsuruCmd "github.com/tsuru/tsuru/cmd"
)

type Manager struct {
	Target string
	Token  string
	Writer io.Writer
}

func NewTsuruManager() (*Manager, error) {
	target, err := tsuruCmd.GetTarget()
	if err != nil {
		return nil, err
	}

	token, err := tsuruCmd.ReadToken()
	if err != nil {
		return nil, err
	}

	return &Manager{Target: target, Token: token, Writer: os.Stdout}, nil
}
