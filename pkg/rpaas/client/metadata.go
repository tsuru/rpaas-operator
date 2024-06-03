// Copyright 2024 tsuru authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package client

import (
	"context"
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

	// pathName := fmt.Sprintf("/resources/%s/metadata", instance)
	// req, err := c.newRequest("POST", pathName, metadata, instance)
	// if err != nil {
	// 	return err
	// }
	//
	// response, err := c.do(ctx, req)
	// if err != nil {
	// 	return err
	// }
	//
	// if response.StatusCode != http.StatusOK {
	// 	return newErrUnexpectedStatusCodeFromResponse(response)
	// }

	return nil
}

func (c *client) UnsetMetadata(ctx context.Context, instance string, metadata *types.Metadata) error {
	if instance == "" {
		return ErrMissingInstance
	}

	// pathName := fmt.Sprintf("/resources/%s/metadata", instance)
	// req, err := c.newRequest("POST", pathName, metadata, instance)
	// if err != nil {
	// 	return err
	// }
	//
	// response, err := c.do(ctx, req)
	// if err != nil {
	// 	return err
	// }
	//
	// if response.StatusCode != http.StatusOK {
	// 	return newErrUnexpectedStatusCodeFromResponse(response)
	// }

	return nil
}
