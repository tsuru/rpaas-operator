// Copyright 2023 tsuru authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package cmd

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type RpaasAPIServerGenerator func(t *testing.T, handler http.Handler) (*httptest.Server, []string)

var AllRpaasAPIServerGenerators = []RpaasAPIServerGenerator{
	NewRpaasAPIServerWithNoAuthentication,
	NewRpaasAPIServerWithAuthentication,
	NewRpaasAPIServerThroughTsuruProxy,
}

func NewRpaasAPIServerWithNoAuthentication(t *testing.T, handler http.Handler) (*httptest.Server, []string) {
	t.Helper()

	require.NotNil(t, handler, "you must provide an HTTP handler")

	h := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Contains(t, r.UserAgent(), "rpaasv2-cli")
		handler.ServeHTTP(w, r)
	})

	server := httptest.NewServer(h)
	return server, []string{"rpaasv2", "--rpaas-url", server.URL}
}

func NewRpaasAPIServerWithAuthentication(t *testing.T, handler http.Handler) (*httptest.Server, []string) {
	t.Helper()

	require.NotNil(t, handler, "you must provide an HTTP handler")

	user := "admin@example.com"
	password := "fake-password"

	h := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		u, p, _ := r.BasicAuth()
		if !assert.Equal(t, user, u) || !assert.Equal(t, password, p) {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}

		handler.ServeHTTP(w, r)
	})

	server, args := NewRpaasAPIServerWithNoAuthentication(t, h)
	return server, append(args, "--rpaas-user", user, "--rpaas-password", password)
}

func NewRpaasAPIServerThroughTsuruProxy(t *testing.T, handler http.Handler) (*httptest.Server, []string) {
	t.Helper()

	require.NotNil(t, handler, "you must provide an HTTP handler")

	token := "fake-tsuru-token"

	h := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !assert.Equal(t, fmt.Sprintf("Bearer %s", token), r.Header.Get("Authorization")) {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}

		handler.ServeHTTP(w, r)
	})

	server, _ := NewRpaasAPIServerWithNoAuthentication(t, h)
	return server, []string{"rpaasv2", "--tsuru-target", server.URL, "--tsuru-token", token}
}
