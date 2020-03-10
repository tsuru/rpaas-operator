// Copyright 2020 tsuru authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package api

import (
	"net/http"

	"github.com/labstack/echo/v4"
)

func instanceInfo(c echo.Context) error {
	manager, err := getManager(c)
	if err != nil {
		return err
	}

	info, err := manager.GetInstanceInfo(c.Request().Context(), c.Param("instance"))
	if err != nil {
		return err
	}

	return c.JSON(http.StatusOK, info)
}
