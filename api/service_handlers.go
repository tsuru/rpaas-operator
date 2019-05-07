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
	err = manager.CreateInstance(args)
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
	err = manager.DeleteInstance(name)
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
	plans, err := manager.GetPlans()
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
	instance, err := manager.GetInstance(c.Param("instance"))
	if err != nil {
		return err
	}
	address := "<pending>"
	if instance.Spec.Service != nil && instance.Spec.Service.LoadBalancerIP != "" {
		address = instance.Spec.Service.LoadBalancerIP
	}
	replicas := "0"
	if instance.Spec.Replicas != nil {
		replicas = fmt.Sprintf("%d", *instance.Spec.Replicas)
	}
	routes := make([]string, len(instance.Spec.Locations))
	for i, loc := range instance.Spec.Locations {
		routes[i] = loc.Config.Value
	}
	address, err := manager.GetInstanceAddress(name)
	if err != nil {
		return err
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
	return c.NoContent(http.StatusInternalServerError)
}

func serviceUnbindApp(c echo.Context) error {
	return c.NoContent(http.StatusInternalServerError)
}
