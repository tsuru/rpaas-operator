// Copyright 2020 tsuru authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package web

import (
	"crypto/tls"
	"errors"
	"fmt"
	"io"
	"mime"
	"net/http"
	"net/url"

	"github.com/labstack/echo/v4"

	"github.com/tsuru/rpaas-operator/internal/pkg/rpaas"
	"github.com/tsuru/rpaas-operator/pkg/rpaas/client/types"
)

func deleteCertificate(c echo.Context) error {
	ctx := c.Request().Context()
	manager, err := getManager(ctx)
	if err != nil {
		return err
	}

	instance := c.Param("instance")
	certName, err := url.QueryUnescape(c.Param("name"))
	if err != nil {
		return err
	}
	err = manager.DeleteCertificate(ctx, instance, certName)
	if err != nil {
		return err
	}

	return c.NoContent(http.StatusOK)
}

func updateCertificate(c echo.Context) error {
	ctx := c.Request().Context()

	rawCertificate, err := getValueFromFormOrMultipart(c.Request(), "cert")
	if err != nil {
		return &rpaas.ValidationError{Msg: "cannot read the certificate from request", Internal: err}
	}

	rawKey, err := getValueFromFormOrMultipart(c.Request(), "key")
	if err != nil {
		return &rpaas.ValidationError{Msg: "cannot read the key from request", Internal: err}
	}

	certificate, err := tls.X509KeyPair(rawCertificate, rawKey)
	if err != nil {
		return &rpaas.ValidationError{Msg: "could not load the given certificate and key", Internal: err}
	}

	manager, err := getManager(ctx)
	if err != nil {
		return err
	}

	err = manager.UpdateCertificate(ctx, c.Param("instance"), c.FormValue("name"), certificate)
	if err != nil {
		return err
	}

	return c.NoContent(http.StatusOK)
}

func listCertManagerRequests(c echo.Context) error {
	ctx := c.Request().Context()

	manager, err := getManager(ctx)
	if err != nil {
		return err
	}

	requests, err := manager.GetCertManagerRequests(ctx, c.Param("instance"))
	if err != nil {
		return err
	}

	if requests == nil {
		requests = make([]types.CertManager, 0)
	}

	return c.JSON(http.StatusOK, requests)
}

func updateCertManagerRequest(c echo.Context) error {
	ctx := c.Request().Context()

	manager, err := getManager(ctx)
	if err != nil {
		return err
	}

	var in types.CertManager
	if err = c.Bind(&in); err != nil {
		return err
	}

	err = validateCertManagerRequest(in)
	if err != nil {
		return err
	}

	err = manager.UpdateCertManagerRequest(ctx, c.Param("instance"), in)
	if err != nil {
		return err
	}

	return c.NoContent(http.StatusOK)
}

func validateCertManagerRequest(in types.CertManager) error {
	// Set the limits
	const maxCommonNameLength = 64
	const domainSuffixLength = 37                                     // Largest suffix identified
	maxInstanceNameLength := maxCommonNameLength - domainSuffixLength // 64 - 37 = 27

	// Instance name validation
	if len(in.Name) > maxInstanceNameLength {
		return &rpaas.ValidationError{
			Msg: fmt.Sprintf(
				"The certificate name '%s' exceeds the limit of %d characters.",
				in.Name, maxInstanceNameLength,
			),
			Internal: nil,
		}
	}

	// Validation of the in.DNSNames array
	for _, dns := range in.DNSNames {
		if len(dns) > maxCommonNameLength {
			return &rpaas.ValidationError{
				Msg: fmt.Sprintf(
					"The DNS name '%s' exceeds the limit of %d characters.",
					dns, maxCommonNameLength,
				),
				Internal: nil,
			}
		}
		if dns == "" {
			return &rpaas.ValidationError{
				Msg:      "DNS names cannot contain empty values.",
				Internal: nil,
			}
		}
	}

	return nil
}

func deleteCertManagerRequest(c echo.Context) error {
	ctx := c.Request().Context()

	manager, err := getManager(ctx)
	if err != nil {
		return err
	}

	instanceName := c.Param("instance")
	name := c.QueryParam("name")
	issuer := c.QueryParam("issuer")

	if name != "" {
		if err := manager.DeleteCertManagerRequestByName(ctx, instanceName, name); err != nil {
			return err
		}
	} else {
		if err := manager.DeleteCertManagerRequestByIssuer(ctx, instanceName, issuer); err != nil {
			return err
		}
	}

	return c.NoContent(http.StatusOK)
}

func getValueFromFormOrMultipart(r *http.Request, key string) ([]byte, error) {
	ct, _, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
	if err != nil {
		return nil, fmt.Errorf("failed to parse content-type header: %w", err)
	}

	switch ct {
	case "application/x-www-form-urlencoded":
		if value := r.FormValue(key); len(value) > 0 {
			return []byte(value), nil
		}

		return nil, errors.New("http: no such field")

	case "multipart/form-data":
		f, _, err := r.FormFile(key)
		if err != nil {
			return nil, err
		}
		defer f.Close()

		return io.ReadAll(f)
	}

	return nil, nil
}
