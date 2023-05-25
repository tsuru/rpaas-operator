package web

import (
	"net/http"

	"github.com/labstack/echo/v4"
	"github.com/tsuru/rpaas-operator/internal/pkg/rpaas"
	clientTypes "github.com/tsuru/rpaas-operator/pkg/rpaas/client/types"
)

func getAutoscale(c echo.Context) error {
	ctx := c.Request().Context()
	manager, err := getManager(ctx)
	if err != nil {
		return err
	}

	autoscale, err := manager.GetAutoscale(ctx, c.Param("instance"))
	if err != nil {
		return err
	}

	if autoscale == nil {
		autoscale = &clientTypes.Autoscale{}
	}

	return c.JSON(http.StatusOK, autoscale)
}

func createAutoscale(c echo.Context) error {
	ctx := c.Request().Context()
	manager, err := getManager(ctx)
	if err != nil {
		return err
	}

	var autoscale clientTypes.Autoscale
	if err = c.Bind(&autoscale); err != nil {
		return err
	}

	err = manager.CreateAutoscale(ctx, c.Param("instance"), &autoscale)
	if err != nil {
		return err
	}

	return c.NoContent(http.StatusOK)
}

func updateAutoscale(c echo.Context) error {
	ctx := c.Request().Context()
	manager, err := getManager(ctx)
	if err != nil {
		return err
	}

	originalAutoscale, err := manager.GetAutoscale(ctx, c.Param("instance"))
	if err != nil {
		if serr, ok := err.(rpaas.NotFoundError); ok {
			return serr
		}
	}

	var autoscale clientTypes.Autoscale
	if err = c.Bind(&autoscale); err != nil {
		return err
	}

	if originalAutoscale != nil {
		updateValueIfNeeded(&autoscale.MaxReplicas, originalAutoscale.MaxReplicas)
		updateValueIfNeeded(&autoscale.MinReplicas, originalAutoscale.MinReplicas)
		updateValueIfNeeded(&autoscale.CPU, originalAutoscale.CPU)
		updateValueIfNeeded(&autoscale.Memory, originalAutoscale.Memory)
		updateValueIfNeeded(&autoscale.RPS, originalAutoscale.RPS)
	}

	err = manager.UpdateAutoscale(ctx, c.Param("instance"), &autoscale)
	if err != nil {
		return err
	}

	return c.NoContent(http.StatusCreated)
}

func updateValueIfNeeded(field **int32, value *int32) {
	if *field == nil && value != nil {
		*field = value
	}
}

func removeAutoscale(c echo.Context) error {
	ctx := c.Request().Context()
	manager, err := getManager(ctx)
	if err != nil {
		return err
	}

	err = manager.DeleteAutoscale(ctx, c.Param("instance"))
	if err != nil {
		return err
	}

	return c.NoContent(http.StatusOK)
}
