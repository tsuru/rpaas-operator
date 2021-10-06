// Copyright 2021 tsuru authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package client

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"time"
)

func (args LogArgs) Validate() error {
	if args.Instance == "" {
		return ErrMissingInstance
	}

	return nil
}

func (c *client) Log(ctx context.Context, args LogArgs) error {
	if err := args.Validate(); err != nil {
		return err
	}

	httpClient := *c.client
	httpClient.Timeout = time.Duration(0)

	serverAddress := c.formatURL(fmt.Sprintf("/resources/%s/log", args.Instance), args.Instance)
	u, err := url.Parse(serverAddress)
	if err != nil {
		return err
	}

	qs := u.Query()
	qs.Set("color", strconv.FormatBool(args.Color))
	qs.Set("follow", strconv.FormatBool(args.Follow))
	if args.Lines > 0 {
		qs.Set("lines", strconv.FormatInt(int64(args.Lines), 10))
	}
	if args.Since > 0 {
		sinceSeconds := int64(args.Since.Seconds())
		qs.Set("since", strconv.FormatInt(sinceSeconds, 10))
	}
	if args.Pod != "" {
		qs.Set("pod", args.Pod)
	}
	if args.Container != "" {
		qs.Set("container", args.Container)
	}
	u.RawQuery = qs.Encode()

	req, err := http.NewRequestWithContext(ctx, "GET", u.String(), nil)
	if err != nil {
		return err
	}
	c.baseAuthHeader(req.Header)

	resp, err := httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if _, err = io.Copy(args.Out, resp.Body); err != io.EOF {
		return err
	}

	return nil
}
