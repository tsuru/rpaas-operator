// Copyright 2020 tsuru authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package api

import (
	"crypto/tls"
	"fmt"
	"net/http"

	"github.com/labstack/echo/v4"
	"github.com/tsuru/rpaas-operator/config"
	"github.com/tsuru/rpaas-operator/internal/pkg/rpaas"
)

func deleteCertificate(c echo.Context) error {
	manager, err := getManager(c)
	if err != nil {
		return err
	}

	instance := c.Param("instance")
	certName := c.Param("name")
	err = manager.DeleteCertificate(c.Request().Context(), instance, certName)
	if err != nil {
		return err
	}

	return c.NoContent(http.StatusOK)
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

func getCertificates(c echo.Context) error {
	manager, err := getManager(c)
	if err != nil {
		return err
	}

	certList, err := manager.GetCertificates(c.Request().Context(), c.Param("instance"))
	if err != nil {
		return err
	}

	if certList == nil {
		certList = make([]rpaas.CertificateData, 0)
	}

	if config.Get().SuppressPrivateKeyOnCertificatesList {
		for i := range certList {
			certList[i].Key = "*** private ***"
		}
	}

	return c.JSON(http.StatusOK, certList)
}
