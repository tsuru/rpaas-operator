// Copyright 2020 tsuru authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package api

import (
	"io"
	"net/http"
	"net/url"
	"strconv"
	"time"

	"github.com/gorilla/websocket"
	"github.com/labstack/echo/v4"
	"github.com/tsuru/rpaas-operator/config"
	"github.com/tsuru/rpaas-operator/internal/pkg/rpaas"
)

func exec(c echo.Context) error {
	useWebSocket, _ := strconv.ParseBool(c.QueryParam("ws"))
	if useWebSocket {
		return wsExec(c)
	}
	return http2Exec(c)
}

var wsUpgrader websocket.Upgrader = websocket.Upgrader{
	HandshakeTimeout: config.Get().WebSocketHandshakeTimeout,
	ReadBufferSize:   config.Get().WebSocketReadBufferSize,
	WriteBufferSize:  config.Get().WebSocketWriteBufferSize,
	CheckOrigin:      checkOrigin,
}

type wsReadWriter struct {
	*websocket.Conn
}

func (r *wsReadWriter) Read(p []byte) (int, error) {
	messageType, re, err := r.NextReader()
	if err != nil {
		return 0, err
	}

	if messageType != websocket.TextMessage {
		return 0, nil
	}

	return re.Read(p)
}

func (r *wsReadWriter) Write(p []byte) (int, error) {
	return len(p), r.WriteMessage(websocket.TextMessage, p)
}

func wsExec(c echo.Context) error {
	conn, err := wsUpgrader.Upgrade(c.Response(), c.Request(), nil)
	if err != nil {
		return err
	}
	defer conn.Close()

	cfg := config.Get()

	quit := make(chan bool, 1)
	defer close(quit)

	conn.SetReadDeadline(time.Now().Add(cfg.WebSocketMaxIdleTime))

	conn.SetCloseHandler(func(code int, text string) error {
		quit <- true
		return conn.WriteControl(websocket.CloseMessage, websocket.FormatCloseMessage(code, ""), time.Now().Add(cfg.WebSocketWriteWait))
	})

	go func() {
		for {
			select {
			case <-quit:
				return

			case <-time.After(cfg.WebSocketPingInterval):
				conn.WriteControl(websocket.PingMessage, nil, time.Now().Add(cfg.WebSocketWriteWait))
			}
		}
	}()

	conn.SetPongHandler(func(s string) error {
		conn.SetReadDeadline(time.Now().Add(cfg.WebSocketMaxIdleTime))
		return nil
	})

	wsRW := &wsReadWriter{conn}
	args := extractExecArgs(c.QueryParams())
	if args.Interactive {
		args.Stdin = wsRW
	}
	args.Stdout = wsRW
	args.Stderr = wsRW
	return execCommandOnInstance(c, args)
}

func checkOrigin(r *http.Request) bool {
	origin := r.Header.Get("Origin")
	if origin == "" {
		return true
	}

	allowedOrigins := config.Get().WebSocketAllowedOrigins
	if len(allowedOrigins) == 0 {
		return true
	}

	for _, ao := range allowedOrigins {
		if ao == origin {
			return true
		}
	}
	return false
}

type http2Writer struct {
	io.Writer
}

func (c *http2Writer) Write(arr []byte) (int, error) {
	n, err := c.Writer.Write(arr)
	if err != nil {
		return n, err
	}

	if f, ok := c.Writer.(http.Flusher); ok {
		f.Flush()
	}
	return n, nil
}

func http2Exec(c echo.Context) error {
	if c.Request().ProtoMajor != 2 {
		return c.String(http.StatusHTTPVersionNotSupported, "this endpoint only works over HTTP/2")
	}

	if c.Request().Method != http.MethodPost {
		return c.String(http.StatusMethodNotAllowed, "only POST method is supported")
	}

	buffer := &http2Writer{c.Response().Writer}
	args := extractExecArgs(c.QueryParams())
	if args.Interactive {
		args.Stdin = c.Request().Body
	}
	args.Stdout, args.Stderr = buffer, buffer
	return execCommandOnInstance(c, args)
}

func execCommandOnInstance(c echo.Context, args rpaas.ExecArgs) error {
	manager, err := getManager(c)
	if err != nil {
		return err
	}
	return manager.Exec(c.Request().Context(), c.Param("instance"), args)
}

func extractExecArgs(r url.Values) rpaas.ExecArgs {
	tty, _ := strconv.ParseBool(r.Get("tty"))
	interactive, _ := strconv.ParseBool(r.Get("interactive"))
	width, _ := strconv.ParseUint(r.Get("width"), 10, 16)
	height, _ := strconv.ParseUint(r.Get("height"), 10, 16)
	return rpaas.ExecArgs{
		Command:        r["command"],
		Pod:            r.Get("pod"),
		Container:      r.Get("container"),
		TTY:            tty,
		Interactive:    interactive,
		TerminalWidth:  uint16(width),
		TerminalHeight: uint16(height),
	}
}
