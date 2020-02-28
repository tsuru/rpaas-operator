// Copyright 2020 tsuru authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package api

import (
	"net/http"

	"github.com/labstack/echo/v4"
	"github.com/tsuru/rpaas-operator/internal/pkg/rpaas"
)

func instanceInfo(c echo.Context) error {
	manager, err := getManager(c)
	if err != nil {
		return err
	}
	instanceName := c.Param("instance")
	instance, err := manager.GetInstance(c.Request().Context(), instanceName)
	if err != nil {
		return err
	}

	var infoHelper rpaas.InstanceInfo
	infoHelper.RpaasInstanceSpec = instance.Spec

	return c.JSON(http.StatusOK, infoHelper)
}
