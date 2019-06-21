package api

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/labstack/echo"
	"github.com/tsuru/rpaas-operator/rpaas"
)

func serviceCreate(c echo.Context) error {
	var args rpaas.CreateArgs
	err := c.Bind(&args)
	if err != nil {
		return err
	}
	manager, err := getManager(c)
	if err != nil {
		return err
	}
	err = manager.CreateInstance(c.Request().Context(), args)
	if err != nil {
		return err
	}
	return c.NoContent(http.StatusCreated)
}

func serviceDelete(c echo.Context) error {
	name := c.Param("instance")
	if len(name) == 0 {
		return c.String(http.StatusBadRequest, "name is required")
	}
	manager, err := getManager(c)
	if err != nil {
		return err
	}
	err = manager.DeleteInstance(c.Request().Context(), name)
	if err != nil {
		return err
	}
	return c.NoContent(http.StatusOK)
}

type plan struct {
	Name        string `json:"name"`
	Description string `json:"description"`
}

func servicePlans(c echo.Context) error {
	manager, err := getManager(c)
	if err != nil {
		return err
	}
	plans, err := manager.GetPlans(c.Request().Context())
	if err != nil {
		return err
	}
	result := make([]plan, len(plans))
	for i, p := range plans {
		result[i] = plan{
			Name:        p.Name,
			Description: "no plan description",
		}
	}
	return c.JSON(http.StatusOK, result)
}

func serviceInfo(c echo.Context) error {
	manager, err := getManager(c)
	if err != nil {
		return err
	}
	instanceName := c.Param("instance")
	instance, err := manager.GetInstance(c.Request().Context(), instanceName)
	if err != nil {
		return err
	}
	replicas := "0"
	if instance.Spec.Replicas != nil {
		replicas = fmt.Sprintf("%d", *instance.Spec.Replicas)
	}
	routes := make([]string, len(instance.Spec.Locations))
	for i, loc := range instance.Spec.Locations {
		routes[i] = loc.Config.Value
	}
	address, err := manager.GetInstanceAddress(c.Request().Context(), instanceName)
	if err != nil {
		return err
	}
	if address == "" {
		address = "pending"
	}
	ret := []map[string]string{
		{
			"label": "Address",
			"value": address,
		},
		{
			"label": "Instances",
			"value": replicas,
		},
		{
			"label": "Routes",
			"value": strings.Join(routes, "\n"),
		},
	}
	return c.JSON(http.StatusOK, ret)
}

func serviceBindApp(c echo.Context) error {
	manager, err := getManager(c)
	if err != nil {
		return err
	}

	var args rpaas.BindAppArgs
	if err = c.Bind(&args); err != nil {
		return err
	}

	if err = manager.BindApp(c.Request().Context(), c.Param("instance"), args); err != nil {
		return err
	}

	return c.NoContent(http.StatusCreated)
}

func serviceUnbindApp(c echo.Context) error {
	manager, err := getManager(c)
	if err != nil {
		return err
	}

	if err = manager.UnbindApp(c.Request().Context(), c.Param("instance")); err != nil {
		return err
	}

	return c.NoContent(http.StatusOK)
}
