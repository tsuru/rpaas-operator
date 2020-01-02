// Copyright 2019 tsuru authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package api

import (
	"io/ioutil"
	"net/http"

	"github.com/labstack/echo/v4"
)

type scaleParameters struct {
	Quantity int32 `form:"quantity"`
}

func scale(c echo.Context) error {
	var data scaleParameters
	if err := c.Bind(&data); err != nil {
		return c.String(http.StatusBadRequest, "quantity is either missing or not valid")
	}
	manager, err := getManager(c)
	if err != nil {
		return err
	}
	if err = manager.Scale(c.Request().Context(), c.Param("instance"), data.Quantity); err != nil {
		return err
	}
	return c.NoContent(http.StatusCreated)
}

func getFormFileContent(c echo.Context, key string) ([]byte, error) {
	fileHeader, err := c.FormFile(key)
	if err != nil {
		return []byte{}, err
	}
	file, err := fileHeader.Open()
	if err != nil {
		return []byte{}, err
	}
	defer file.Close()
	rawContent, err := ioutil.ReadAll(file)
	if err != nil {
		return []byte{}, err
	}
	return rawContent, nil
}

func serviceNodeStatus(c echo.Context) error {
	manager, err := getManager(c)
	if err != nil {
		return err
	}
	instance := c.Param("instance")
	_, podStatus, err := manager.GetInstanceStatus(c.Request().Context(), instance)
	if err != nil {
		return err
	}
	return c.JSON(200, podStatus)
}

func healthcheck(c echo.Context) error {
	return c.String(http.StatusOK, "OK")
}
