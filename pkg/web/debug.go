// Copyright 2023 tsuru authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package web

import (
	"net/url"
	"strconv"

	"github.com/gorilla/websocket"
	"github.com/labstack/echo/v4"

	"github.com/tsuru/rpaas-operator/internal/config"
	"github.com/tsuru/rpaas-operator/internal/pkg/rpaas"
)

func debug(c echo.Context) error {
	var wsUpgrader websocket.Upgrader = websocket.Upgrader{
		HandshakeTimeout: config.Get().WebSocketHandshakeTimeout,
		ReadBufferSize:   config.Get().WebSocketReadBufferSize,
		WriteBufferSize:  config.Get().WebSocketWriteBufferSize,
		CheckOrigin:      checkOrigin,
	}
	useWebSocket, _ := strconv.ParseBool(c.QueryParam("ws"))
	if useWebSocket {
		ws := &wsTransport{extractArgs: extractDebugArgs, command: debugCommandOnInstance}
		return ws.Run(c, &wsUpgrader)
	}
	http := &http2Transport{extractArgs: extractDebugArgs, command: debugCommandOnInstance}
	return http.Run(c)
}

func debugCommandOnInstance(c echo.Context, args commonArgs) error {
	ctx := c.Request().Context()
	manager, err := getManager(ctx)
	if err != nil {
		return err
	}
	debugArgs := args.(*rpaas.DebugArgs)
	return manager.Debug(ctx, c.Param("instance"), *debugArgs)
}

func extractDebugArgs(r url.Values) commonArgs {
	tty, _ := strconv.ParseBool(r.Get("tty"))
	interactive, _ := strconv.ParseBool(r.Get("interactive"))
	width, _ := strconv.ParseUint(r.Get("width"), 10, 16)
	height, _ := strconv.ParseUint(r.Get("height"), 10, 16)
	return &rpaas.DebugArgs{
		Image:   r.Get("image"),
		Command: r["command"],
		CommonTerminalArgs: rpaas.CommonTerminalArgs{
			Pod:            r.Get("pod"),
			Container:      r.Get("container"),
			TTY:            tty,
			Interactive:    interactive,
			TerminalWidth:  uint16(width),
			TerminalHeight: uint16(height),
		},
	}
}
