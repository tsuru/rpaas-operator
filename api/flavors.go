// Copyright 2019 tsuru authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package api

import (
	"net/http"

	"github.com/labstack/echo/v4"
	"github.com/tsuru/rpaas-operator/internal/pkg/rpaas"
)

func getServiceFlavors(c echo.Context) error {
	manager, err := getManager(c)
	if err != nil {
		return err
	}

	flavors, err := manager.GetFlavors(c.Request().Context())
	if err != nil {
		return err
	}

	if flavors == nil {
		flavors = make([]rpaas.Flavor, 0)
	}

	return c.JSON(http.StatusOK, flavors)
}

func getInstanceFlavors(c echo.Context) error {
	return getServiceFlavors(c)
}
