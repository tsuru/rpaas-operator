// Copyright 2019 tsuru authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package client

import (
	"context"
	"net/http"

	"github.com/tsuru/rpaas-operator/pkg/rpaas/client/types"
)

type ScaleArgs struct {
	Instance string
	Replicas int32
}

type UpdateCertificateArgs struct {
	Instance    string
	Name        string
	Certificate string
	Key         string

	boundary string
}

type UpdateBlockArgs struct {
	Instance string
	Name     string
	Content  string
}

type DeleteBlockArgs struct {
	Instance string
	Name     string
}

type ListBlocksArgs struct {
	Instance string
}

type Block struct {
	Name    string `json:"block_name"`
	Content string `json:"content"`
}

type DeleteRouteArgs struct {
	Instance string
	Path     string
}

type ListRoutesArgs struct {
	Instance string
}

type UpdateRouteArgs struct {
	Instance    string
	Path        string
	Destination string
	HTTPSOnly   bool
	Content     string
}

type Route struct {
	Path        string `json:"path"`
	Destination string `json:"destination,omitempty"`
	HTTPSOnly   bool   `json:"https_only,omitempty"`
	Content     string `json:"content,omitempty"`
}

type InfoArgs struct {
	Instance string
	Service  string
}

type Client interface {
	GetPlans(ctx context.Context, instance string) ([]types.Plan, *http.Response, error)
	GetFlavors(ctx context.Context, instance string) ([]types.Flavor, *http.Response, error)
	Scale(ctx context.Context, args ScaleArgs) (*http.Response, error)
	Info(ctx context.Context, args InfoArgs) (*http.Response, error)
	UpdateCertificate(ctx context.Context, args UpdateCertificateArgs) (*http.Response, error)
	UpdateBlock(ctx context.Context, args UpdateBlockArgs) (*http.Response, error)
	DeleteBlock(ctx context.Context, args DeleteBlockArgs) (*http.Response, error)
	ListBlocks(ctx context.Context, args ListBlocksArgs) ([]Block, *http.Response, error)
	DeleteRoute(ctx context.Context, args DeleteRouteArgs) (*http.Response, error)
	ListRoutes(ctx context.Context, args ListRoutesArgs) ([]Route, *http.Response, error)
	UpdateRoute(ctx context.Context, args UpdateRouteArgs) (*http.Response, error)
}
