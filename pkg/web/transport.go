package web

import (
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"

	"github.com/gorilla/websocket"
	"github.com/labstack/echo/v4"

	"github.com/tsuru/rpaas-operator/internal/config"
)

type wsReadWriter struct {
	*websocket.Conn
}

func (r *wsReadWriter) Read(p []byte) (int, error) {
	messageType, re, err := r.NextReader()
	if err != nil {
		return 0, err
	}

	if messageType != websocket.TextMessage && messageType != websocket.BinaryMessage {
		return 0, nil
	}

	return re.Read(p)
}

func (r *wsReadWriter) Write(p []byte) (int, error) {
	return len(p), r.WriteMessage(websocket.TextMessage, p)
}

type commonArgs interface {
	SetStdout(io.Writer)
	SetStderr(io.Writer)
	SetStdin(io.Reader)
	GetInteractive() bool
}

type wsTransport struct {
	extractArgs func(r url.Values) commonArgs
	command     func(c echo.Context, args commonArgs) error
}

func (w *wsTransport) Run(c echo.Context, wsUpgrader *websocket.Upgrader) error {
	conn, err := wsUpgrader.Upgrade(c.Response(), c.Request(), nil)
	if err != nil {
		return err
	}

	cfg := config.Get()
	defer func() {
		code, message := websocket.CloseNormalClosure, ""
		if err != nil {
			// NOTE: logging the error here since we have no guarantees that
			// client is going to receive it.
			c.Logger().Errorf("failed to run the remote command: %v", err)
			code, message = websocket.CloseInternalServerErr, err.Error()
		}

		nerr := conn.WriteControl(websocket.CloseMessage, websocket.FormatCloseMessage(code, message), time.Now().Add(cfg.WebSocketWriteWait))
		if nerr != nil {
			c.Logger().Errorf("failed to write the close message to peer %s: %v", conn.RemoteAddr(), nerr)
			conn.Close()
		}
	}()

	quit := make(chan bool)
	defer close(quit)

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

	conn.SetReadDeadline(time.Now().Add(cfg.WebSocketMaxIdleTime))
	conn.SetPongHandler(func(s string) error {
		conn.SetReadDeadline(time.Now().Add(cfg.WebSocketMaxIdleTime))
		return nil
	})

	wsRW := &wsReadWriter{conn}
	args := w.extractArgs(c.QueryParams())
	if args.GetInteractive() {
		args.SetStdin(wsRW)
	}
	args.SetStdout(wsRW)
	args.SetStderr(wsRW)
	err = w.command(c, args)

	// NOTE: avoiding to return error since the connection has already been
	// hijacked by websocket at this point.
	//
	// See: https://github.com/labstack/echo/issues/268
	return nil
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

type http2Transport struct {
	extractArgs func(r url.Values) commonArgs
	command     func(c echo.Context, args commonArgs) error
}

func (h *http2Transport) Run(c echo.Context) error {
	if c.Request().ProtoMajor != 2 {
		return c.String(http.StatusHTTPVersionNotSupported, "this endpoint only works over HTTP/2")
	}

	if c.Request().Method != http.MethodPost {
		return c.String(http.StatusMethodNotAllowed, "only POST method is supported")
	}

	buffer := &http2Writer{c.Response().Writer}
	args := h.extractArgs(c.QueryParams())
	fmt.Printf("result struct: %+v\n", args)
	if args.GetInteractive() {
		args.SetStdin(c.Request().Body)
	}
	args.SetStdout(buffer)
	args.SetStderr(buffer)
	return h.command(c, args)
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
