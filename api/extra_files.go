package api

import (
	"fmt"
	"io/ioutil"
	"net/http"

	"github.com/labstack/echo"
	"github.com/tsuru/rpaas-operator/rpaas"
)

func listExtraFiles(c echo.Context) error {
	// TODO:
	return nil
}

func getExtraFile(c echo.Context) error {
	// TODO:
	return nil
}

func addExtraFiles(c echo.Context) error {
	manager, err := getManager(c)
	if err != nil {
		return err
	}
	files, err := decodeMultipartFiles(c)
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
	err = manager.CreateExtraFiles(c.Request().Context(), c.Param("instance"), files...)
	if err != nil {
		return err
	}
	return c.String(http.StatusCreated, fmt.Sprintf("New %d files were added\n", len(files)))
}

func updateExtraFiles(c echo.Context) error {
	// TODO:
	return nil
}

func deleteExtraFile(c echo.Context) error {
	// TODO:
	return nil
}

func decodeMultipartFiles(c echo.Context) (files []rpaas.File, err error) {
	mf, err := c.MultipartForm()
	if err != nil {
		return
	}
	mfhs := mf.File["files"]
	files = make([]rpaas.File, len(mfhs))
	for i, fh := range mfhs {
		f, err := fh.Open()
		if err != nil {
			return nil, err
		}
		defer f.Close()
		content, err := ioutil.ReadAll(f)
		if err != nil {
			return nil, err
		}
		files[i] = rpaas.File{
			Name:    fh.Filename,
			Content: content,
		}
	}
	return
}
