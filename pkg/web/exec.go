// Copyright 2020 tsuru authors. All rights reserved.
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

func exec(c echo.Context) error {
	var wsUpgrader websocket.Upgrader = websocket.Upgrader{
		HandshakeTimeout: config.Get().WebSocketHandshakeTimeout,
		ReadBufferSize:   config.Get().WebSocketReadBufferSize,
		WriteBufferSize:  config.Get().WebSocketWriteBufferSize,
		CheckOrigin:      checkOrigin,
	}
	useWebSocket, _ := strconv.ParseBool(c.QueryParam("ws"))
	if useWebSocket {
		ws := &wsTransport{extractArgs: extractExecArgs, command: execCommandOnInstance}
		return ws.Run(c, &wsUpgrader)
	}
	http := &http2Transport{extractArgs: extractExecArgs, command: execCommandOnInstance}
	return http.Run(c)
}

func execCommandOnInstance(c echo.Context, args commonArgs) error {
	ctx := c.Request().Context()
	manager, err := getManager(ctx)
	if err != nil {
		return err
	}
	execArgs := args.(*rpaas.ExecArgs)
	return manager.Exec(ctx, c.Param("instance"), *execArgs)
}

func extractExecArgs(r url.Values) commonArgs {
	tty, _ := strconv.ParseBool(r.Get("tty"))
	interactive, _ := strconv.ParseBool(r.Get("interactive"))
	width, _ := strconv.ParseUint(r.Get("width"), 10, 16)
	height, _ := strconv.ParseUint(r.Get("height"), 10, 16)
	return &rpaas.ExecArgs{
		CommonTerminalArgs: rpaas.CommonTerminalArgs{
			Command:        r["command"],
			Pod:            r.Get("pod"),
			Container:      r.Get("container"),
			TTY:            tty,
			Interactive:    interactive,
			TerminalWidth:  uint16(width),
			TerminalHeight: uint16(height),
		},
	}
}
