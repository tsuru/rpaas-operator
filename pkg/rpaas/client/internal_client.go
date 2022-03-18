// Copyright 2019 tsuru authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package client

import (
	"context"
	"crypto/tls"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net"
	"net/http"
	"net/url"
	"os"
	"time"

	"github.com/gorilla/websocket"
	"github.com/tsuru/rpaas-operator/pkg/rpaas/client/types"
)

var (
	ErrMissingTsuruTarget       = fmt.Errorf("rpaasv2: tsuru target cannot be empty")
	ErrMissingTsuruToken        = fmt.Errorf("rpaasv2: tsuru token cannot be empty")
	ErrMissingTsuruService      = fmt.Errorf("rpaasv2: tsuru service cannot be empty")
	ErrMissingInstance          = fmt.Errorf("rpaasv2: instance cannot be empty")
	ErrMissingBlockName         = fmt.Errorf("rpaasv2: block name cannot be empty")
	ErrMissingPath              = fmt.Errorf("rpaasv2: path cannot be empty")
	ErrInvalidMaxReplicasNumber = fmt.Errorf("rpaasv2: max replicas can't be lower than 1")
	ErrInvalidMinReplicasNumber = fmt.Errorf("rpaasv2: min replicas can't be lower than 1 and can't be higher than the maximum number of replicas")
	ErrInvalidCPUUsage          = fmt.Errorf("rpaasv2: CPU usage can't be lower than 1%%")
	ErrInvalidMemoryUsage       = fmt.Errorf("rpaasv2: memory usage can't be lower than 1%%")
	ErrMissingValues            = fmt.Errorf("rpaasv2: values can't be all empty")
	ErrMissingExecCommand       = fmt.Errorf("rpaasv2: command cannot be empty")
)

type ErrUnexpectedStatusCode struct {
	Status int
	Body   string
}

func (e *ErrUnexpectedStatusCode) Error() string {
	humanStatus := fmt.Sprintf("%d %s", e.Status, http.StatusText(e.Status))

	if e.Body != "" {
		return fmt.Sprintf("rpaasv2: unexpected status code: %s, detail: %s", humanStatus, e.Body)
	}
	return fmt.Sprintf("rpaasv2: unexpected status code: %s", humanStatus)
}

func newErrUnexpectedStatusCodeFromResponse(r *http.Response) error {
	body, err := ioutil.ReadAll(r.Body)
	if err != nil {
		return err
	}

	return &ErrUnexpectedStatusCode{Status: r.StatusCode, Body: string(body)}
}

func isNotFoundError(err error) bool {
	if httpErr, ok := err.(*ErrUnexpectedStatusCode); ok {
		if httpErr.Status == http.StatusNotFound {
			return true
		}
	}

	return false
}

type ClientOptions struct {
	Timeout            time.Duration
	InsecureSkipVerify bool
}

var DefaultClientOptions = ClientOptions{
	Timeout: 10 * time.Second,
}

func NewClient(address, user, password string) (Client, error) {
	return NewClientWithOptions(address, user, password, DefaultClientOptions)
}

func NewClientWithOptions(address, user, password string, opts ClientOptions) (Client, error) {
	if address == "" {
		return nil, fmt.Errorf("cannot create a client without address")
	}

	return &client{
		rpaasAddress:  address,
		rpaasUser:     user,
		rpaasPassword: password,
		client:        newHTTPClient(opts),
		ws:            websocket.DefaultDialer,
	}, nil
}

func NewClientThroughTsuru(target, token, service string) (Client, error) {
	return NewClientThroughTsuruWithOptions(target, token, service, DefaultClientOptions)
}

func NewClientThroughTsuruWithOptions(target, token, service string, opts ClientOptions) (Client, error) {
	if t, ok := os.LookupEnv("TSURU_TARGET"); target == "" && ok {
		target = t
	}

	if t, ok := os.LookupEnv("TSURU_TOKEN"); token == "" && ok {
		token = t
	}

	if target == "" {
		return nil, ErrMissingTsuruTarget
	}

	if token == "" {
		return nil, ErrMissingTsuruToken
	}

	if service == "" {
		return nil, ErrMissingTsuruService
	}

	return &client{
		tsuruTarget:  target,
		tsuruToken:   token,
		tsuruService: service,
		throughTsuru: true,
		client:       newHTTPClient(opts),
		ws:           websocket.DefaultDialer,
	}, nil
}

