// Copyright 2024 tsuru authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package client

import (
	"context"
	"fmt"
	"net/http"
)

func (c *client) Stop(ctx context.Context, instance string) error {
	if instance == "" {
		return ErrMissingInstance
	}

	pathName := fmt.Sprintf("/resources/%s/stop", instance)
	req, err := c.newRequest("POST", pathName, nil, instance)
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
