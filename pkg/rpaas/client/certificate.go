// Copyright 2021 tsuru authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package client

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/url"
	"strings"
)

func (args UpdateCertificateArgs) Validate() error {
	if args.Instance == "" {
		return ErrMissingInstance
	}

	if args.Certificate == "" {
		return fmt.Errorf("rpaasv2: certificate cannot be empty")
	}

	if args.Key == "" {
		return fmt.Errorf("rpaasv2: key cannot be empty")
	}

	return nil
}

func (c *client) UpdateCertificate(ctx context.Context, args UpdateCertificateArgs) error {
	if err := args.Validate(); err != nil {
		return err
	}

	buffer := &bytes.Buffer{}
	w := multipart.NewWriter(buffer)

	if args.boundary != "" {
		if err := w.SetBoundary(args.boundary); err != nil {
			return err
		}
	}

	var part io.Writer
	var err error
	{
		part, err = w.CreateFormFile("cert", "cert.pem")
		if err != nil {
			return err
		}

		part.Write([]byte(args.Certificate))
	}
	{
		part, err = w.CreateFormFile("key", "key.pem")
		if err != nil {
			return err
		}

		part.Write([]byte(args.Key))
	}

	if err = w.WriteField("name", args.Name); err != nil {
		return err
	}

	if err = w.Close(); err != nil {
		return err
	}

	body := strings.NewReader(buffer.String())
	pathName := fmt.Sprintf("/resources/%s/certificate", args.Instance)
	req, err := c.newRequest("POST", pathName, body, args.Instance)
	req.Header.Set("Content-Type", fmt.Sprintf("multipart/form-data; boundary=%q", w.Boundary()))
	if err != nil {
		return err
	}

	response, err := c.do(ctx, req)
	if err != nil {
		return err
	}

	if response.StatusCode != http.StatusOK {
		return newErrUnexpectedStatusCodeFromResponse(response)
	}

	return nil
}

func (args *DeleteCertificateArgs) Validate() error {
	if args.Instance == "" {
		return ErrMissingInstance
	}

	return nil
}

func (c *client) DeleteCertificate(ctx context.Context, args DeleteCertificateArgs) error {
	if err := args.Validate(); err != nil {
		return err
	}

	args.Name = url.QueryEscape(args.Name)
	pathName := fmt.Sprintf("/resources/%s/certificate/%s", args.Instance, args.Name)
	req, err := c.newRequest("DELETE", pathName, nil, args.Instance)
	if err != nil {
		return err
	}

	response, err := c.do(ctx, req)
	if err != nil {
		return err
	}

	if response.StatusCode != http.StatusOK {
		return newErrUnexpectedStatusCodeFromResponse(response)
	}

	return nil
}
