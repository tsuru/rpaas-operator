// Copyright 2021 tsuru authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package rpaas

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"net/http"
	"sync"

	v1 "k8s.io/api/core/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

type tail struct {
	LogArgs
	Pod v1.Pod
	kcs kubernetes.Interface
}

type TailManager struct {
	mu   sync.Mutex
	once sync.Once
	wg   sync.WaitGroup
	done chan struct{}
}

func NewTailManager() TailManager {
	return TailManager{
		mu:   sync.Mutex{},
		once: sync.Once{},
		wg:   sync.WaitGroup{},
		done: make(chan struct{}),
	}
}

func (tm *TailManager) Start(ctx context.Context, t tail) error {
	tm.wg.Add(1)
	tm.once.Do(
		func() {
			go tm.tailsDone()
		},
	)
	return t.start(ctx, &tm.mu, &tm.wg)
}

func (tm *TailManager) tailsDone() {
	tm.wg.Wait()
	tm.done <- struct{}{}
}

func (t *tail) Write(msg string) {
	prefix := fmt.Sprintf("[%s]: ", t.Pod.Name)
	fmt.Fprint(t.Buffer, prefix+msg)
}

func (t *tail) consumeRequest(ctx context.Context, req rest.ResponseWrapper, mu *sync.Mutex, wg *sync.WaitGroup) error {
	stream, err := req.Stream(ctx)
	if err != nil {
		return err
	}
	defer func() {
		stream.Close()
		wg.Done()
	}()

	reader := bufio.NewReader(stream)
	for {
		stringBytes, err := reader.ReadBytes('\n')
		if len(stringBytes) != 0 {
			msg := string(stringBytes)
			mu.Lock()
			t.Write(msg)
			mu.Unlock()
			if f, ok := t.Buffer.(http.Flusher); ok {
				f.Flush()
			}
		}
		if err == io.EOF {
			return nil
		}
		if err != nil {
			return err
		}
	}
}

func (t *tail) start(ctx context.Context, mu *sync.Mutex, wg *sync.WaitGroup) error {
	container := "nginx"
	if t.Container != "" {
		container = t.Container
	}

	req := t.kcs.CoreV1().Pods(t.Pod.Namespace).GetLogs(t.Pod.Name, &v1.PodLogOptions{
		Follow:       t.Follow,
		TailLines:    t.Lines,
		Container:    container,
		SinceSeconds: t.SinceSeconds,
		Timestamps:   t.WithTimestamp,
	})

	if err := t.consumeRequest(ctx, req, mu, wg); err != nil {
		return err
	}
	return nil
}
