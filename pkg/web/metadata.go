// Copyright 2024 tsuru authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package web

import (
	"net/http"

	"github.com/labstack/echo/v4"

	clientTypes "github.com/tsuru/rpaas-operator/pkg/rpaas/client/types"
)

func getMetadata(c echo.Context) error {
	ctx := c.Request().Context()
	manager, err := getManager(ctx)
	if err != nil {
		return err
	}

	metadata, err := manager.GetMetadata(ctx, c.Param("instance"))
	if err != nil {
		return err
	}

	return c.JSON(http.StatusOK, metadata)
}

func setMetadata(c echo.Context) error {
	ctx := c.Request().Context()
	manager, err := getManager(ctx)
	if err != nil {
		return err
	}

	var metadata clientTypes.Metadata
	if err = c.Bind(&metadata); err != nil {
		return err
	}

	err = manager.SetMetadata(ctx, c.Param("instance"), &metadata)
	if err != nil {
		return err
	}

	return c.NoContent(http.StatusOK)
}

func unsetMetadata(c echo.Context) error {
	ctx := c.Request().Context()
	manager, err := getManager(ctx)
	if err != nil {
		return err
	}

	var metadata clientTypes.Metadata
	if err = c.Bind(&metadata); err != nil {
		return err
	}

	err = manager.UnsetMetadata(ctx, c.Param("instance"), &metadata)
	if err != nil {
		return err
	}

	return c.NoContent(http.StatusOK)
}
