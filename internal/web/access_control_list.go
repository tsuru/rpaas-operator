// Copyright 2021 tsuru authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package web

import (
	"net/http"
	"strconv"

	"github.com/labstack/echo/v4"
	"github.com/tsuru/rpaas-operator/api/v1alpha1"
)

func getAccessControlList(c echo.Context) error {
	ctx := c.Request().Context()
	manager, err := getManager(ctx)
	if err != nil {
		return err
	}

	instance := c.Param("instance")

	upstreams, err := manager.GetAccessControlList(ctx, instance)
	if err != nil {
		return err
	}

	return c.JSON(http.StatusOK, upstreams.Spec.Items)
}

func addAccessControlList(c echo.Context) error {
	ctx := c.Request().Context()
	manager, err := getManager(ctx)
	if err != nil {
		return err
	}

	instance := c.Param("instance")

	u := v1alpha1.RpaasAccessControlListItem{}
	err = c.Bind(&u)
	if err != nil {
		return err
	}

	err = manager.AddAccessControlList(ctx, instance, u)
	if err != nil {
		return err
	}

	return c.NoContent(http.StatusCreated)
}

func deleteAccessControlList(c echo.Context) error {
	ctx := c.Request().Context()
	manager, err := getManager(ctx)
	if err != nil {
		return err
	}

	instance := c.Param("instance")

	host := c.QueryParam("host")
	portString := c.QueryParam("port")
	port := 0
	if portString != "" {
		port, err = strconv.Atoi(portString)
		if err != nil {
			return err
		}
	}

	if err := manager.DeleteAccessControlList(ctx, instance, host, port); err != nil {
		return err
	}

	return c.NoContent(http.StatusNoContent)
}
