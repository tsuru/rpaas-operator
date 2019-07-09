package api

import (
	"fmt"
	"io/ioutil"
	"net/http"
	"strings"

	"github.com/labstack/echo"
	"github.com/tsuru/rpaas-operator/rpaas"
)

func deleteRoute(c echo.Context) error {
	manager, err := getManager(c)
	if err != nil {
		return err
	}

	path, err := formValue(c.Request(), "path")
	if err != nil {
		return err
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
	rawBody, err := ioutil.ReadAll(req.Body)
	if err != nil {
		return "", err
	}
	defer req.Body.Close()

	if req.Header.Get("content-type") != echo.MIMEApplicationForm {
		return "", fmt.Errorf("content-type is not application form")
	}

	var value string
	for _, formValues := range strings.Split(string(rawBody), "&") {
		formValue := strings.Split(formValues, "=")
		if len(formValue) == 2 && formValue[0] == key {
			value = formValue[1]
			break
		}
	}

	return value, nil
}
