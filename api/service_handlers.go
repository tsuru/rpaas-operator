// Copyright 2019 tsuru authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package api

import (
	"bytes"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"strings"

	"github.com/ajg/form"
	"github.com/labstack/echo/v4"
	"github.com/tsuru/rpaas-operator/internal/pkg/rpaas"
)

func serviceCreate(c echo.Context) error {
	args := rpaas.CreateArgs{
		// NOTE: using a different decoder for Parameters since the `r.PostForm()`
		// method does not understand the format used by github.com/ajf.form.
		Parameters: decodeFormParameters(c.Request()),
	}
	if err := c.Bind(&args); err != nil {
		return err
	}

	manager, err := getManager(c)
	if err != nil {
		return err
	}

	if err = manager.CreateInstance(c.Request().Context(), args); err != nil {
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
	args := rpaas.UpdateInstanceArgs{
		// NOTE: using a different decoder for Parameters since the `r.PostForm()`
		// method does not understand the format used by github.com/ajf.form.
		Parameters: decodeFormParameters(c.Request()),
	}
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

func servicePlans(c echo.Context) error {
	manager, err := getManager(c)
	if err != nil {
		return err
	}

	plans, err := manager.GetPlans(c.Request().Context())
	if err != nil {
		return err
	}

	if plans == nil {
		plans = make([]rpaas.Plan, 0)
	}

	return c.JSON(http.StatusOK, plans)
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

	if err = manager.UnbindApp(c.Request().Context(), c.Param("instance"), appName); err != nil {
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

func decodeFormParameters(r *http.Request) map[string]interface{} {
	if r == nil {
		return nil
	}

	body := r.Body
	defer body.Close()

	var buffer bytes.Buffer
	reader := io.TeeReader(body, &buffer)

	var obj struct {
		Parameters map[string]interface{} `form:"parameters"`
	}
	newFormDecoder(reader).Decode(&obj)
	r.Body = ioutil.NopCloser(&buffer)
	return obj.Parameters
}

func newFormDecoder(r io.Reader) *form.Decoder {
	decoder := form.NewDecoder(r)
	decoder.IgnoreCase(true)
	decoder.IgnoreUnknownKeys(true)
	return decoder
}
