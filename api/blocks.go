package api

import (
	"fmt"
	"net/http"

	"github.com/labstack/echo"
	"github.com/tsuru/rpaas-operator/rpaas"
)

func deleteBlock(c echo.Context) error {
	manager, err := getManager(c)
	if err != nil {
		return err
	}
	blockName := c.Param("block")
	err = manager.DeleteBlock(c.Param("instance"), blockName)
	switch err {
	case nil:
		return c.String(http.StatusOK, fmt.Sprintf("block %q was successfully removed", blockName))
	case rpaas.ErrBlockInvalid:
		return c.String(http.StatusBadRequest, fmt.Sprintf("%s", err))
	case rpaas.ErrBlockIsNotDefined:
		return c.NoContent(http.StatusNoContent)
	default:
		return err
	}
}

func listBlocks(c echo.Context) error {
	manager, err := getManager(c)
	if err != nil {
		return err
	}
	blocks, err := manager.ListBlocks(c.Param("instance"))
	if err != nil {
		return err
	}
	result := struct {
		Blocks []rpaas.ConfigurationBlock `json:"blocks"`
	}{blocks}
	return c.JSON(http.StatusOK, result)
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
	err = manager.UpdateBlock(c.Param("instance"), block)
	switch err {
	case nil:
		return c.NoContent(http.StatusCreated)
	case rpaas.ErrBlockInvalid:
		return c.String(http.StatusBadRequest, fmt.Sprintf("%s", err))
	default:
		return err
	}
}
