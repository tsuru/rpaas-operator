// Copyright 2021 tsuru authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package web

import (
	"io"
	"net/http"
	"regexp"
	"strconv"
	"sync"

	"github.com/labstack/echo/v4"
	"github.com/tsuru/rpaas-operator/internal/pkg/rpaas"
)

type flushWriter struct {
	sync.Mutex

	w io.Writer
}

func (fw *flushWriter) Write(p []byte) (int, error) {
	fw.Lock()
	defer fw.Unlock()

	n, err := fw.w.Write(p)
	if err != nil {
		return n, err
	}

	if f, ok := fw.w.(http.Flusher); ok && fw.w != nil {
		f.Flush()
	}
	return n, nil
}

func parseQueries(pod, container string) (map[string]*regexp.Regexp, error) {
	var p *regexp.Regexp
	var c *regexp.Regexp
	var err error

	everything, err := regexp.Compile(`.*`)
	if err != nil {
		return nil, err
	}

	if pod != "" {
		p, err = regexp.Compile(pod)
		if err != nil {
			return nil, err
		}
	} else {
		p = everything
	}

	if container != "" {
		c, err = regexp.Compile(container)
		if err != nil {
			return nil, err
		}
	} else {
		c = everything
	}

	return map[string]*regexp.Regexp{
		"pod":       p,
		"container": c,
	}, nil
}

func extractLogArgs(c echo.Context) (rpaas.LogArgs, error) {
	params := c.Request().URL.Query()
	var err error

	var pLines *int64
	lines, _ := strconv.ParseInt(params.Get("lines"), 10, 64)
	if lines > 0 {
		pLines = &lines
	}

	tSince := int64(24 * 60 * 60) // last 24 hours by default
	if since, _ := strconv.ParseInt(params.Get("since"), 10, 64); since > 0 {
		tSince = since
	}

	follow, _ := strconv.ParseBool(params.Get("follow"))
	withTimestamp, _ := strconv.ParseBool(params.Get("timestamp"))
	color, _ := strconv.ParseBool(params.Get("color"))

	queries, err := parseQueries(params.Get("pod"), params.Get("container"))
	if err != nil {
		return rpaas.LogArgs{}, err
	}

	args := rpaas.LogArgs{
		Pod:           queries["pod"],
		Container:     queries["container"],
		Lines:         pLines,
		Follow:        follow,
		Since:         tSince,
		WithTimestamp: withTimestamp,
		Buffer: &flushWriter{
			w: c.Response().Writer,
		},
		Color: color,
	}

	return args, nil
}

func log(c echo.Context) error {
	args, err := extractLogArgs(c)
	if err != nil {
		return err
	}
	ctx := c.Request().Context()
	manager, err := getManager(ctx)
	if err != nil {
		return err
	}

	return manager.Log(ctx, c.Param("instance"), args)
}
