package api

import (
	"fmt"
	"net/http"

	"github.com/labstack/echo/v4"
	"github.com/tsuru/rpaas-operator/rpaas"
)

func cachePurge(c echo.Context) error {
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
