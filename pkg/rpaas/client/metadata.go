// Copyright 2024 tsuru authors. All rights reserved.
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

func (c *client) GetMetadata(ctx context.Context, instance string) (*types.Metadata, error) {
	if instance == "" {
		return nil, ErrMissingInstance
	}

	pathName := fmt.Sprintf("/resources/%s/metadata", instance)
	req, err := c.newRequest("GET", pathName, nil, instance)
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

	var metadata types.Metadata
	if err = unmarshalBody(response, &metadata); err != nil {
		return nil, err
	}

	return &metadata, nil
}

func (c *client) SetMetadata(ctx context.Context, instance string, metadata *types.Metadata) error {
	if instance == "" {
		return ErrMissingInstance
	}

	b, err := json.Marshal(metadata)
	if err != nil {
		return err
	}
	body := bytes.NewReader(b)

	pathName := fmt.Sprintf("/resources/%s/metadata", instance)
	req, err := c.newRequest("POST", pathName, body, instance)
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")

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

func (c *client) UnsetMetadata(ctx context.Context, instance string, metadata *types.Metadata) error {
	if instance == "" {
		return ErrMissingInstance
	}

	b, err := json.Marshal(metadata)
	if err != nil {
		return err
	}
	body := bytes.NewReader(b)

	pathName := fmt.Sprintf("/resources/%s/metadata", instance)
	req, err := c.newRequest("DELETE", pathName, body, instance)
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
