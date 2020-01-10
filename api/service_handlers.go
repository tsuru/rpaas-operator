// Copyright 2019 tsuru authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package api

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/labstack/echo/v4"
	"github.com/tsuru/rpaas-operator/internal/pkg/rpaas"
)

func serviceCreate(c echo.Context) error {
	if c.Request().ContentLength == 0 {
		return echo.NewHTTPError(http.StatusBadRequest, "Request body can't be empty")
	}

	var args rpaas.CreateArgs
	err := c.Bind(&args)
	if err != nil {
		return err
	}
	manager, err := getManager(c)
	if err != nil {
		return err
	}
	err = manager.CreateInstance(c.Request().Context(), args)
	if err != nil {
		return err
	}
	return c.NoContent(http.StatusCreated)
}

func serviceDelete(c echo.Context) error {
	name := c.Param("instance")
	if len(name) == 0 {
		return c.String(http.StatusBadRequest, "name is required")
	}
	manager, err := getManager(c)
	if err != nil {
		return err
	}
	err = manager.DeleteInstance(c.Request().Context(), name)
	if err != nil {
		return err
	}
	return c.NoContent(http.StatusOK)
}

func serviceUpdate(c echo.Context) error {
	if c.Request().ContentLength == 0 {
		return echo.NewHTTPError(http.StatusBadRequest, "Request body can't be empty")
	}

	var args rpaas.UpdateInstanceArgs
	if err := c.Bind(&args); err != nil {
		return err
	}

	manager, err := getManager(c)
	if err != nil {
		return err
	}

	if err = manager.UpdateInstance(c.Request().Context(), c.Param("instance"), args); err != nil {
		return err
	}

	return c.NoContent(http.StatusOK)
}

type plan struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Default     bool   `json:"default"`
}

func servicePlans(c echo.Context) error {
	manager, err := getManager(c)
	if err != nil {
		return err
	}

	plans, err := manager.GetPlans(c.Request().Context())
	if err != nil {
		return err
	}

	var result []plan
	for _, p := range plans {
		result = append(result, plan{
			Name:        p.Name,
			Description: p.Spec.Description,
			Default:     p.Spec.Default,
		})
	}

	if result == nil {
		result = []plan{}
	}

	return c.JSON(http.StatusOK, result)
}

func serviceInfo(c echo.Context) error {
	manager, err := getManager(c)
	if err != nil {
		return err
	}
	instanceName := c.Param("instance")
	instance, err := manager.GetInstance(c.Request().Context(), instanceName)
	if err != nil {
		return err
	}
	replicas := "0"
	if instance.Spec.Replicas != nil {
		replicas = fmt.Sprintf("%d", *instance.Spec.Replicas)
	}
	address, err := manager.GetInstanceAddress(c.Request().Context(), instanceName)
	if err != nil {
		return err
	}
	if address == "" {
		address = "pending"
	}
	var routes []string
	for _, location := range instance.Spec.Locations {
		routes = append(routes, location.Path)
	}
	ret := []map[string]string{
		{
			"label": "Address",
			"value": address,
		},
		{
			"label": "Instances",
			"value": replicas,
		},
		{
			"label": "Routes",
			"value": strings.Join(routes, "\n"),
		},
	}
	return c.JSON(http.StatusOK, ret)
}

func serviceBindApp(c echo.Context) error {
	if c.Request().ContentLength == 0 {
		return echo.NewHTTPError(http.StatusBadRequest, "Request body can't be empty")
	}

	manager, err := getManager(c)
	if err != nil {
		return err
	}

	var args rpaas.BindAppArgs
	if err = c.Bind(&args); err != nil {
		return err
	}

	if err = manager.BindApp(c.Request().Context(), c.Param("instance"), args); err != nil {
		return err
	}

	return c.JSON(http.StatusCreated, map[string]string{})
}

func serviceUnbindApp(c echo.Context) error {
	manager, err := getManager(c)
	if err != nil {
		return err
	}

	appName, err := formValue(c.Request(), "app-name")
	if err != nil {
		return err
	}

	if err = manager.UnbindApp(c.Request().Context(), appName, c.Param("instance")); err != nil {
		return err
	}

	return c.NoContent(http.StatusOK)
}

func serviceBindUnit(c echo.Context) error {
	return c.NoContent(http.StatusCreated)
}

func serviceUnbindUnit(c echo.Context) error {
	return c.NoContent(http.StatusOK)
}

func serviceStatus(c echo.Context) error {
	manager, err := getManager(c)
	if err != nil {
		return err
	}

	address, err := manager.GetInstanceAddress(c.Request().Context(), c.Param("instance"))
	if err != nil {
		return err
	}

	if address == "" {
		return c.NoContent(http.StatusAccepted)
	}

	return c.NoContent(http.StatusNoContent)
}
