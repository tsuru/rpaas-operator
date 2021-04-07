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

func (args GetAutoscaleArgs) Validate() error {
	if args.Instance == "" {
		return ErrMissingInstance
	}

	return nil
}

func (c *client) GetAutoscale(ctx context.Context, args GetAutoscaleArgs) (*types.Autoscale, error) {
	if err := args.Validate(); err != nil {
		return nil, err
	}

	pathName := fmt.Sprintf("/resources/%s/autoscale", args.Instance)
	req, err := c.newRequest("GET", pathName, nil, args.Instance)
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
	var spec *types.Autoscale
	err = json.NewDecoder(resp.Body).Decode(&spec)
	if err != nil {
		return nil, err
	}

	return spec, nil
}

func (args UpdateAutoscaleArgs) Validate() error {
	if args.Instance == "" {
		return ErrMissingInstance
	}

	if *args.MaxReplicas == 0 && args.MaxReplicas == args.MinReplicas {
		return ErrMissingValues
	}
	return nil
}

func (c *client) shouldCreate(ctx context.Context, instance string) (bool, error) {
	_, err := c.GetAutoscale(ctx, GetAutoscaleArgs{Instance: instance})
	if err != nil {
		if isNotFoundError(err) {
			return true, nil
		}
		return false, err
	}
	return false, nil
}

func (c *client) UpdateAutoscale(ctx context.Context, args UpdateAutoscaleArgs) error {
	if err := args.Validate(); err != nil {
		return err
	}

	values := types.Autoscale{}
	if args.MaxReplicas != nil && *args.MaxReplicas > 0 {
		values.MaxReplicas = args.MaxReplicas
	}
	if args.MinReplicas != nil && *args.MinReplicas > 0 {
		values.MinReplicas = args.MinReplicas
	}
	if args.CPU != nil && *args.CPU > 0 {
		values.CPU = args.CPU
	}
	if args.Memory != nil && *args.Memory > 0 {
		values.Memory = args.Memory
	}

	shouldCreate, err := c.shouldCreate(ctx, args.Instance)
	if err != nil {
		return err
	}

	b, err := json.Marshal(values)
	if err != nil {
		return err
	}
	body := bytes.NewReader(b)

	var request *http.Request
	pathName := fmt.Sprintf("/resources/%s/autoscale", args.Instance)
	if shouldCreate {
		request, err = c.newRequest("POST", pathName, body, args.Instance)
	} else {
		request, err = c.newRequest("PATCH", pathName, body, args.Instance)
	}
	if err != nil {
		return err
	}
	request.Header.Set("Content-Type", "application/json; charset=UTF-8")

	resp, err := c.do(ctx, request)
	if err != nil {
		return err
	}

	expectedStatus := http.StatusCreated
	if shouldCreate {
		expectedStatus = http.StatusOK
	}

	if resp.StatusCode != expectedStatus {
		return newErrUnexpectedStatusCodeFromResponse(resp)
	}

	return nil
}

func (args RemoveAutoscaleArgs) Validate() error {
	if args.Instance == "" {
		return ErrMissingInstance
	}
	return nil
}

func (c *client) RemoveAutoscale(ctx context.Context, args RemoveAutoscaleArgs) error {
	if err := args.Validate(); err != nil {
		return err
	}

	pathName := fmt.Sprintf("/resources/%s/autoscale", args.Instance)
	req, err := c.newRequest("DELETE", pathName, nil, args.Instance)
	if err != nil {
		return err
	}

	resp, err := c.do(ctx, req)
	if err != nil {
		return err
	}

	if resp.StatusCode != http.StatusOK {
		return newErrUnexpectedStatusCodeFromResponse(resp)
	}

	return nil
}
