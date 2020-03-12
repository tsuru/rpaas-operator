// Copyright 2019 tsuru authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package client

import (
	"context"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var (
	FakeTsuruToken   = "f4k3t0k3n"
	FakeTsuruService = "rpaasv2"
)

func TestNewClientThroughTsuruWithOptions(t *testing.T) {
	tests := []struct {
		name          string
		target        string
		token         string
		service       string
		opts          ClientOptions
		expected      *client
		expectedError string
		setUp         func(t *testing.T)
		teardown      func(t *testing.T)
	}{
		{
			name:          "missing all mandatory arguments",
			expectedError: ErrMissingTsuruTarget.Error(),
		},
		{
			name:          "missing Tsuru service",
			target:        "https://tsuru.example.com",
			token:         "some-token",
			expectedError: ErrMissingTsuruService.Error(),
		},
		{
			name:    "creating a client successfully",
			target:  "https://tsuru.example.com",
			token:   "some-token",
			service: "rpaasv2",
			expected: &client{
				tsuruTarget:  "https://tsuru.example.com",
				tsuruToken:   "some-token",
				tsuruService: "rpaasv2",
				throughTsuru: true,
				client:       &http.Client{},
			},
		},
		{
			name:    "getting Tsuru target and token from env vars",
			service: "rpaasv2",
			opts: ClientOptions{
				Timeout: 5 * time.Second,
			},
			expected: &client{
				tsuruTarget:  "https://tsuru.example.com",
				tsuruToken:   "tsuru-token",
				tsuruService: "rpaasv2",
				throughTsuru: true,
				client: &http.Client{
					Timeout: 5 * time.Second,
				},
			},
			setUp: func(t *testing.T) {
				require.NoError(t, os.Setenv("TSURU_TARGET", "https://tsuru.example.com"))
				require.NoError(t, os.Setenv("TSURU_TOKEN", "tsuru-token"))
			},
			teardown: func(t *testing.T) {
				require.NoError(t, os.Unsetenv("TSURU_TARGET"))
				require.NoError(t, os.Unsetenv("TSURU_TOKEN"))
			},
		},
		{
			name:    "when tsuru target and token both are set on args and env vars, should prefer the args ones",
			target:  "https://tsuru.example.com",
			token:   "tok3n",
			service: "rpaasv2",
			expected: &client{
				tsuruTarget:  "https://tsuru.example.com",
				tsuruToken:   "tok3n",
				tsuruService: "rpaasv2",
				throughTsuru: true,
				client:       &http.Client{},
			},
			setUp: func(t *testing.T) {
				require.NoError(t, os.Setenv("TSURU_TARGET", "https://other.tsuru.example.com"))
				require.NoError(t, os.Setenv("TSURU_TOKEN", "tsuru-token"))
			},
			teardown: func(t *testing.T) {
				require.NoError(t, os.Unsetenv("TSURU_TARGET"))
				require.NoError(t, os.Unsetenv("TSURU_TOKEN"))
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.setUp != nil {
				tt.setUp(t)
			}
			rpaasClient, err := NewClientThroughTsuruWithOptions(tt.target, tt.token, tt.service, tt.opts)
			if tt.expectedError != "" {
				assert.EqualError(t, err, tt.expectedError)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tt.expected, rpaasClient.(*client))
			}
			if tt.teardown != nil {
				tt.teardown(t)
			}
		})
	}
}

func TestClientThroughTsuru_Scale(t *testing.T) {
	tests := []struct {
		name          string
		args          ScaleArgs
		expectedError string
		handler       http.HandlerFunc
	}{
		{
			name: "when replicas number is negative",
			args: ScaleArgs{
				Instance: "my-instance",
				Replicas: int32(-1),
			},
			expectedError: "rpaasv2: replicas must be greater or equal than zero",
		},
		{
			name: "when server returns an unexpected status code",
			args: ScaleArgs{
				Instance: "my-instance",
				Replicas: int32(777),
			},
			expectedError: ErrUnexpectedStatusCode.Error(),
			handler: func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusNotFound)
				fmt.Fprintf(w, "instance not found")
			},
		},
		{
			name: "when server returns the expected response",
			args: ScaleArgs{
				Instance: "my-instance",
				Replicas: int32(777),
			},
			handler: func(w http.ResponseWriter, r *http.Request) {
				assert.Equal(t, r.Method, "POST")
				assert.Equal(t, fmt.Sprintf("/services/%s/proxy/%s?callback=%s", FakeTsuruService, "my-instance", "/resources/my-instance/scale"), r.URL.RequestURI())
				assert.Equal(t, "Bearer f4k3t0k3n", r.Header.Get("Authorization"))
				assert.Equal(t, "application/x-www-form-urlencoded", r.Header.Get("Content-Type"))
				assert.Equal(t, "quantity=777", getBody(t, r))
				w.WriteHeader(http.StatusOK)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client, server := newClientThroughTsuru(t, tt.handler)
			defer server.Close()
			_, err := client.Scale(context.TODO(), tt.args)
			if tt.expectedError == "" {
				require.NoError(t, err)
				return
			}
			assert.EqualError(t, err, tt.expectedError)
		})
	}
}

