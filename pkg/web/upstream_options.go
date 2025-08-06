// Copyright 2025 tsuru authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package web

import (
	"net/http"

	"github.com/labstack/echo/v4"

	"github.com/tsuru/rpaas-operator/internal/pkg/rpaas"
)

func getUpstreamOptions(c echo.Context) error {
	ctx := c.Request().Context()
	manager, err := getManager(ctx)
	if err != nil {
		return err
	}

	upstreamOptions, err := manager.GetUpstreamOptions(ctx, c.Param("instance"))
	if err != nil {
		return err
	}

	return c.JSON(http.StatusOK, upstreamOptions)
}

func addUpstreamOptions(c echo.Context) error {
	ctx := c.Request().Context()
	if c.Request().ContentLength == 0 {
		return echo.NewHTTPError(http.StatusBadRequest, "Request body can't be empty")
	}

	manager, err := getManager(ctx)
	if err != nil {
		return err
	}

	var args rpaas.UpstreamOptionsArgs
	if err = c.Bind(&args); err != nil {
		return err
	}

	if err = manager.AddUpstreamOptions(ctx, c.Param("instance"), args); err != nil {
		return err
	}

	return c.NoContent(http.StatusCreated)
}

func updateUpstreamOptions(c echo.Context) error {
	ctx := c.Request().Context()
	if c.Request().ContentLength == 0 {
		return echo.NewHTTPError(http.StatusBadRequest, "Request body can't be empty")
	}

	manager, err := getManager(ctx)
	if err != nil {
		return err
	}

	var args rpaas.UpstreamOptionsArgs
	if err = c.Bind(&args); err != nil {
		return err
	}

	// Set the bind from URL parameter, not from request body
	args.PrimaryBind = c.Param("bind")
	if args.PrimaryBind == "" {
		return echo.NewHTTPError(http.StatusBadRequest, "bind parameter is required")
	}

	if err = manager.UpdateUpstreamOptions(ctx, c.Param("instance"), args); err != nil {
		return err
	}

	return c.NoContent(http.StatusOK)
}

func deleteUpstreamOptions(c echo.Context) error {
	ctx := c.Request().Context()
	manager, err := getManager(ctx)
	if err != nil {
		return err
	}

	primaryBind := c.Param("bind")
	if primaryBind == "" {
		return echo.NewHTTPError(http.StatusBadRequest, "bind parameter is required")
	}

	if err = manager.DeleteUpstreamOptions(ctx, c.Param("instance"), primaryBind); err != nil {
		return err
	}

	return c.NoContent(http.StatusOK)
}
