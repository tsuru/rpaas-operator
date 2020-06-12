package api

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"net/http"

	"github.com/tsuru/rpaas-operator/internal/pkg/rpaas"
	"golang.org/x/net/http2"
	"golang.org/x/net/http2/h2c"
)

type cmdReadWrite struct {
	body   io.Reader
	writer io.Writer
}

func (c *cmdReadWrite) Write(arr []byte) (int, error) {
	defer func() {
		flusher, _ := c.writer.(http.Flusher)
		flusher.Flush()
	}()

	return c.writer.Write(arr)
}

func (c *cmdReadWrite) Read(arr []byte) (int, error) {
	reader := bufio.NewReader(c.body)
	line, err := reader.ReadBytes('\n')
	if err != nil {
		return 0, err
	}

	return copy(arr, line), nil
}

func setupExecRoute(a *api) error {
	h2s := &http2.Server{}
	mux := http.NewServeMux()
	mux.Handle("/exec", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var useTty bool
		if tty := r.FormValue("tty"); tty == "true" {
			useTty = true
		}
		if r.URL == nil {
			w.WriteHeader(http.StatusInternalServerError)
			w.Write([]byte("missing URL"))
			return
		}
		instanceName := r.FormValue("instance")

		if err := r.ParseForm(); err != nil {
			fmt.Printf("error: %s\n\n", err.Error())
			w.WriteHeader(http.StatusInternalServerError)
			w.Write([]byte(err.Error()))
		}

		var buffer io.ReadWriter
		buffer = &cmdReadWrite{
			body:   r.Body,
			writer: w,
		}
		err := a.rpaasManager.Exec(context.TODO(), instanceName, rpaas.ExecArgs{
			Stdin:          buffer,
			Stdout:         buffer,
			Stderr:         buffer,
			Tty:            useTty,
			Command:        r.Form["command"],
			TerminalWidth:  r.FormValue("width"),
			TerminalHeight: r.FormValue("height"),
		})
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			w.Write([]byte(err.Error()))
		}
	}))
	mux.Handle("/", a.e)
	a.e.Server.Handler = h2c.NewHandler(mux, h2s)
	return nil
}