func TestClientThroughTsuru_UpdateCertificate(t *testing.T) {
	tests := []struct {
		name          string
		args          UpdateCertificateArgs
		expectedError string
		handler       http.HandlerFunc
	}{
		{
			name: "when instance is empty",
			args: UpdateCertificateArgs{
				Certificate: "some cert",
				Key:         "some key",
			},
			expectedError: "rpaasv2: instance cannot be empty",
		},
		{
			name: "when certificate is empty",
			args: UpdateCertificateArgs{
				Instance: "my-instance",
				Key:      "some key",
			},
			expectedError: "rpaasv2: certificate cannot be empty",
		},
		{
			name: "when key is empty",
			args: UpdateCertificateArgs{
				Instance:    "my-instance",
				Certificate: "some cert",
			},
			expectedError: "rpaasv2: key cannot be empty",
		},
		{
			name: "when the server returns the expected response",
			args: UpdateCertificateArgs{
				Instance:    "my-instance",
				Certificate: `my certificate`,
				Key:         `my key`,
				boundary:    "custom-boundary",
			},
			handler: func(w http.ResponseWriter, r *http.Request) {
				assert.Equal(t, r.Method, "POST")
				assert.Equal(t, fmt.Sprintf("/services/%s/proxy/%s?callback=%s", FakeTsuruService, "my-instance", "/resources/my-instance/certificate"), r.URL.RequestURI())
				assert.Equal(t, "Bearer f4k3t0k3n", r.Header.Get("Authorization"))
				assert.Equal(t, "multipart/form-data; boundary=\"custom-boundary\"", r.Header.Get("Content-Type"))
				assert.Equal(t, "--custom-boundary\r\nContent-Disposition: form-data; name=\"cert\"; filename=\"cert.pem\"\r\nContent-Type: application/octet-stream\r\n\r\nmy certificate\r\n--custom-boundary\r\nContent-Disposition: form-data; name=\"key\"; filename=\"key.pem\"\r\nContent-Type: application/octet-stream\r\n\r\nmy key\r\n--custom-boundary\r\nContent-Disposition: form-data; name=\"name\"\r\n\r\n\r\n--custom-boundary--\r\n", getBody(t, r))
				w.WriteHeader(http.StatusOK)
			},
		},
		{
			name: "when the server returns an error",
			args: UpdateCertificateArgs{
				Instance:    "my-instance",
				Certificate: `my certificate`,
				Key:         `my key`,
			},
			expectedError: ErrUnexpectedStatusCode.Error(),
			handler: func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusNotFound)
				fmt.Fprintf(w, "instance not found")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client, server := newClientThroughTsuru(t, tt.handler)
			defer server.Close()
			_, err := client.UpdateCertificate(context.TODO(), tt.args)
			if tt.expectedError == "" {
				require.NoError(t, err)
				return
			}
			assert.EqualError(t, err, tt.expectedError)
		})
	}
}

