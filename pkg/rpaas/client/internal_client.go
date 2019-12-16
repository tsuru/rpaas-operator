// Copyright 2019 tsuru authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package client

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"time"

	"github.com/tsuru/rpaas-operator/pkg/rpaas/client/types"
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
	if t, ok := os.LookupEnv("TSURU_TARGET"); ok {
		target = t
	}

	if t, ok := os.LookupEnv("TSURU_TOKEN"); ok {
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

func (c *client) Scale(ctx context.Context, args ScaleArgs) (*http.Response, error) {
	return nil, fmt.Errorf("not implemented yet")
}