func newHTTPClient(opts ClientOptions) *http.Client {
	transport := &http.Transport{
		Proxy: http.ProxyFromEnvironment,
		DialContext: (&net.Dialer{
			Timeout:   30 * time.Second,
			KeepAlive: 30 * time.Second,
		}).DialContext,
		ForceAttemptHTTP2:     true,
		MaxIdleConns:          100,
		IdleConnTimeout:       90 * time.Second,
		TLSHandshakeTimeout:   10 * time.Second,
		ExpectContinueTimeout: 1 * time.Second,
		TLSClientConfig: &tls.Config{
			InsecureSkipVerify: opts.InsecureSkipVerify,
		},
	}
	return &http.Client{
		Timeout:   opts.Timeout,
		Transport: transport,
	}
}

type client struct {
	rpaasAddress  string
	rpaasUser     string
	rpaasPassword string

	tsuruTarget  string
	tsuruToken   string
	tsuruService string
	throughTsuru bool

	client *http.Client
	ws     *websocket.Dialer
}

var _ Client = &client{}

func (c *client) GetPlans(ctx context.Context, instance string) ([]types.Plan, error) {
	return nil, fmt.Errorf("not implemented yet")
}

func (c *client) GetFlavors(ctx context.Context, instance string) ([]types.Flavor, error) {
	return nil, fmt.Errorf("not implemented yet")
}

func (c *client) SetService(service string) (Client, error) {
	if service == "" {
		return nil, ErrMissingTsuruService
	}

	newClient := *c
	newClient.tsuruService = service
	return &newClient, nil
}

func (c *client) newRequest(method, pathName string, body io.Reader, instance string) (*http.Request, error) {
	return c.newRequestWithQueryString(method, pathName, body, instance, nil)
}

func (c *client) newRequestWithQueryString(method, pathName string, body io.Reader, instance string, qs url.Values) (*http.Request, error) {
	url := c.formatURLWithQueryString(pathName, instance, qs)
	request, err := http.NewRequest(method, url, body)
	if err != nil {
		return nil, err
	}

	c.baseAuthHeader(request.Header)

	return request, nil
}

func (c *client) baseAuthHeader(h http.Header) http.Header {
	if h == nil {
		h = http.Header{}
	}

	if c.throughTsuru {
		h.Set("Authorization", fmt.Sprintf("Bearer %s", c.tsuruToken))
	} else if c.rpaasUser != "" && c.rpaasPassword != "" {
		h.Set("Authorization", fmt.Sprintf("Basic %s", basicAuth(c.rpaasUser, c.rpaasPassword)))
	}

	return h
}

func (c *client) do(ctx context.Context, request *http.Request) (*http.Response, error) {
	return c.client.Do(request.WithContext(ctx))
}

func (c *client) formatURL(pathName, instance string) string {
	if !c.throughTsuru {
		return fmt.Sprintf("%s%s", c.rpaasAddress, pathName)
	}

	return fmt.Sprintf("%s/services/%s/proxy/%s?callback=%s", c.tsuruTarget, c.tsuruService, instance, pathName)
}

func (c *client) formatURLWithQueryString(pathName, instance string, qs url.Values) string {
	qsData := qs.Encode()

	if !c.throughTsuru {
		if qsData != "" {
			qsData = "?" + qsData
		}

		return fmt.Sprintf("%s%s%s", c.rpaasAddress, pathName, qsData)
	}

	if qsData != "" {
		qsData = "&" + qsData
	}

	return fmt.Sprintf("%s/services/%s/proxy/%s?callback=%s%s", c.tsuruTarget, c.tsuruService, instance, pathName, qsData)
}

func unmarshalBody(resp *http.Response, dst interface{}) error {
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return err
	}

	defer resp.Body.Close()
	return json.Unmarshal(body, dst)
}

func basicAuth(username, password string) string {
	auth := username + ":" + password
	return base64.StdEncoding.EncodeToString([]byte(auth))
}
