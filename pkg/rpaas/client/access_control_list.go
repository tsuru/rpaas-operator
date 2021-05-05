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

	"github.com/tsuru/rpaas-operator/pkg/rpaas/client/types"
)

func (c *client) AddAccessControlList(ctx context.Context, instance, host string, port int) error {
	if instance == "" {
		return ErrMissingInstance
	}

	values := types.AllowedUpstream{
		Host: host,
		Port: port,
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

	if response.StatusCode != http.StatusCreated {
		return newErrUnexpectedStatusCodeFromResponse(response)
	}

	return nil
}

func (c *client) RemoveAccessControlList(ctx context.Context, instance, host string, port int) error {
	if instance == "" {
		return ErrMissingInstance
	}

	values := types.AllowedUpstream{
		Host: host,
		Port: port,
	}

	b, err := json.Marshal(values)
	if err != nil {
		return err
	}
	body := bytes.NewReader(b)

	pathName := fmt.Sprintf("/resources/%s/acl", instance)
	req, err := c.newRequest("DELETE", pathName, body, instance)
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")

	response, err := c.do(ctx, req)
	if err != nil {
		return err
	}

	if response.StatusCode != http.StatusNoContent {
		return newErrUnexpectedStatusCodeFromResponse(response)
	}

	return nil
}

func (c *client) ListAccessControlList(ctx context.Context, instance string) ([]types.AllowedUpstream, error) {
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
	var acls []types.AllowedUpstream
	err = json.NewDecoder(resp.Body).Decode(&acls)
	if err != nil {
		return nil, err
	}

	return acls, nil
}
