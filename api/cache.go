// Copyright 2019 tsuru authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package api

import (
	"encoding/json"
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

func cachePurgeBulk(c echo.Context) error {
	if c.Request().ContentLength == 0 {
		return echo.NewHTTPError(http.StatusBadRequest, "Request body can't be empty")
	}

	name := c.Param("instance")
	if len(name) == 0 {
		return c.String(http.StatusBadRequest, "instance is required")
	}
	manager, err := getManager(c)
	if err != nil {
		return err
	}

	var argsList []rpaas.PurgeCacheArgs
	if err = json.NewDecoder(c.Request().Body).Decode(&argsList); err != nil {
		return c.String(http.StatusBadRequest, err.Error())
	}

	status := http.StatusOK
	var results []rpaas.PurgeCacheBulkResult
	for _, args := range argsList {
		var r rpaas.PurgeCacheBulkResult
		count, err := manager.PurgeCache(c.Request().Context(), name, args)
		if err != nil {
			status = http.StatusInternalServerError
			r = rpaas.PurgeCacheBulkResult{Path: args.Path, Error: err.Error()}
		} else {
			r = rpaas.PurgeCacheBulkResult{Path: args.Path, InstancesPurged: count}
		}
		results = append(results, r)
	}

	return c.JSON(status, results)
}
