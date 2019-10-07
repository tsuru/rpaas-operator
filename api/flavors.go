// Copyright 2019 tsuru authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package api

import (
	"net/http"
	"sort"

	"github.com/labstack/echo/v4"
	"github.com/tsuru/rpaas-operator/config"
)

type flavor struct {
	Name        string `json:"name"`
	Description string `json:"description"`
}

func getServiceFlavors(c echo.Context) error {
	flavors := make([]flavor, 0)
	conf := config.Get()
	for _, f := range conf.Flavors {
		flavors = append(flavors, flavor{
			Name:        f.Name,
			Description: f.Description,
		})
	}

	sort.SliceStable(flavors, func(i, j int) bool { return flavors[i].Name < flavors[j].Name })

	return c.JSON(http.StatusOK, flavors)
}

func getInstanceFlavors(c echo.Context) error {
	return getServiceFlavors(c)
}
