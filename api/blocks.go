package api

import (
	"fmt"
	"net/http"

	"github.com/labstack/echo"
	"github.com/tsuru/rpaas-operator/rpaas"
)

type Block struct {
	Name    string `form:"block_name" json:"block_name"`
	Content string `form:"content" json:"content"`
}

func deleteBlock(c echo.Context) error {
	manager, err := getManager(c)
	if err != nil {
		return err
	}
	block := c.Param("block")
	err = manager.DeleteBlock(c.Param("instance"), block)
	switch err {
	case nil:
		return c.String(http.StatusOK, fmt.Sprintf("block %q was successfully removed", block))
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
		Blocks []Block `json:"blocks"`
	}{
		Blocks: make([]Block, len(blocks)),
	}
	var index int
	for name, content := range blocks {
		result.Blocks[index] = Block{Name: name, Content: content}
		index++
	}
	return c.JSON(http.StatusOK, result)
}

func updateBlock(c echo.Context) error {
	manager, err := getManager(c)
	if err != nil {
		return err
	}
	var data Block
	if err = c.Bind(&data); err != nil {
		return err
	}
	err = manager.UpdateBlock(c.Param("instance"), data.Name, data.Content)
	switch err {
	case nil:
		return c.NoContent(http.StatusCreated)
	case rpaas.ErrBlockInvalid:
		return c.String(http.StatusBadRequest, fmt.Sprintf("%s", err))
	default:
		return err
	}
}