func TestClientThroughTsuru_UpdateBlock(t *testing.T) {
	tests := []struct {
		name          string
		args          UpdateBlockArgs
		expectedError string
		handler       http.HandlerFunc
	}{
		{
			name:          "when instance is empty",
			expectedError: "rpaasv2: instance cannot be empty",
		},
		{
			name: "when block name is empty",
			args: UpdateBlockArgs{
				Instance: "some-instance",
				Content:  "some content",
			},
			expectedError: "rpaasv2: block name cannot be empty",
		},
		{
			name: "when content is empty",
			args: UpdateBlockArgs{
				Instance: "my-instance",
				Name:     "server",
			},
			expectedError: "rpaasv2: content cannot be empty",
		},
		{
			name: "when the server returns the expected response",
			args: UpdateBlockArgs{
				Instance: "my-instance",
				Name:     "http",
				Content:  "# NGINX configuration block",
			},
			handler: func(w http.ResponseWriter, r *http.Request) {
				assert.Equal(t, r.Method, "POST")
				assert.Equal(t, fmt.Sprintf("/services/%s/proxy/%s?callback=%s", FakeTsuruService, "my-instance", "/resources/my-instance/block"), r.URL.RequestURI())
				assert.Equal(t, "Bearer f4k3t0k3n", r.Header.Get("Authorization"))
				assert.Equal(t, "application/x-www-form-urlencoded", r.Header.Get("Content-Type"))
				assert.Equal(t, "block_name=http&content=%23+NGINX+configuration+block", getBody(t, r))
				w.WriteHeader(http.StatusOK)
			},
		},
		{
			name: "when the server returns an error",
			args: UpdateBlockArgs{
				Instance: "my-instance",
				Name:     "server",
				Content:  "Some NGINX snippet",
			},
			expectedError: "rpaasv2: unexpected status code",
			handler: func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusNotFound)
				fmt.Fprintf(w, "instance not found")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client, server := newClientThroughTsuru(t, tt.handler)
			defer server.Close()
			_, err := client.UpdateBlock(context.TODO(), tt.args)
			if tt.expectedError == "" {
				require.NoError(t, err)
				return
			}
			assert.EqualError(t, err, tt.expectedError)
		})
	}
}

func TestClientThroughTsuru_DeleteBlock(t *testing.T) {
	tests := []struct {
		name          string
		args          DeleteBlockArgs
		expectedError string
		handler       http.HandlerFunc
	}{
		{
			name:          "when instance is empty",
			expectedError: "rpaasv2: instance cannot be empty",
		},
		{
			name: "when block name is empty",
			args: DeleteBlockArgs{
				Instance: "some-instance",
			},
			expectedError: "rpaasv2: block name cannot be empty",
		},
		{
			name: "when the server returns the expected response",
			args: DeleteBlockArgs{
				Instance: "my-instance",
				Name:     "http",
			},
			handler: func(w http.ResponseWriter, r *http.Request) {
				assert.Equal(t, r.Method, "DELETE")
				assert.Equal(t, fmt.Sprintf("/services/%s/proxy/%s?callback=%s", FakeTsuruService, "my-instance", "/resources/my-instance/block/http"), r.URL.RequestURI())
				assert.Equal(t, "Bearer f4k3t0k3n", r.Header.Get("Authorization"))
				w.WriteHeader(http.StatusOK)
			},
		},
		{
			name: "when the server returns an error",
			args: DeleteBlockArgs{
				Instance: "my-instance",
				Name:     "server",
			},
			expectedError: "rpaasv2: unexpected status code",
			handler: func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusNotFound)
				fmt.Fprintf(w, "instance not found")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client, server := newClientThroughTsuru(t, tt.handler)
			defer server.Close()
			_, err := client.DeleteBlock(context.TODO(), tt.args)
			if tt.expectedError == "" {
				require.NoError(t, err)
				return
			}
			assert.EqualError(t, err, tt.expectedError)
		})
	}
}

func TestClientThroughTsuru_ListBlocks(t *testing.T) {
	tests := []struct {
		name          string
		args          ListBlocksArgs
		expected      []Block
		expectedError string
		handler       http.HandlerFunc
	}{
		{
			name:          "when instance is empty",
			expectedError: "rpaasv2: instance cannot be empty",
		},
		{
			name: "when the server returns an error",
			args: ListBlocksArgs{
				Instance: "my-instance",
			},
			expectedError: "rpaasv2: unexpected status code",
			handler: func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusNotFound)
				fmt.Fprintf(w, "instance not found")
			},
		},
		{
			name: "when the server returns the expected response",
			args: ListBlocksArgs{
				Instance: "my-instance",
			},
			expected: []Block{
				{
					Name:    "http",
					Content: "Some HTTP conf",
				},
				{
					Name:    "server",
					Content: "Some server conf",
				},
			},
			handler: func(w http.ResponseWriter, r *http.Request) {
				assert.Equal(t, r.Method, "GET")
				assert.Equal(t, fmt.Sprintf("/services/%s/proxy/%s?callback=%s", FakeTsuruService, "my-instance", "/resources/my-instance/block"), r.URL.RequestURI())
				assert.Equal(t, "Bearer f4k3t0k3n", r.Header.Get("Authorization"))
				fmt.Fprintf(w, `{"blocks": [{"block_name": "http", "content": "Some HTTP conf"}, {"block_name": "server", "content": "Some server conf"}]}`)
				w.WriteHeader(http.StatusOK)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client, server := newClientThroughTsuru(t, tt.handler)
			defer server.Close()
			blocks, _, err := client.ListBlocks(context.TODO(), tt.args)
			if tt.expectedError != "" {
				assert.EqualError(t, err, tt.expectedError)
				return
			}
			assert.NoError(t, err)
			assert.Equal(t, tt.expected, blocks)
		})
	}
}

