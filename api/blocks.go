package api

import (
	"net/http"

	"github.com/labstack/echo"
	"github.com/tsuru/rpaas-operator/rpaas"
)

func deleteBlock(c echo.Context) error {
	manager, err := getManager(c)
	if err != nil {
		return err
	}

	err = manager.DeleteBlock(c.Request().Context(), c.Param("instance"), c.Param("block"))
	if err != nil {
		return err
	}

	return c.NoContent(http.StatusOK)
}

func listBlocks(c echo.Context) error {
	manager, err := getManager(c)
	if err != nil {
		return err
	}

	blocks, err := manager.ListBlocks(c.Request().Context(), c.Param("instance"))
	if err != nil {
		return err
	}

	if blocks == nil {
		blocks = make([]rpaas.ConfigurationBlock, 0)
	}

	return c.JSON(http.StatusOK, struct {
		Blocks []rpaas.ConfigurationBlock `json:"blocks"`
	}{blocks})
}

func updateBlock(c echo.Context) error {
	manager, err := getManager(c)
	if err != nil {
		return err
	}

	var block rpaas.ConfigurationBlock
	if err = c.Bind(&block); err != nil {
		return err
	}

	err = manager.UpdateBlock(c.Request().Context(), c.Param("instance"), block)
	if err != nil {
		return err
	}

	return c.NoContent(http.StatusOK)
}
