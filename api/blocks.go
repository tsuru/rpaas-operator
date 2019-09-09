// Copyright 2019 tsuru authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package api

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/labstack/echo/v4"
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

func deleteLuaBlock(c echo.Context) error {
	manager, err := getManager(c)
	if err != nil {
		return err
	}

	luaBlockType, err := formValue(c.Request(), "lua_module_type")
	if err != nil {
		return err
	}

	err = manager.DeleteBlock(c.Request().Context(), c.Param("instance"), luaBlockName(luaBlockType))
	if err != nil {
		return err
	}

	return c.NoContent(http.StatusOK)
}

func listLuaBlocks(c echo.Context) error {
	manager, err := getManager(c)
	if err != nil {
		return err
	}

	blocks, err := manager.ListBlocks(c.Request().Context(), c.Param("instance"))
	if err != nil {
		return err
	}

	type luaBlock struct {
		LuaName string `json:"lua_name"`
		Content string `json:"content"`
	}

	luaBlocks := []luaBlock{}
	for _, block := range blocks {
		if strings.HasPrefix(block.Name, luaBlockName("")) {
			luaBlocks = append(luaBlocks, luaBlock{
				LuaName: block.Name,
				Content: block.Content,
			})
		}
	}

	return c.JSON(http.StatusOK, struct {
		Modules []luaBlock `json:"modules"`
	}{Modules: luaBlocks})
}

func updateLuaBlock(c echo.Context) error {
	manager, err := getManager(c)
	if err != nil {
		return err
	}

	in := struct {
		LuaModuleType string `form:"lua_module_type"`
		Content       string `form:"content"`
	}{}
	if err = c.Bind(&in); err != nil {
		return err
	}

	block := rpaas.ConfigurationBlock{
		Name:    luaBlockName(in.LuaModuleType),
		Content: in.Content,
	}

	err = manager.UpdateBlock(c.Request().Context(), c.Param("instance"), block)
	if err != nil {
		return err
	}

	return c.NoContent(http.StatusOK)
}

func luaBlockName(blockType string) string {
	return fmt.Sprintf("lua-%s", blockType)
}
