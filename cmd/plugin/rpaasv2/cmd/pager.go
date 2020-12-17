// Copyright 2020 tsuru authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
package cmd

import (
	"bytes"
	"io"
	"os/exec"
	"strings"
	"syscall"

	"golang.org/x/crypto/ssh/terminal"
)

type descriptable interface {
	Fd() uintptr
}

func newPagerWriter(baseWriter io.Writer) io.Writer {
	pager, found := syscall.Getenv("TSURU_PAGER")
	if found && pager == "" {
		return baseWriter
	}

	outputDesc, ok := baseWriter.(descriptable)
	if !ok {
		return baseWriter
	}
	terminalFd := int(outputDesc.Fd())
	if !terminal.IsTerminal(terminalFd) {
		return baseWriter
	}

	if pager == "" {
		pager = "less -RFXE"
	}
	return &pagerWriter{baseWriter: baseWriter, pager: pager}
}

type pagerWriter struct {
	baseWriter io.Writer
	pagerPipe  io.WriteCloser
	cmd        *exec.Cmd
	pager      string
	buf        bytes.Buffer
	erroed     bool
}

func (w *pagerWriter) Write(data []byte) (int, error) {
	if w.pagerPipe != nil {
		return w.pagerPipe.Write(data)
	}
	if w.erroed {
		return w.baseWriter.Write(data)
	}
	w.buf.Write(data)
	if w.cmd == nil {
		var err error
		pagerParts := strings.Split(w.pager, " ")
		w.cmd = exec.Command(pagerParts[0], pagerParts[1:]...)
		w.cmd.Stdout = w.baseWriter
		w.pagerPipe, err = w.cmd.StdinPipe()
		if err != nil {
			w.erroed = true
		}
		err = w.cmd.Start()
		if err != nil {
			w.pagerPipe = nil
			w.erroed = true
		}
	}
	w.flush()
	return len(data), nil
}

func (w *pagerWriter) Wait() error {
	if w.cmd == nil {
		return nil
	}

	return w.cmd.Wait()
}

func (w *pagerWriter) flush() {
	if w.pagerPipe != nil {
		w.pagerPipe.Write(w.buf.Bytes())
	} else {
		w.baseWriter.Write(w.buf.Bytes())
	}
	w.buf.Reset()
}
