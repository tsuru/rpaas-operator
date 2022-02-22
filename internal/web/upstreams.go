// Copyright 2021 tsuru authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package web

import (
	"net/http"

	"github.com/labstack/echo/v4"
	"github.com/tsuru/rpaas-operator/api/v1alpha1"
)

func getUpstreams(c echo.Context) error {
	ctx := c.Request().Context()
	manager, err := getManager(ctx)
	if err != nil {
		return err
	}

	upstreams, err := manager.GetUpstreams(ctx, c.Param("instance"))
	if err != nil {
		return err
	}

	return c.JSON(http.StatusOK, upstreams)
}

func addUpstream(c echo.Context) error {
	ctx := c.Request().Context()
	manager, err := getManager(ctx)
	if err != nil {
		return err
	}

	var upstream v1alpha1.AllowedUpstream
	if err = c.Bind(&upstream); err != nil {
		return err
	}

	err = manager.AddUpstream(ctx, c.Param("instance"), upstream)
	if err != nil {
		return err
	}

	return c.NoContent(http.StatusCreated)
}

func deleteUpstream(c echo.Context) error {
	ctx := c.Request().Context()
	manager, err := getManager(ctx)
	if err != nil {
		return err
	}

	var upstream v1alpha1.AllowedUpstream
	if err = c.Bind(&upstream); err != nil {
		return err
	}

	if err := manager.DeleteUpstream(ctx, c.Param("instance"), upstream); err != nil {
		return err
	}

	return c.NoContent(http.StatusNoContent)
}