func TestClientThroughTsuru_DeleteRoute(t *testing.T) {
	tests := []struct {
		name          string
		args          DeleteRouteArgs
		expectedError string
		handler       http.HandlerFunc
	}{
		{
			name:          "when instance is empty",
			expectedError: "rpaasv2: instance cannot be empty",
		},
		{
			name: "when path is empty",
			args: DeleteRouteArgs{
				Instance: "my-instance",
			},
			expectedError: "rpaasv2: path cannot be empty",
		},
		{
			name: "when the server returns an error",
			args: DeleteRouteArgs{
				Instance: "my-instance",
				Path:     "/custom/path",
			},
			expectedError: "rpaasv2: unexpected status code",
			handler: func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusNotFound)
				fmt.Fprintf(w, "instance not found")
			},
		},
		{
			name: "when the server returns the expected response",
			args: DeleteRouteArgs{
				Instance: "my-instance",
				Path:     "/custom/path",
			},
			handler: func(w http.ResponseWriter, r *http.Request) {
				assert.Equal(t, r.Method, "DELETE")
				assert.Equal(t, fmt.Sprintf("/services/%s/proxy/%s?callback=%s", FakeTsuruService, "my-instance", "/resources/my-instance/route"), r.URL.RequestURI())
				assert.Equal(t, "Bearer f4k3t0k3n", r.Header.Get("Authorization"))
				assert.Equal(t, "application/x-www-form-urlencoded", r.Header.Get("Content-Type"))
				assert.Equal(t, "path=%2Fcustom%2Fpath", getBody(t, r))
				w.WriteHeader(http.StatusOK)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client, server := newClientThroughTsuru(t, tt.handler)
			defer server.Close()
			_, err := client.DeleteRoute(context.TODO(), tt.args)
			if tt.expectedError != "" {
				assert.EqualError(t, err, tt.expectedError)
				return
			}
			assert.NoError(t, err)
		})
	}
}

func TestClientThroughTsuru_ListRoutes(t *testing.T) {
	tests := []struct {
		name          string
		args          ListRoutesArgs
		expected      []Route
		expectedError string
		handler       http.HandlerFunc
	}{
		{
			name:          "when instance is empty",
			expectedError: "rpaasv2: instance cannot be empty",
		},
		{
			name: "when the server returns an error",
			args: ListRoutesArgs{
				Instance: "my-instance",
			},
			expectedError: "rpaasv2: unexpected status code",
			handler: func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusNotFound)
				fmt.Fprintf(w, "instance not found")
			},
		},
		{
			name: "when the server returns the expected response",
			args: ListRoutesArgs{
				Instance: "my-instance",
			},
			expected: []Route{
				{
					Path:        "/static",
					Destination: "static.apps.tsuru.example.com",
				},
				{
					Path:        "/login",
					Destination: "login.apps.tsuru.example.com",
					HTTPSOnly:   true,
				},
				{
					Path:    "/custom/path",
					Content: "# some NGINX configuration",
				},
			},
			handler: func(w http.ResponseWriter, r *http.Request) {
				assert.Equal(t, r.Method, "GET")
				assert.Equal(t, fmt.Sprintf("/services/%s/proxy/%s?callback=%s", FakeTsuruService, "my-instance", "/resources/my-instance/route"), r.URL.RequestURI())
				assert.Equal(t, "Bearer f4k3t0k3n", r.Header.Get("Authorization"))
				fmt.Fprintf(w, `{"paths": [{"path": "/static", "destination": "static.apps.tsuru.example.com"}, {"path": "/login", "destination": "login.apps.tsuru.example.com", "https_only": true}, {"path": "/custom/path", "content": "# some NGINX configuration"}]}`)
				w.WriteHeader(http.StatusOK)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client, server := newClientThroughTsuru(t, tt.handler)
			defer server.Close()
			blocks, _, err := client.ListRoutes(context.TODO(), tt.args)
			if tt.expectedError != "" {
				assert.EqualError(t, err, tt.expectedError)
				return
			}
			assert.NoError(t, err)
			assert.Equal(t, tt.expected, blocks)
		})
	}
}

