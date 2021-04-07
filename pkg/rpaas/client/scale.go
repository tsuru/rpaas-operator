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
)

func (args ScaleArgs) Validate() error {
	if args.Instance == "" {
		return ErrMissingInstance
	}

	if args.Replicas < int32(0) {
		return fmt.Errorf("rpaasv2: replicas must be greater or equal than zero")
	}

	return nil
}

func (c *client) Scale(ctx context.Context, args ScaleArgs) error {
	if err := args.Validate(); err != nil {
		return err
	}

	pathName := fmt.Sprintf("/resources/%s/scale", args.Instance)
	values := url.Values{}
	values.Set("quantity", fmt.Sprint(args.Replicas))
	body := strings.NewReader(values.Encode())
	req, err := c.newRequest("POST", pathName, body, args.Instance)
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
