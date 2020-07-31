// Copyright 2020 tsuru authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package api

import (
	"io"
	"net/http"

	"github.com/labstack/echo/v4"
	"github.com/tsuru/rpaas-operator/internal/pkg/rpaas"
)

type cmdReadWrite struct {
	body   io.Reader
	writer io.Writer
}

func (c *cmdReadWrite) Write(arr []byte) (int, error) {
	defer func() {
		flusher, _ := c.writer.(http.Flusher)
		flusher.Flush()
	}()

	return c.writer.Write(arr)
}

func (c *cmdReadWrite) Read(arr []byte) (int, error) {
	return c.body.Read(arr)
}

func exec(c echo.Context) error {
	manager, err := getManager(c)
	if err != nil {
		return err
	}

	var args rpaas.ExecArgs
	if err := c.Bind(&args); err != nil {
		return err
	}

	buffer := &cmdReadWrite{
		body:   c.Request().Body,
		writer: c.Response().Writer,
	}

	return manager.Exec(c.Request().Context(), c.Param("instance"), rpaas.ExecArgs{
		Tty:            args.Tty,
		Command:        args.Command,
		TerminalWidth:  args.TerminalWidth,
		TerminalHeight: args.TerminalHeight,
		Stdin:          buffer,
		Stdout:         buffer,
		Stderr:         buffer,
	})
}