func TestClientThroughTsuru_Info(t *testing.T) {
	tests := []struct {
		name          string
		args          InfoArgs
		expectedError string
		handler       http.HandlerFunc
	}{
		{
			name:          "when instance is empty",
			expectedError: "rpaasv2: instance cannot be empty",
		},
		{
			name: "when all args are valid",
			args: InfoArgs{
				Instance: "my-instance",
				Raw:      true,
			},
			handler: func(w http.ResponseWriter, r *http.Request) {
				assert.Equal(t, r.Method, "GET")
				assert.Equal(t, fmt.Sprintf("/services/%s/proxy/%s?callback=%s", FakeTsuruService, "my-instance", "/resources/my-instance/info"), r.URL.RequestURI())
				assert.Equal(t, "Bearer f4k3t0k3n", r.Header.Get("Authorization"))
				fmt.Fprintf(w, `{"address":[{"hostname":"some-host","ip":"0.0.0.0"},{"hostname":"some-host2","ip":"0.0.0.1"}],"replicas":5,"plan":"basic","locations":[{"path":"some-path","destination":"some-destination"}],"binds":[{"name":"some-name","host":"some-host"},{"name":"some-name2","host":"some-host2"}],"team":"some team","name":"my-instance","description":"some description","tags":["tag1","tag2","tag3"]}`)
				w.WriteHeader(http.StatusOK)
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client, server := newClientThroughTsuru(t, tt.handler)
			defer server.Close()
			_, _, err := client.Info(context.TODO(), tt.args)
			if tt.expectedError != "" {
				assert.EqualError(t, err, tt.expectedError)
				return
			}
			assert.NoError(t, err)
		})
	}
}

func TestClientThroughTsuru_UpdateRoute(t *testing.T) {
	tests := []struct {
		name          string
		args          UpdateRouteArgs
		expectedError string
		handler       http.HandlerFunc
	}{
		{
			name:          "when instance is empty",
			expectedError: "rpaasv2: instance cannot be empty",
		},
		{
			name: "when path is empty",
			args: UpdateRouteArgs{
				Instance: "my-instance",
			},
			expectedError: "rpaasv2: path cannot be empty",
		},
		{
			name: "when the server returns an error",
			args: UpdateRouteArgs{
				Instance:    "my-instance",
				Path:        "/app",
				Destination: "app.tsuru.example.com",
			},
			expectedError: "rpaasv2: unexpected status code",
			handler: func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusNotFound)
				fmt.Fprintf(w, "instance not found")
			},
		},
		{
			name: "when the server returns the expected response",
			args: UpdateRouteArgs{
				Instance:    "my-instance",
				Path:        "/app",
				Destination: "app.tsuru.example.com",
			},
			handler: func(w http.ResponseWriter, r *http.Request) {
				assert.Equal(t, r.Method, "POST")
				assert.Equal(t, fmt.Sprintf("/services/%s/proxy/%s?callback=%s", FakeTsuruService, "my-instance", "/resources/my-instance/route"), r.URL.RequestURI())
				assert.Equal(t, "Bearer f4k3t0k3n", r.Header.Get("Authorization"))
				expected := url.Values{
					"path":        []string{"/app"},
					"destination": []string{"app.tsuru.example.com"},
					"https_only":  []string{"false"},
					"content":     []string{""},
				}
				values, err := url.ParseQuery(getBody(t, r))
				assert.NoError(t, err)
				assert.Equal(t, expected, values)
				w.WriteHeader(http.StatusCreated)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client, server := newClientThroughTsuru(t, tt.handler)
			defer server.Close()
			_, err := client.UpdateRoute(context.TODO(), tt.args)
			if tt.expectedError != "" {
				assert.EqualError(t, err, tt.expectedError)
				return
			}
			assert.NoError(t, err)
		})
	}
}

func newClientThroughTsuru(t *testing.T, h http.Handler) (Client, *httptest.Server) {
	server := httptest.NewServer(h)
	client, err := NewClientThroughTsuru(server.URL, FakeTsuruToken, FakeTsuruService)
	require.NoError(t, err)
	return client, server
}

func getBody(t *testing.T, r *http.Request) string {
	body, err := ioutil.ReadAll(r.Body)
	require.NoError(t, err)
	defer r.Body.Close()
	return string(body)
}
