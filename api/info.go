// Copyright 2020 tsuru authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package api

import (
	"net/http"

	"github.com/labstack/echo/v4"
	"github.com/tsuru/rpaas-operator/internal/pkg/rpaas"
	"github.com/tsuru/rpaas-operator/pkg/apis/extensions/v1alpha1"
)

type infoPayload struct {
	instance     *v1alpha1.RpaasInstance
	instanceName string
	ctx          echo.Context
	manager      rpaas.RpaasManager
}

func buildInfo(payload infoPayload) (*rpaas.InfoBuilder, error) {
	infoHelper := rpaas.NewInfoInstance(payload.instance)

	err := infoHelper.SetAddress(payload.ctx.Request().Context(), payload.manager, payload.instanceName)
	if err != nil {
		return infoHelper, err
	}

	err = infoHelper.SetTeam(payload.instance)
	if err != nil {
		return infoHelper, err
	}

	return infoHelper, nil
}

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

	payload := infoPayload{
		instance:     instance,
		instanceName: instanceName,
		ctx:          c,
		manager:      manager,
	}

	info, err := buildInfo(payload)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, info)
	}

	return c.JSON(http.StatusOK, info)
}
