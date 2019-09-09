// Copyright 2019 tsuru authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package api

import (
	"net/http"

	"github.com/labstack/echo/v4"
)

func getServiceFlavors(c echo.Context) error {
	return c.JSON(http.StatusOK, []struct {
		name        string
		description string
	}{})
}

func getInstanceFlavors(c echo.Context) error {
	return getServiceFlavors(c)
}
