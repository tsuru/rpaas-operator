// Copyright 2023 tsuru authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package client_test

import (
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"

	. "github.com/tsuru/rpaas-operator/pkg/rpaas/client"
)

func TestBasicAuthTransport(t *testing.T) {
	t.Parallel()

	tests := map[string]struct {
		transport http.RoundTripper
		assert    func(t *testing.T, r *http.Request)
	}{
		"with default http transport": {},

		"with custom base http transport": {
			transport: &dummyTransport{},
			assert: func(t *testing.T, r *http.Request) {
				assert.Equal(t, "1", r.Header.Get("X-Dummy-Transport"))
			},
		},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if tt.assert != nil {
					tt.assert(t, r)
				}

				assert.Equal(t, "Basic YWRtaW46YWRtaW4=", r.Header.Get("Authorization"))
				fmt.Fprintln(w, "OK")
			})

			server := httptest.NewServer(handler)
			defer server.Close()

			httpClient := server.Client()
			httpClient.Transport = &BasicAuthTransport{
				Username: "admin",
				Password: "admin",
				Base:     tt.transport,
			}

			resp, err := httpClient.Get(server.URL)
			assert.NoError(t, err)

			assert.Equal(t, http.StatusOK, resp.StatusCode)
			defer resp.Body.Close()

			body, err := io.ReadAll(resp.Body)
			assert.NoError(t, err)
			assert.Equal(t, "OK\n", string(body))
		})
	}
}

type dummyTransport struct{}

func (d *dummyTransport) RoundTrip(r *http.Request) (*http.Response, error) {
	r.Header.Set("X-Dummy-Transport", "1")
	return http.DefaultTransport.RoundTrip(r)
}
