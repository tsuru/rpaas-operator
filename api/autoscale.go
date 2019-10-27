package api

import (
	"net/http"

	"github.com/labstack/echo/v4"
	"github.com/tsuru/rpaas-operator/internal/pkg/rpaas"
)

func getAutoscale(c echo.Context) error {
	manager, err := getManager(c)
	if err != nil {
		return err
	}

	autoscale, err := manager.GetAutoscale(c.Request().Context(), c.Param("instance"))
	if err != nil {
		return err
	}

	if autoscale == nil {
		autoscale = &rpaas.Autoscale{}
	}

	return c.JSON(http.StatusOK, struct {
		Autoscale *rpaas.Autoscale `json:"autoscale"`
	}{autoscale})
}

func createAutoscale(c echo.Context) error {
	manager, err := getManager(c)
	if err != nil {
		return err
	}

	var autoscale rpaas.Autoscale
	if err = c.Bind(&autoscale); err != nil {
		return err
	}

	err = manager.CreateAutoscale(c.Request().Context(), c.Param("instance"), &autoscale)
	if err != nil {
		return err
	}

	return c.NoContent(http.StatusOK)
}

func updateAutoscale(c echo.Context) error {
	manager, err := getManager(c)
	if err != nil {
		return err
	}

	var autoscale rpaas.Autoscale
	if err = c.Bind(&autoscale); err != nil {
		return err
	}

	err = manager.UpdateAutoscale(c.Request().Context(), c.Param("instance"), &autoscale)
	if err != nil {
		return err
	}

	return c.NoContent(http.StatusOK)
}

func removeAutoscale(c echo.Context) error {
	manager, err := getManager(c)
	if err != nil {
		return err
	}

	err = manager.DeleteAutoscale(c.Request().Context(), c.Param("instance"))
	if err != nil {
		return err
	}

	return c.NoContent(http.StatusOK)
}
