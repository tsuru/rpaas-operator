// Copyright 2019 tsuru authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package api

import (
	"fmt"
	"net/http"

	"github.com/labstack/echo/v4"
	"github.com/tsuru/rpaas-operator/internal/pkg/rpaas"
)

func cachePurge(c echo.Context) error {
	if c.Request().ContentLength == 0 {
		return echo.NewHTTPError(http.StatusBadRequest, "Request body can't be empty")
	}

	var args rpaas.PurgeCacheArgs
	err := c.Bind(&args)
	if err != nil {
		return err
	}
	name := c.Param("instance")
	if len(name) == 0 {
		return c.String(http.StatusBadRequest, "instance is required")
	}
	manager, err := getManager(c)
	if err != nil {
		return err
	}
	count, err := manager.PurgeCache(c.Request().Context(), name, args)
	if err != nil {
		return err
	}
	return c.String(http.StatusOK, fmt.Sprintf("Object purged on %d servers", count))
}
