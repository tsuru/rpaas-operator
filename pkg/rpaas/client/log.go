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
	values := url.Values{
		"follow": []string{strconv.FormatBool(args.Follow)},
	}
	pathName := fmt.Sprintf("/resources/%s/log?%s", args.Instance, values.Encode())
	req, err := c.newRequest("GET", pathName, nil, args.Instance)
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := c.do(ctx, req)
	if err != nil {
		return err
	}

	return writeOut(resp.Body)
}
