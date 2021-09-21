// Copyright 2021 tsuru authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package client

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"net/url"
	"os"
	"strconv"
)

func writeOut(body io.ReadCloser) error {
	writer := bufio.NewWriter(os.Stdout)
	reader := bufio.NewReader(body)
	defer body.Close()
	_, err := io.Copy(writer, reader)
	if err != nil {
		return err
	}
	return nil
}

func (c *client) Log(ctx context.Context, args LogArgs) error {
	values := url.Values{}
	values.Set("follow", fmt.Sprintf("%v", args.Follow))
	values.Set("timestamp", fmt.Sprintf("%v", args.WithTimestamp))
	for _, state := range args.States {
		values.Add("states", state)
	}

	if lines := strconv.FormatInt(int64(args.Lines), 10); lines != "" {
		values.Set("lines", lines)
	}
	if since := strconv.FormatInt(int64(args.Since), 10); since != "" {
		values.Set("since", since)
	}
	if args.Pod != "" {
		values.Set("pod", args.Pod)
	}
	if args.Container != "" {
		values.Set("container", args.Container)
	}
	pathName := fmt.Sprintf("/resources/%s/log?%s", args.Instance, values.Encode())
	req, err := c.newRequest("GET", pathName, nil, args.Instance)
	if err != nil {
		return err
	}

	resp, err := c.do(ctx, req)
	if err != nil {
		return err
	}

	return writeOut(resp.Body)
}
