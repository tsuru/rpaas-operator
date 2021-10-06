// Copyright 2021 tsuru authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package web

import (
	"fmt"
	"io"
	"net/http"
	"strconv"
	"sync"

	"github.com/labstack/echo/v4"
	"github.com/tsuru/rpaas-operator/internal/pkg/rpaas"
)

type flushWriter struct {
	sync.Mutex
	io.Writer
}

func (fw *flushWriter) Write(p []byte) (int, error) {
	fw.Lock()
	defer fw.Unlock()

	n, err := fmt.Fprintln(fw.Writer, string(p))
	if err != nil {
		return n, err
	}

	if f, ok := fw.Writer.(http.Flusher); ok {
		f.Flush()
	}

	return n, nil
}

func extractLogArgs(c echo.Context) rpaas.LogArgs {
	params := c.Request().URL.Query()

	var lines *int64
	if l, err := strconv.ParseInt(params.Get("lines"), 10, 64); err == nil && l > 0 {
		lines = &l
	}

	var since *int64
	if s, err := strconv.ParseInt(params.Get("since"), 10, 64); err == nil && s > 0 {
		since = &s
	}

	follow, _ := strconv.ParseBool(params.Get("follow"))
	color, _ := strconv.ParseBool(params.Get("color"))

	return rpaas.LogArgs{
		Stdout:    &flushWriter{Writer: c.Response().Writer},
		Stderr:    io.Discard,
		Pod:       params.Get("pod"),
		Container: params.Get("container"),
		Lines:     lines,
		Follow:    follow,
		Color:     color,
		Since:     since,
	}
}

func log(c echo.Context) error {
	manager, err := getManager(c.Request().Context())
	if err != nil {
		return err
	}

	return manager.Log(c.Request().Context(), c.Param("instance"), extractLogArgs(c))
}
