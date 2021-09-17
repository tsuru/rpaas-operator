// Copyright 2021 tsuru authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package web

import (
	"encoding/json"
	"io"
	"net/http"
	"regexp"
	"strconv"
	"sync"
	"text/template"
	"time"

	"github.com/fatih/color"

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

func setTemplate() (*template.Template, error) {
	t := "{{color .PodColor .PodName}} {{color .ContainerColor .ContainerName}} {{.Message}}\r\n"
	funcs := map[string]interface{}{
		"json": func(in interface{}) (string, error) {
			b, err := json.Marshal(in)
			if err != nil {
				return "", err
			}
			return string(b), nil
		},
		"color": func(color color.Color, text string) string {
			return color.SprintFunc()(text)
		},
	}
	return template.New("log").Funcs(funcs).Parse(t)
}

func extractLogArgs(c echo.Context) (rpaas.LogArgs, error) {
	params := c.Request().URL.Query()
	var err error

	var pLines *int64
	lines, _ := strconv.ParseInt(params.Get("lines"), 10, 64)
	if lines > 0 {
		pLines = &lines
	}

	tSince := 48 * time.Hour // last 15 minutes by default
	since, _ := strconv.ParseInt(params.Get("since"), 10, 64)
	if since > 0 {
		tSince = time.Second * time.Duration(since)
	}

	follow, _ := strconv.ParseBool(params.Get("follow"))
	withTimestamp, _ := strconv.ParseBool(params.Get("timestamp"))

	queries, err := parseQueries(params.Get("pod"), params.Get("container"))
	if err != nil {
		return rpaas.LogArgs{}, err
	}

	states := []string{"running", "waiting", "terminated"}
	if s, ok := params["states"]; ok && len(s) > 0 {
		states = s
	}

	template, err := setTemplate()
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
		Template:        template,
		ContainerStates: states,
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
