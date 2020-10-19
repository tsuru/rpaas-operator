// Copyright 2020 tsuru authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package api

import (
	"bufio"
	"crypto/tls"
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"
	"testing"

	"github.com/gorilla/websocket"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/net/http2"

	"github.com/tsuru/rpaas-operator/internal/config"
	"github.com/tsuru/rpaas-operator/internal/pkg/rpaas"
	"github.com/tsuru/rpaas-operator/internal/pkg/rpaas/fake"
)

var h2cClient = &http.Client{
	Transport: &http2.Transport{
		AllowHTTP: true,
		DialTLS: func(network, addr string, cfg *tls.Config) (net.Conn, error) {
			return net.Dial(network, addr)
		},
	},
}

func Test_Exec(t *testing.T) {
	require.NoError(t, config.Init())
	originalConfig := config.Get()
	defer config.Set(originalConfig)

	var called bool
	var clientCh, serverCh chan bool

	tests := []struct {
		name           string
		manager        *fake.RpaasManager
		request        func(t *testing.T, serverURL string)
		expectedCalled bool
	}{
		{
			name: "over websocket from a not allowed origin",
			request: func(t *testing.T, serverURL string) {
				uri := fmt.Sprintf("ws://%s/resources/my-instance/exec?ws=true&command=bash&tty=true&width=80&height=24", strings.TrimPrefix(serverURL, "http://"))
				_, response, err := websocket.DefaultDialer.Dial(uri, http.Header{"origin": {"evil.test"}})
				require.Error(t, err)
				assert.Equal(t, http.StatusForbidden, response.StatusCode)
				assert.Equal(t, "Forbidden", bodyContent(response))
			},
		},
		{
			name: "upgrading to WebSocket via HTTP/1.1",
			request: func(t *testing.T, serverURL string) {
				uri := fmt.Sprintf("ws://%s/resources/my-instance/exec?ws=true&interactive=true&command=bash&tty=true&width=80&height=24", strings.TrimPrefix(serverURL, "http://"))
				conn, _, err := websocket.DefaultDialer.Dial(uri, nil)
				require.NoError(t, err)
				defer conn.Close()

				<-clientCh
				mtype, b, err := conn.ReadMessage()
				require.NoError(t, err)
				assert.Equal(t, mtype, websocket.TextMessage)
				assert.Equal(t, string(b), "root@some-hostname # ")

				conn.WriteMessage(websocket.TextMessage, []byte("my-interactive-command\n"))
				serverCh <- true

				<-clientCh
				mtype, b, err = conn.ReadMessage()
				require.NoError(t, err)
				assert.Equal(t, mtype, websocket.TextMessage)
				assert.Equal(t, string(b), "some error :/\n")
			},
			manager: &fake.RpaasManager{
				FakeExec: func(instance string, args rpaas.ExecArgs) error {
					called = true
					assert.Equal(t, instance, "my-instance")
					assert.Equal(t, args.Command, []string{"bash"})
					assert.Equal(t, args.TTY, true)
					assert.Equal(t, args.TerminalWidth, uint16(80))
					assert.Equal(t, args.TerminalHeight, uint16(24))
					assert.NotNil(t, args.Stdin)
					assert.NotNil(t, args.Stderr)
					assert.NotNil(t, args.Stdout)

					fmt.Fprintf(args.Stdout, "root@some-hostname # ")
					clientCh <- true

					<-serverCh
					r := bufio.NewReader(args.Stdin)

					body, _, err := r.ReadLine()
					assert.NoError(t, err)
					assert.Equal(t, "my-interactive-command", string(body))

					fmt.Fprintf(args.Stderr, "some error :/\n")
					clientCh <- true
					return nil
				},
			},
			expectedCalled: true,
		},
		{
			name: "using HTTP/2 directly (h2c)",
			request: func(t *testing.T, serverURL string) {
				pr, pw := io.Pipe()
				uri := fmt.Sprintf("%s/resources/my-instance/exec?ws=false&interactive=true&command=/bin/sh&tty=true&width=124&height=80", serverURL)
				request, err := http.NewRequest("POST", uri, pr)
				require.NoError(t, err)

				response, err := h2cClient.Do(request)
				require.NoError(t, err)
				defer response.Body.Close()
				assert.NotNil(t, response)
				require.Equal(t, http.StatusOK, response.StatusCode)

				<-clientCh
				r := bufio.NewReader(response.Body)
				body, _, err := r.ReadLine()
				require.NoError(t, err)
				assert.Equal(t, "root@some-hostname # ", string(body))

				fmt.Fprintf(pw, "./my-command.sh -abcde\r\n")
				serverCh <- true

				<-clientCh
				body, _, err = r.ReadLine()
				require.NoError(t, err)
				assert.Equal(t, "some command error", string(body))
			},
			manager: &fake.RpaasManager{
				FakeExec: func(instance string, args rpaas.ExecArgs) error {
					called = true
					assert.Equal(t, instance, "my-instance")
					assert.Equal(t, args.Command, []string{"/bin/sh"})
					assert.Equal(t, args.TTY, true)
					assert.Equal(t, args.TerminalWidth, uint16(124))
					assert.Equal(t, args.TerminalHeight, uint16(80))
					assert.NotNil(t, args.Stdin)
					assert.NotNil(t, args.Stderr)
					assert.NotNil(t, args.Stdout)

					fmt.Fprintf(args.Stdout, "root@some-hostname # \r\n")
					clientCh <- true

					<-serverCh
					r := bufio.NewReader(args.Stdin)
					body, _, err := r.ReadLine()
					require.NoError(t, err)
					assert.Equal(t, "./my-command.sh -abcde", string(body))

					fmt.Fprintf(args.Stderr, "some command error\r\n")
					clientCh <- true
					return nil
				},
			},
			expectedCalled: true,
		},
		{
			name: "trying execute over HTTP/1.x",
			request: func(t *testing.T, serverURL string) {
				uri := fmt.Sprintf("%s/resources/my-instance/exec?command=some-command", serverURL)
				request, err := http.NewRequest("POST", uri, nil)
				require.NoError(t, err)

				response, err := http.DefaultClient.Do(request)
				require.NoError(t, err)
				assert.Equal(t, http.StatusHTTPVersionNotSupported, response.StatusCode)
				assert.Equal(t, bodyContent(response), "this endpoint only works over HTTP/2")
			},
		},
		{
			name: "using HTTP/2 with method different than POST",
			request: func(t *testing.T, serverURL string) {
				uri := fmt.Sprintf("%s/resources/my-instance/exec?command=some-command", serverURL)
				request, err := http.NewRequest("GET", uri, nil)
				require.NoError(t, err)

				response, err := h2cClient.Do(request)
				require.NoError(t, err)
				assert.Equal(t, http.StatusMethodNotAllowed, response.StatusCode)
				assert.Equal(t, bodyContent(response), "only POST method is supported")
			},
		},
	}

	for _, tt := range tests {
		cfg := config.Get()
		cfg.WebSocketAllowedOrigins = []string{"rpaasv2.example.com"}
		config.Set(cfg)

		t.Run(tt.name, func(t *testing.T) {
			called = false
			clientCh, serverCh = make(chan bool), make(chan bool)
			defer close(clientCh)
			defer close(serverCh)

			l, err := net.Listen("tcp", "127.0.0.1:0")
			require.NoError(t, err)
			defer l.Close()

			webApi, err := New(tt.manager)
			require.NoError(t, err)
			webApi.e.Listener = l
			go webApi.Start()
			defer webApi.Stop()

			tt.request(t, fmt.Sprintf("http://%s", l.Addr().String()))
			assert.Equal(t, called, tt.expectedCalled)
		})
	}
}
