package api

import (
	"github.com/labstack/echo"
	"github.com/tsuru/rpaas-operator/rpaas"
	"net/http"
)

func cachePurge(c echo.Context) error {
	var args rpaas.PurgeCacheArgs
	err := c.Bind(&args)
	if err != nil {
		return err
	}
	name := c.Param("instance")
	if len(name) == 0 {
		return c.String(http.StatusBadRequest, "name is required")
	}
	manager, err := getManager(c)
	if err != nil {
		return err
	}
	err = manager.PurgeCache(c.Request().Context(), name, args)
	if err != nil {
		return err
	}
	return c.NoContent(http.StatusOK)
}
