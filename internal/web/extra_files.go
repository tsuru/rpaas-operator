// Copyright 2019 tsuru authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package web

import (
	"fmt"
	"io/ioutil"
	"mime/multipart"
	"net/http"
	"net/url"

	"github.com/labstack/echo/v4"

	"github.com/tsuru/rpaas-operator/internal/pkg/rpaas"
)

func listExtraFiles(c echo.Context) error {
	ctx := c.Request().Context()
	manager, err := getManager(ctx)
	if err != nil {
		return err
	}
	files, err := manager.GetExtraFiles(ctx, c.Param("instance"))
	if err != nil {
		return err
	}
	names := make([]string, len(files))
	for i, file := range files {
		names[i] = file.Name
	}
	return c.JSON(http.StatusOK, names)
}

func getExtraFile(c echo.Context) error {
	ctx := c.Request().Context()
	manager, err := getManager(ctx)
	if err != nil {
		return err
	}
	files, err := manager.GetExtraFiles(ctx, c.Param("instance"))
	if err != nil {
		return err
	}
	filename, err := url.PathUnescape(c.Param("name"))
	if err != nil {
		return &echo.HTTPError{
			Code:     http.StatusBadRequest,
			Message:  fmt.Sprintf("%s", err),
			Internal: err,
		}
	}
	for _, file := range files {
		if file.Name == filename {
			return c.JSON(http.StatusOK, file)
		}
	}
	return &rpaas.NotFoundError{Msg: fmt.Sprintf("file %q not found", filename)}
}

func addExtraFiles(c echo.Context) error {
	ctx := c.Request().Context()
	manager, err := getManager(ctx)
	if err != nil {
		return err
	}
	files, err := getFiles(c)
	if err != nil {
		return &echo.HTTPError{
			Code:     http.StatusBadRequest,
			Message:  "multipart form files is not valid",
			Internal: err,
		}
	}
	if len(files) == 0 {
		return &echo.HTTPError{
			Code:    http.StatusBadRequest,
			Message: "files form field is required",
		}
	}
	err = manager.CreateExtraFiles(ctx, c.Param("instance"), files...)
	if err != nil {
		return err
	}
	return c.String(http.StatusCreated, fmt.Sprintf("New %d files were added\n", len(files)))
}

func updateExtraFiles(c echo.Context) error {
	ctx := c.Request().Context()
	manager, err := getManager(ctx)
	if err != nil {
		return err
	}
	files, err := getFiles(c)
	if err != nil {
		return &echo.HTTPError{
			Code:     http.StatusBadRequest,
			Message:  "multipart form files is not valid",
			Internal: err,
		}
	}
	if len(files) == 0 {
		return &echo.HTTPError{
			Code:    http.StatusBadRequest,
			Message: "files form field is required",
		}
	}
	err = manager.UpdateExtraFiles(ctx, c.Param("instance"), files...)
	if err != nil {
		return err
	}
	return c.String(http.StatusOK, fmt.Sprintf("%d files were successfully updated\n", len(files)))
}

func deleteExtraFile(c echo.Context) error {
	ctx := c.Request().Context()
	manager, err := getManager(ctx)
	if err != nil {
		return err
	}
	filename, err := url.PathUnescape(c.Param("name"))
	if err != nil {
		return &echo.HTTPError{
			Code:     http.StatusBadRequest,
			Message:  fmt.Sprintf("%s", err),
			Internal: err,
		}
	}
	err = manager.DeleteExtraFiles(ctx, c.Param("instance"), filename)
	if err != nil {
		return err
	}
	return c.String(http.StatusOK, fmt.Sprintf("file %q was successfully removed\n", filename))
}

// getFiles retrieves all multipart files with form name "files" and translate
// those to `rpaas.File`s.
func getFiles(c echo.Context) ([]rpaas.File, error) {
	mf, err := c.MultipartForm()
	if err != nil {
		return nil, err
	}
	fileHeaders := mf.File["files"]
	files := make([]rpaas.File, len(fileHeaders))
	for i, fh := range fileHeaders {
		file, err := newRpaasFile(fh)
		if err != nil {
			return nil, err
		}
		files[i] = file
	}
	return files, nil
}

// newRpaasFile creates a rpaas.File instance from an uploaded file part into fh.
//
// TODO(nettoclaudio): limit the fh.Size against an API max file size config.
func newRpaasFile(fh *multipart.FileHeader) (file rpaas.File, err error) {
	uploaded, err := fh.Open()
	if err != nil {
		return
	}
	defer uploaded.Close()
	rawContent, err := ioutil.ReadAll(uploaded)
	if err != nil {
		return
	}
	file.Name = fh.Filename
	file.Content = rawContent
	return
}
