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
	qs := url.Values{}
	qs.Set("follow", strconv.FormatBool(args.Follow))
	qs.Set("timestamp", strconv.FormatBool(args.WithTimestamp))
	for _, state := range args.States {
		qs.Add("states", state)
	}
	if args.Lines > 0 {
		qs.Set("lines", strconv.FormatInt(int64(args.Lines), 10))
	}
	if args.Since > 0 {
		qs.Set("since", strconv.FormatInt(int64(args.Since), 10))
	}
	if args.Pod != "" {
		qs.Set("pod", args.Pod)
	}
	if args.Container != "" {
		qs.Set("container", args.Container)
	}

	// for some reason the echo api escapes the first & character to ?
	// the "sane" uri should be: pathName := fmt.Sprintf("/resources/%s/log?%s", args.Instance, qs.Encode())
	pathName := fmt.Sprintf("/resources/%s/log?%s", args.Instance, qs.Encode())

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
