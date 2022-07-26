// Copyright 2019 tsuru authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package web

import (
	"net/http"

	"github.com/labstack/echo/v4"
)

type scaleParameters struct {
	Quantity int32 `form:"quantity"`
}

func scale(c echo.Context) error {
	ctx := c.Request().Context()
	var data scaleParameters
	if err := c.Bind(&data); err != nil {
		return c.String(http.StatusBadRequest, "quantity is either missing or not valid")
	}
	manager, err := getManager(ctx)
	if err != nil {
		return err
	}
	if err = manager.Scale(ctx, c.Param("instance"), data.Quantity); err != nil {
		return err
	}
	return c.NoContent(http.StatusOK)
}

func serviceNodeStatus(c echo.Context) error {
	ctx := c.Request().Context()
	manager, err := getManager(ctx)
	if err != nil {
		return err
	}
	instance := c.Param("instance")
	_, podStatus, err := manager.GetInstanceStatus(ctx, instance)
	if err != nil {
		return err
	}
	return c.JSON(200, podStatus)
}

func healthcheck(c echo.Context) error {
	return c.String(http.StatusOK, "OK")
}
