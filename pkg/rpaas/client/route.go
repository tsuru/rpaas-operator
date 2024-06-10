// Copyright 2021 tsuru authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package client

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"github.com/ajg/form"

	"github.com/tsuru/rpaas-operator/pkg/rpaas/client/types"
)

func (args DeleteRouteArgs) Validate() error {
	if args.Instance == "" {
		return ErrMissingInstance
	}

	if args.Path == "" {
		return ErrMissingPath
	}

	return nil
}

func (c *client) DeleteRoute(ctx context.Context, args DeleteRouteArgs) error {
	if err := args.Validate(); err != nil {
		return err
	}

	pathName := fmt.Sprintf("/resources/%s/route", args.Instance)
	values := url.Values{}
	values.Set("path", args.Path)
	body := strings.NewReader(values.Encode())
	req, err := c.newRequest("DELETE", pathName, body)
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	response, err := c.do(ctx, req)
	if err != nil {
		return err
	}

	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return newErrUnexpectedStatusCodeFromResponse(response)
	}

	return nil
}

func (args ListRoutesArgs) Validate() error {
	if args.Instance == "" {
		return ErrMissingInstance
	}

	return nil
}

func (c *client) ListRoutes(ctx context.Context, args ListRoutesArgs) ([]types.Route, error) {
	if err := args.Validate(); err != nil {
		return nil, err
	}

	pathName := fmt.Sprintf("/resources/%s/route", args.Instance)
	req, err := c.newRequest("GET", pathName, nil)
	if err != nil {
		return nil, err
	}

	response, err := c.do(ctx, req)
	if err != nil {
		return nil, err
	}

	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return nil, newErrUnexpectedStatusCodeFromResponse(response)
	}

	var routes struct {
		Routes []types.Route `json:"paths"`
	}
	if err = unmarshalBody(response, &routes); err != nil {
		return nil, err
	}

	return routes.Routes, nil
}

func (args UpdateRouteArgs) Validate() error {
	if args.Instance == "" {
		return ErrMissingInstance
	}

	if args.Path == "" {
		return ErrMissingPath
	}

	return nil
}

func (c *client) UpdateRoute(ctx context.Context, args UpdateRouteArgs) error {
	if err := args.Validate(); err != nil {
		return err
	}

	values := types.Route{
		Path:        args.Path,
		Destination: args.Destination,
		HTTPSOnly:   args.HTTPSOnly,
		Content:     args.Content,
	}

	b, err := form.EncodeToString(values)
	if err != nil {
		return err
	}
	body := strings.NewReader(b)

	pathName := fmt.Sprintf("/resources/%s/route", args.Instance)
	req, err := c.newRequest("POST", pathName, body)
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	response, err := c.do(ctx, req)
	if err != nil {
		return err
	}

	defer response.Body.Close()

	if response.StatusCode != http.StatusCreated {
		return newErrUnexpectedStatusCodeFromResponse(response)
	}

	return nil
}
