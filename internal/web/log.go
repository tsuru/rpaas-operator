package web

import (
	"strconv"

	"github.com/labstack/echo/v4"
	"github.com/tsuru/rpaas-operator/internal/pkg/rpaas"
)

func extractLogArgs(c echo.Context) rpaas.LogArgs {
	var pLines *int64
	lines, err := strconv.ParseInt(c.FormValue("lines"), 10, 64)
	if err == nil && lines > 0 {
		pLines = &lines
	}
	var pSince *int64
	since, err := strconv.ParseInt(c.FormValue("since"), 10, 64)
	if err == nil && since > 0 {
		pSince = &since
	}
	follow, _ := strconv.ParseBool(c.FormValue("follow"))
	withTimestamp, _ := strconv.ParseBool(c.FormValue("timestamp"))

	args := rpaas.LogArgs{
		Pod:           c.FormValue("pod"),
		Container:     c.FormValue("container"),
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
