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
	infoHelper.Replicas = instance.Spec.Replicas
	infoHelper.Plan = instance.Spec.PlanName
	infoHelper.Locations = instance.Spec.Locations
	infoHelper.Service = instance.Spec.Service
	_, err = manager.GetInstanceAddress(c.Request().Context(), instanceName)
	if err != nil {
		return err
	}
	// infoHelper.Address.Ip = ipAddress

	return c.JSON(http.StatusOK, infoHelper)
}
