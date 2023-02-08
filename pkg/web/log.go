// Copyright 2021 tsuru authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package web

import (
	"errors"
	"io"
	"net/http"
	"strconv"
	"sync"

	"github.com/labstack/echo/v4"
	"github.com/tsuru/rpaas-operator/internal/pkg/rpaas"
)

type flushWriter struct {
	w      io.Writer
	m      sync.Mutex
	closed bool
}

func (fw *flushWriter) Write(p []byte) (int, error) {
	fw.m.Lock()
	defer fw.m.Unlock()

	if fw.closed {
		return 0, nil
	}

	if fw.w == nil {
		return 0, errors.New("no writer available")
	}

	n, err := fw.w.Write(p)
	if err != nil {
		return 0, err
	}

	if n < len(p) {
		return 0, io.ErrShortWrite
	}

	fw.w.Write([]byte{'\n'}) // carrier return

	if f, ok := fw.w.(http.Flusher); ok {
		f.Flush()
	}

	return n, nil
}

func (fw *flushWriter) Close() {
	fw.m.Lock()
	defer fw.m.Unlock()

	fw.closed = true
	fw.w = io.Discard
}

func extractLogArgs(c echo.Context, w io.Writer) rpaas.LogArgs {
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
		Stdout:    w,
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

	w := &flushWriter{w: c.Response()}
	defer w.Close()

	return manager.Log(c.Request().Context(), c.Param("instance"), extractLogArgs(c, w))
}
