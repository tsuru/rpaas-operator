// Copyright 2019 tsuru authors. All rights reserved.
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

func (c *client) ListUpstreamOptions(ctx context.Context, args ListUpstreamOptionsArgs) ([]types.UpstreamOptions, error) {
	if args.Instance == "" {
		return nil, ErrMissingInstance
	}

	pathName := fmt.Sprintf("/resources/%s/upstream-options", args.Instance)
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

	var upstreamOptions []types.UpstreamOptions
	if err = unmarshalBody(response, &upstreamOptions); err != nil {
		return nil, err
	}

	return upstreamOptions, nil
}

func (c *client) AddUpstreamOptions(ctx context.Context, args AddUpstreamOptionsArgs) error {
	if args.Instance == "" {
		return ErrMissingInstance
	}

	if args.PrimaryBind == "" {
		return fmt.Errorf("rpaasv2: primary bind cannot be empty")
	}

	pathName := fmt.Sprintf("/resources/%s/upstream-options", args.Instance)

	body := map[string]interface{}{
		"bind":                 args.PrimaryBind,
		"canary":               args.CanaryBinds,
		"loadBalance":          args.LoadBalance,
		"trafficShapingPolicy": args.TrafficShapingPolicy,
	}

	bodyBytes, err := json.Marshal(body)
	if err != nil {
		return err
	}

	req, err := c.newRequest("POST", pathName, bytes.NewReader(bodyBytes))
	if err != nil {
		return err
	}

	req.Header.Set("Content-Type", "application/json")

	response, err := c.do(ctx, req)
	if err != nil {
		return err
	}
	defer response.Body.Close()

	if response.StatusCode != http.StatusCreated && response.StatusCode != http.StatusOK {
		return newErrUnexpectedStatusCodeFromResponse(response)
	}

	return nil
}

func (c *client) UpdateUpstreamOptions(ctx context.Context, args UpdateUpstreamOptionsArgs) error {
	if args.Instance == "" {
		return ErrMissingInstance
	}

	if args.PrimaryBind == "" {
		return fmt.Errorf("rpaasv2: primary bind cannot be empty")
	}

	pathName := fmt.Sprintf("/resources/%s/upstream-options/%s", args.Instance, args.PrimaryBind)

	body := map[string]interface{}{
		"canary":               args.CanaryBinds,
		"loadBalance":          args.LoadBalance,
		"trafficShapingPolicy": args.TrafficShapingPolicy,
	}

	bodyBytes, err := json.Marshal(body)
	if err != nil {
		return err
	}

	req, err := c.newRequest("PUT", pathName, bytes.NewReader(bodyBytes))
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

func (c *client) DeleteUpstreamOptions(ctx context.Context, args DeleteUpstreamOptionsArgs) error {
	if args.Instance == "" {
		return ErrMissingInstance
	}

	if args.PrimaryBind == "" {
		return fmt.Errorf("rpaasv2: primary bind cannot be empty")
	}

	pathName := fmt.Sprintf("/resources/%s/upstream-options/%s", args.Instance, args.PrimaryBind)
	req, err := c.newRequest("DELETE", pathName, nil)
	if err != nil {
		return err
	}

	response, err := c.do(ctx, req)
	if err != nil {
		return err
	}
	defer response.Body.Close()

	if response.StatusCode != http.StatusOK && response.StatusCode != http.StatusNoContent {
		return newErrUnexpectedStatusCodeFromResponse(response)
	}

	return nil
}
