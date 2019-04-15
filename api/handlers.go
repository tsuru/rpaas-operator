package api

import (
	"context"
	"crypto/tls"
	"fmt"
	"io/ioutil"
	"net/http"
	"strconv"

	"github.com/labstack/echo"
	"github.com/sirupsen/logrus"
)

func scale(c echo.Context) error {
	qty := c.FormValue("quantity")
	if len(qty) == 0 {
		return c.String(http.StatusBadRequest, "missing quantity")
	}
	intQty, err := strconv.Atoi(qty)
	if err != nil || intQty <= 0 {
		return c.String(http.StatusBadRequest, "invalid quantity: "+qty)
	}
	manager, err := getManager(c)
	if err != nil {
		return err
	}

	name := c.Param("instance")
	instance, err := manager.GetInstance(name)
	if err != nil {
		return err
	}

	int32Qty := int32(intQty)
	instance.Spec.Replicas = &int32Qty
	err = cli.Update(context.TODO(), instance)
	if err != nil {
		logrus.Error(err)
		return c.JSON(http.StatusInternalServerError, err)
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
	err = manager.UpdateCertificate(instance, certificate)
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
