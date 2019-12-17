// Copyright 2019 tsuru authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package client

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/tsuru/rpaas-operator/pkg/rpaas/client/types"
)

var (
	ErrUnexpectedStatusCode = fmt.Errorf("rpaasv2: unexpected status code")
)

type ClientOptions struct {
	Timeout time.Duration
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

	if target == "" || token == "" || service == "" {
		return nil, fmt.Errorf("cannot create a client over tsuru without either target, token or service")
	}

	return &client{
		tsuruTarget:  target,
		tsuruToken:   token,
		tsuruService: service,
		throughTsuru: true,
		client:       newHTTPClient(opts),
	}, nil
}

func newHTTPClient(opts ClientOptions) *http.Client {
	return &http.Client{
		Timeout: opts.Timeout,
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
}

var _ Client = &client{}

func (c *client) GetPlans(ctx context.Context, instance string) ([]types.Plan, *http.Response, error) {
	return nil, nil, fmt.Errorf("not implemented yet")
}

func (c *client) GetFlavors(ctx context.Context, instance string) ([]types.Flavor, *http.Response, error) {
	return nil, nil, fmt.Errorf("not implemented yet")
}

func (args ScaleArgs) Validate() error {
	if args.Instance == "" {
		return fmt.Errorf("rpaasv2: instance cannot be empty")
	}

	if args.Replicas < int32(0) {
		return fmt.Errorf("rpaasv2: replicas must be greater or equal than zero")
	}

	return nil
}

func (c *client) Scale(ctx context.Context, args ScaleArgs) (*http.Response, error) {
	if err := args.Validate(); err != nil {
		return nil, err
	}

	request, err := c.buildRequest("Scale", args)
	if err != nil {
		return nil, err
	}

	response, err := c.do(ctx, request)
	if err != nil {
		return response, err
	}

	if response.StatusCode != http.StatusOK {
		return response, ErrUnexpectedStatusCode
	}

	return response, nil
}

func (c *client) buildRequest(operation string, data interface{}) (req *http.Request, err error) {
	switch operation {
	case "Scale":
		args := data.(ScaleArgs)
		pathName := fmt.Sprintf("/resources/%s/scale", args.Instance)
		values := url.Values{}
		values.Set("quantity", fmt.Sprint(args.Replicas))
		body := strings.NewReader(values.Encode())
		req, err = c.newRequest("POST", pathName, body, args.Instance)
	default:
		err = fmt.Errorf("rpaasv2: unknown operation")
	}

	return
}

func (c *client) newRequest(method, pathName string, body io.Reader, instance string) (*http.Request, error) {
	url := c.formatURL(pathName, instance)
	request, err := http.NewRequest(method, url, body)
	if err != nil {
		return nil, err
	}

	if c.throughTsuru {
		request.Header.Set("Authorization", fmt.Sprintf("Bearer %s", c.tsuruToken))
		return request, nil
	}

	if c.rpaasUser != "" && c.rpaasPassword != "" {
		request.SetBasicAuth(c.rpaasUser, c.rpaasPassword)
	}

	return request, nil
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
