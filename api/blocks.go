package api

import (
	"fmt"
	"net/http"

	"github.com/labstack/echo"
	"github.com/tsuru/rpaas-operator/rpaas"
)

func updateBlock(c echo.Context) error {
	manager, err := getManager(c)
	if err != nil {
		return err
	}
	data := struct {
		Name    string `form:"block_name"`
		Content string `form:"content"`
	}{}
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
