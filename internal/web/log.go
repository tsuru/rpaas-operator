// Copyright 2021 tsuru authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package web

import (
	"strconv"

	"github.com/labstack/echo/v4"
	"github.com/tsuru/rpaas-operator/internal/pkg/rpaas"
)

func extractLogArgs(c echo.Context) rpaas.LogArgs {
	var pLines *int64
	lines, err := strconv.ParseInt(c.QueryParam("lines"), 10, 64)
	if err == nil && lines > 0 {
		pLines = &lines
	}
	var pSince *int64
	since, err := strconv.ParseInt(c.QueryParam("since"), 10, 64)
	if err == nil && since > 0 {
		pSince = &since
	}
	follow, _ := strconv.ParseBool(c.QueryParam("follow"))
	withTimestamp, _ := strconv.ParseBool(c.QueryParam("timestamp"))

	args := rpaas.LogArgs{
		Pod:           c.QueryParam("pod"),
		Container:     c.QueryParam("container"),
		Lines:         pLines,
		Follow:        follow,
		SinceSeconds:  pSince,
		WithTimestamp: withTimestamp,
		Buffer:        c.Response().Writer,
	}

	return args
}

func log(c echo.Context) error {
	args := extractLogArgs(c)
	ctx := c.Request().Context()
	manager, err := getManager(ctx)
	if err != nil {
		return err
	}

	return manager.Log(ctx, c.Param("instance"), args)
}
