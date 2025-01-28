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

func (args UpdateBlockArgs) Validate() error {
	if args.Instance == "" {
		return ErrMissingInstance
	}

	if args.Name == "" {
		return ErrMissingBlockName
	}

	if args.Content == "" {
		return fmt.Errorf("rpaasv2: content cannot be empty")
	}

	return nil
}

func (c *client) UpdateBlock(ctx context.Context, args UpdateBlockArgs) error {
	if err := args.Validate(); err != nil {
		return err
	}

	values := types.Block{
		Name:       args.Name,
		ServerName: args.ServerName,
		Content:    args.Content,
		Extend:     args.Extend,
	}

	b, err := form.EncodeToString(values)
	if err != nil {
		return err
	}
	body := strings.NewReader(b)

	pathName := fmt.Sprintf("/resources/%s/block", args.Instance)
	req, err := c.newRequest("POST", pathName, body)
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	response, err := c.do(ctx, req)
	if err != nil {
		return err
	}

	if response.StatusCode != http.StatusOK {
		return newErrUnexpectedStatusCodeFromResponse(response)
	}

	return nil
}

func (args DeleteBlockArgs) Validate() error {
	if args.Instance == "" {
		return ErrMissingInstance
	}

	if args.Name == "" {
		return ErrMissingBlockName
	}

	return nil
}

func (c *client) DeleteBlock(ctx context.Context, args DeleteBlockArgs) error {
	if err := args.Validate(); err != nil {
		return err
	}

	pathName := fmt.Sprintf("/resources/%s/block/%s", args.Instance, args.Name)
	if args.ServerName != "" {
		qs := url.Values{}
		qs.Add("server_name", args.ServerName)
		pathName = fmt.Sprintf("%s?%s", pathName, qs.Encode())
	}

	req, err := c.newRequest("DELETE", pathName, nil)
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

func (args ListBlocksArgs) Validate() error {
	if args.Instance == "" {
		return ErrMissingInstance
	}

	return nil
}

func (c *client) ListBlocks(ctx context.Context, args ListBlocksArgs) ([]types.Block, error) {
	if err := args.Validate(); err != nil {
		return nil, err
	}

	pathName := fmt.Sprintf("/resources/%s/block", args.Instance)
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

	var blockList struct {
		Blocks []types.Block `json:"blocks"`
	}
	if err = unmarshalBody(response, &blockList); err != nil {
		return nil, err
	}

	return blockList.Blocks, nil
}
