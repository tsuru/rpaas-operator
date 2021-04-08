// Copyright 2021 tsuru authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package client

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strconv"

	"github.com/tsuru/rpaas-operator/pkg/rpaas/client/types"
)

func (c *client) AddAccessControlList(ctx context.Context, instance, host string, port int) error {
	if instance == "" {
		return ErrMissingInstance
	}

	values := types.AllowedHost{
		Host: host,
		Port: &port,
	}

	b, err := json.Marshal(values)
	if err != nil {
		return err
	}
	body := bytes.NewReader(b)

	pathName := fmt.Sprintf("/resources/%s/acl", instance)
	req, err := c.newRequest("POST", pathName, body, instance)
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")

	response, err := c.do(ctx, req)
	if err != nil {
		return err
	}

	if response.StatusCode != http.StatusOK {
		return newErrUnexpectedStatusCodeFromResponse(response)
	}

	return nil
}

func (c *client) ListAccessControlList(ctx context.Context, instance string) (*types.AccessControlList, error) {
	if instance == "" {
		return nil, ErrMissingInstance
	}

	pathName := fmt.Sprintf("/resources/%s/acl", instance)
	req, err := c.newRequest("GET", pathName, nil, instance)
	if err != nil {
		return nil, err
	}

	resp, err := c.do(ctx, req)
	if err != nil {
		return nil, err
	}

	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, newErrUnexpectedStatusCodeFromResponse(resp)
	}

	defer resp.Body.Close()
	var spec *types.AccessControlList
	err = json.NewDecoder(resp.Body).Decode(&spec)
	if err != nil {
		return nil, err
	}

	return spec, nil
}

func (c *client) RemoveAccessControlList(ctx context.Context, instance, host string, port int) error {
	if instance == "" {
		return ErrMissingInstance
	}

	values := url.Values{}
	values.Set("host", host)
	if port != 0 {
		values.Set("port", strconv.Itoa(port))
	}

	pathName := fmt.Sprintf("/resources/%s/acl?%s", instance, values.Encode())
	req, err := c.newRequest("DELETE", pathName, nil, instance)
	if err != nil {
		return err
	}

	response, err := c.do(ctx, req)
	if err != nil {
		return err
	}

	if response.StatusCode != http.StatusNoContent {
		return newErrUnexpectedStatusCodeFromResponse(response)
	}

	return nil
}
