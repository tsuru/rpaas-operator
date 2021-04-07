// Copyright 2021 tsuru authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package web

import (
	"net/http"

	"github.com/labstack/echo/v4"
	"github.com/tsuru/rpaas-operator/api/v1alpha1"
)

func getAllowedUpstreams(c echo.Context) error {
	return nil
}

func addAllowedUpstream(c echo.Context) error {
	ctx := c.Request().Context()
	manager, err := getManager(ctx)
	if err != nil {
		return err
	}

	instance := c.Param("instance")

	u := v1alpha1.RpaasAllowedUpstream{}
	err = c.Bind(&u)
	if err != nil {
		return err
	}

	err = manager.AddAllowedUpstream(ctx, instance, u)
	if err != nil {
		return err
	}

	return c.NoContent(http.StatusCreated)
}

func deleteAllowedUpstream(c echo.Context) error {
	return nil
}
