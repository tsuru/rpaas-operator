package api

import (
	"crypto/tls"
	"fmt"
	"io/ioutil"
	"net/http"

	"github.com/labstack/echo"
)

type scaleParameters struct {
	Quantity int32 `form:"quantity"`
}

func scale(c echo.Context) error {
	var data scaleParameters
	if err := c.Bind(&data); err != nil {
		return c.String(http.StatusBadRequest, "quantity is either missing or not valid")
	}
	manager, err := getManager(c)
	if err != nil {
		return err
	}
	if err = manager.Scale(c.Request().Context(), c.Param("instance"), data.Quantity); err != nil {
		return err
	}
	return c.NoContent(http.StatusCreated)
}

func updateCertificate(c echo.Context) error {
	rawCertificate, err := getFormFileContent(c, "cert")
	if err != nil {
		if err == http.ErrMissingFile {
			return c.String(http.StatusBadRequest, "cert file is either not provided or not valid")
		}
		return err
	}
	rawKey, err := getFormFileContent(c, "key")
	if err != nil {
		if err == http.ErrMissingFile {
			return c.String(http.StatusBadRequest, "key file is either not provided or not valid")
		}
		return err
	}
	certificate, err := tls.X509KeyPair(rawCertificate, rawKey)
	if err != nil {
		return c.String(http.StatusBadRequest, fmt.Sprintf("could not load the given certicate and key: %s", err))
	}
	manager, err := getManager(c)
	if err != nil {
		return err
	}
	instance := c.Param("instance")
	certName := c.FormValue("name")
	err = manager.UpdateCertificate(c.Request().Context(), instance, certName, certificate)
	if err != nil {
		return err
	}
	return c.NoContent(http.StatusOK)
}

func getFormFileContent(c echo.Context, key string) ([]byte, error) {
	fileHeader, err := c.FormFile(key)
	if err != nil {
		return []byte{}, err
	}
	file, err := fileHeader.Open()
	if err != nil {
		return []byte{}, err
	}
	defer file.Close()
	rawContent, err := ioutil.ReadAll(file)
	if err != nil {
		return []byte{}, err
	}
	return rawContent, nil
}

func serviceStatus(c echo.Context) error {
	// TODO: retrieve rollout status
	return c.JSON(200, map[string]interface{}{
		"mock-node": map[string]interface{}{
			"status":  "successful",
			"address": "127.0.0.1",
		},
	})
}

func listExtraFiles(c echo.Context) error {
	// TODO:
	return nil
}

func getExtraFile(c echo.Context) error {
	// TODO:
	return nil
}

func addExtraFiles(c echo.Context) error {
	// TODO:
	return nil
}

func updateExtraFiles(c echo.Context) error {
	// TODO:
	return nil
}

func deleteExtraFile(c echo.Context) error {
	// TODO:
	return nil
}
