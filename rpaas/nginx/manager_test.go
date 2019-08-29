package nginx

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestNginxManager_PurgeCache(t *testing.T) {
	testCases := []struct {
		description   string
		purgePath     string
		preservePath  bool
		assertion     func(*testing.T, error)
		nginxResponse http.HandlerFunc
	}{
		{
			description:  "returns not found error when nginx returns 404 and preservePath is false",
			purgePath:    "/index.html",
			preservePath: false,
			assertion: func(t *testing.T, err error) {
				require.Error(t, err)
			},
			nginxResponse: func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusNotFound)
			},
		},
		{
			description:  "returns not found error when nginx returns 404 and preservePath is true",
			purgePath:    "/index.html",
			preservePath: true,
			assertion: func(t *testing.T, err error) {
				require.Error(t, err)
			},
			nginxResponse: func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusNotFound)
			},
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
					w.WriteHeader(http.StatusNoContent)
				} else {
					w.WriteHeader(http.StatusNotFound)
				}
			},
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
					w.WriteHeader(http.StatusNoContent)
				} else {
					w.WriteHeader(http.StatusNotFound)
				}
			},
		},
		{
			description:  "requests with gzip and identity values for Accept-Encoding header when preservePath is true",
			purgePath:    "/index.html",
			preservePath: true,
			assertion: func(t *testing.T, err error) {
				require.NoError(t, err)
			},
			nginxResponse: func(w http.ResponseWriter, r *http.Request) {
				if r.Header.Get("Accept-Encoding") == "gzip" || r.Header.Get("Accept-Encoding") == "identity" {
					w.WriteHeader(http.StatusNoContent)
				} else {
					w.WriteHeader(http.StatusNotAcceptable)
				}
			},
		},
		{
			description:  "requests with gzip and identity values for Accept-Encoding header when preservePath is false",
			purgePath:    "/index.html",
			preservePath: false,
			assertion: func(t *testing.T, err error) {
				require.NoError(t, err)
			},
			nginxResponse: func(w http.ResponseWriter, r *http.Request) {
				if r.Header.Get("Accept-Encoding") == "gzip" || r.Header.Get("Accept-Encoding") == "identity" {
					w.WriteHeader(http.StatusNoContent)
				} else {
					w.WriteHeader(http.StatusNotAcceptable)
				}
			},
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
			nginx.managePort = uint16(port)

			err = nginx.PurgeCache(url.Hostname(), tt.purgePath, tt.preservePath)
			tt.assertion(t, err)
		})
	}
}
