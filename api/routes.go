// Copyright 2019 tsuru authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package api

import (
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"

	"github.com/labstack/echo/v4"
	"github.com/tsuru/rpaas-operator/rpaas"
)

func deleteRoute(c echo.Context) error {
	manager, err := getManager(c)
	if err != nil {
		return err
	}

	path, err := formValue(c.Request(), "path")
	if err != nil {
		return &rpaas.ValidationError{Msg: err.Error()}
	}

	err = manager.DeleteRoute(c.Request().Context(), c.Param("instance"), path)
	if err != nil {
		return err
	}

	return c.NoContent(http.StatusOK)
}

func getRoutes(c echo.Context) error {
	manager, err := getManager(c)
	if err != nil {
		return err
	}

	routes, err := manager.GetRoutes(c.Request().Context(), c.Param("instance"))
	if err != nil {
		return err
	}

	if routes == nil {
		routes = []rpaas.Route{}
	}

	return c.JSON(http.StatusOK, map[string]interface{}{
		"paths": routes,
	})
}

func updateRoute(c echo.Context) error {
	manager, err := getManager(c)
	if err != nil {
		return err
	}

	var route rpaas.Route
	if err = c.Bind(&route); err != nil {
		return err
	}

	err = manager.UpdateRoute(c.Request().Context(), c.Param("instance"), route)
	if err != nil {
		return err
	}

	return c.NoContent(http.StatusCreated)
}

// formValue does the same as http.Request.FormValue method and works fine on
// DELETE request as well.
func formValue(req *http.Request, key string) (string, error) {
	if req.Header.Get("content-type") != echo.MIMEApplicationForm {
		return "", fmt.Errorf("content-type is not application form")
	}

	rawBody, err := ioutil.ReadAll(req.Body)
	if err != nil {
		return "", err
	}
	defer req.Body.Close()

	if len(rawBody) == 0 {
		return "", fmt.Errorf("missing body message")
	}

	queryByKey, err := url.ParseQuery(string(rawBody))
	if err != nil {
		return "", err
	}

	values := queryByKey[key]
	if len(values) == 0 {
		return "", fmt.Errorf("missing key %q", key)
	}

	return values[0], nil
}
