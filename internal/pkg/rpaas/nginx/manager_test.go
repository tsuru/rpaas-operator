// Copyright 2019 tsuru authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package nginx

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNginxManager_PurgeCache(t *testing.T) {
	testCases := []struct {
		description   string
		purgePath     string
		preservePath  bool
		assertion     func(*testing.T, error)
		nginxResponse http.HandlerFunc
		status        bool
	}{
		{
			description:  "returns not found error when nginx returns 404 and preservePath is false",
			purgePath:    "/index.html",
			preservePath: false,
			assertion: func(t *testing.T, err error) {
				require.NoError(t, err)
			},
			nginxResponse: func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusNotFound)
			},
			status: false,
		},
		{
			description:  "returns not found error when nginx returns 404 and preservePath is true",
			purgePath:    "/index.html",
			preservePath: true,
			assertion: func(t *testing.T, err error) {
				require.NoError(t, err)
			},
			nginxResponse: func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusNotFound)
			},
			status: false,
		},
		{
			description:  "returns not found error when nginx returns 500",
			purgePath:    "/index.html",
			preservePath: true,
			assertion: func(t *testing.T, err error) {
				require.Error(t, err)
			},
			nginxResponse: func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusInternalServerError)
			},
			status: false,
		},
		{
			description:  "makes a request to /purge/<purgePath> when preservePath is true",
			purgePath:    "/some/path/index.html",
			preservePath: true,
			assertion: func(t *testing.T, err error) {
				require.NoError(t, err)
			},
			nginxResponse: func(w http.ResponseWriter, r *http.Request) {
				if r.RequestURI == "/purge/some/path/index.html" {
					w.WriteHeader(http.StatusOK)
				} else {
					w.WriteHeader(http.StatusNotFound)
				}
			},
			status: true,
		},
		{
			description:  "makes a request to /purge/<purgePath> when preservePath is true with custom cache key",
			purgePath:    "0:desktop:myhostname/some/path/index.html",
			preservePath: true,
			assertion: func(t *testing.T, err error) {
				require.NoError(t, err)
			},
			nginxResponse: func(w http.ResponseWriter, r *http.Request) {
				if r.RequestURI == "/purge/0:desktop:myhostname/some/path/index.html" {
					w.WriteHeader(http.StatusOK)
				} else {
					w.WriteHeader(http.StatusNotFound)
				}
			},
			status: true,
		},
		{
			description:  "makes a request to /purge/<protocol>/<purgePath> when preservePath is false",
			purgePath:    "/index.html",
			preservePath: false,
			assertion: func(t *testing.T, err error) {
				require.NoError(t, err)
			},
			nginxResponse: func(w http.ResponseWriter, r *http.Request) {
				if r.RequestURI == "/purge/http/index.html" || r.RequestURI == "/purge/https/index.html" {
					w.WriteHeader(http.StatusOK)
				} else {
					w.WriteHeader(http.StatusNotFound)
				}
			},
			status: true,
		},
		{
			description:  "requests with gzip and identity values for Accept-Encoding header when preservePath is true",
			purgePath:    "/index.html",
			preservePath: true,
			assertion: func(t *testing.T, err error) {
				require.NoError(t, err)
			},
			nginxResponse: func(w http.ResponseWriter, r *http.Request) {
				if (r.Header.Get("Accept-Encoding") == "gzip" || r.Header.Get("Accept-Encoding") == "identity") && r.RequestURI == "/purge/index.html" {
					w.WriteHeader(http.StatusOK)
				} else {
					w.WriteHeader(http.StatusNotAcceptable)
				}
			},
			status: true,
		},
		{
			description:  "requests with gzip and identity values for Accept-Encoding header when preservePath is false",
			purgePath:    "/index.html",
			preservePath: false,
			assertion: func(t *testing.T, err error) {
				require.NoError(t, err)
			},
			nginxResponse: func(w http.ResponseWriter, r *http.Request) {
				if (r.Header.Get("Accept-Encoding") == "gzip" || r.Header.Get("Accept-Encoding") == "identity") && (r.RequestURI == "/purge/http/index.html" || r.RequestURI == "/purge/https/index.html") {
					w.WriteHeader(http.StatusOK)
				} else {
					w.WriteHeader(http.StatusNotAcceptable)
				}
			},
			status: true,
		},
	}

	for _, tt := range testCases {
		t.Run(tt.description, func(t *testing.T) {
			server := httptest.NewServer(tt.nginxResponse)

			url, err := url.Parse(server.URL)
			require.NoError(t, err)

			nginx := NewNginxManager()
			port, err := strconv.ParseUint(url.Port(), 10, 16)
			require.NoError(t, err)

			purgeStatus, err := nginx.PurgeCache(url.Hostname(), tt.purgePath, int32(port), tt.preservePath)
			tt.assertion(t, err)
			assert.Equal(t, tt.status, purgeStatus)
		})
	}
}
