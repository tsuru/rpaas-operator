// Copyright 2023 tsuru authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package client

import (
	"context"
	"fmt"
	"io"
	"net/url"
	"strconv"

	"github.com/gorilla/websocket"
)

func (args DebugArgs) Validate() error {
	if args.Instance == "" {
		return ErrMissingInstance
	}

	return nil
}

func (c *client) Debug(ctx context.Context, args DebugArgs) (*websocket.Conn, error) {
	if err := args.Validate(); err != nil {
		return nil, err
	}

	serverAddress := c.formatURL(fmt.Sprintf("/resources/%s/debug", args.Instance))
	u, err := url.Parse(serverAddress)
	if err != nil {
		return nil, err
	}

	if u.Scheme == "https" {
		u.Scheme = "wss"
	} else {
		u.Scheme = "ws"
	}

	qs := u.Query()
	qs.Set("ws", "true")
	qs["command"] = args.Command
	qs.Set("pod", args.Pod)
	qs.Set("container", args.Container)
	qs.Set("interactive", strconv.FormatBool(args.Interactive))
	qs.Set("image", args.Image)
	qs.Set("tty", strconv.FormatBool(args.TTY))
	qs.Set("width", strconv.FormatInt(int64(args.TerminalWidth), 10))
	qs.Set("height", strconv.FormatInt(int64(args.TerminalHeight), 10))

	u.RawQuery = qs.Encode()

	conn, _, err := c.ws.DialContext(ctx, u.String(), c.baseAuthHeader(nil))
	if err != nil {
		return nil, err
	}

	if args.In != nil {
		go io.Copy(&wsWriter{conn}, args.In)
	}

	return conn, nil
}
