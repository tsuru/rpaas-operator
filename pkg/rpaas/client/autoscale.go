// Copyright 2021 tsuru authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package client

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"

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

	if args.MaxReplicas == 0 && args.MaxReplicas == args.MinReplicas && args.MaxReplicas == args.CPU && args.MaxReplicas == args.Memory {
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

	var request *http.Request
	shouldCreate, err := c.shouldCreate(ctx, args.Instance)
	if err != nil {
		return err
	}
	if shouldCreate {
		pathName := fmt.Sprintf("/resources/%s/autoscale", args.Instance)
		values := url.Values{}
		values.Set("max", fmt.Sprint(args.MaxReplicas))
		values.Set("min", fmt.Sprint(args.MinReplicas))
		values.Set("cpu", fmt.Sprint(args.CPU))
		values.Set("memory", fmt.Sprint(args.Memory))
		body := strings.NewReader(values.Encode())
		request, err = c.newRequest("POST", pathName, body, args.Instance)
		if err != nil {
			return err
		}
		request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	} else {
		pathName := fmt.Sprintf("/resources/%s/autoscale", args.Instance)
		values := url.Values{}
		if args.MaxReplicas > 0 {
			values.Set("max", fmt.Sprint(args.MaxReplicas))
		}
		if args.MinReplicas > 0 {
			values.Set("min", fmt.Sprint(args.MinReplicas))
		}
		if args.CPU > 0 {
			values.Set("cpu", fmt.Sprint(args.CPU))
		}
		if args.Memory > 0 {
			values.Set("memory", fmt.Sprint(args.Memory))
		}
		body := strings.NewReader(values.Encode())
		request, err = c.newRequest("PATCH", pathName, body, args.Instance)
		if err != nil {
			return err
		}
		request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	}

	resp, err := c.do(ctx, request)
	if err != nil {
		return err
	}

	if resp.StatusCode != http.StatusCreated {
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
