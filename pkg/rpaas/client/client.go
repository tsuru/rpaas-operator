// Copyright 2019 tsuru authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package client

import (
	"context"
	"io"
	"net/http"

	"github.com/gorilla/websocket"
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

type InfoArgs struct {
	Instance string
	Raw      bool
}

type GetAutoscaleArgs struct {
	Instance string
	Raw      bool
}

type UpdateAutoscaleArgs struct {
	Instance    string
	MinReplicas int32
	MaxReplicas int32
	CPU         int32
	Memory      int32
}

type RemoveAutoscaleArgs struct {
	Instance string
}

type ExecArgs struct {
	In             io.Reader
	Command        []string
	Instance       string
	Pod            string
	Container      string
	TerminalWidth  uint16
	TerminalHeight uint16
	Interactive    bool
	TTY            bool
}

type Client interface {
	GetPlans(ctx context.Context, instance string) ([]types.Plan, *http.Response, error)
	GetFlavors(ctx context.Context, instance string) ([]types.Flavor, *http.Response, error)
	Scale(ctx context.Context, args ScaleArgs) (*http.Response, error)
	Info(ctx context.Context, args InfoArgs) (*types.InstanceInfo, *http.Response, error)
	UpdateCertificate(ctx context.Context, args UpdateCertificateArgs) (*http.Response, error)
	UpdateBlock(ctx context.Context, args UpdateBlockArgs) (*http.Response, error)
	DeleteBlock(ctx context.Context, args DeleteBlockArgs) (*http.Response, error)
	ListBlocks(ctx context.Context, args ListBlocksArgs) ([]types.Block, *http.Response, error)
	DeleteRoute(ctx context.Context, args DeleteRouteArgs) (*http.Response, error)
	ListRoutes(ctx context.Context, args ListRoutesArgs) ([]types.Route, *http.Response, error)
	UpdateRoute(ctx context.Context, args UpdateRouteArgs) (*http.Response, error)
	GetAutoscale(ctx context.Context, args GetAutoscaleArgs) (*types.Autoscale, *http.Response, error)
	UpdateAutoscale(ctx context.Context, args UpdateAutoscaleArgs) (*http.Response, error)
	RemoveAutoscale(ctx context.Context, args RemoveAutoscaleArgs) (*http.Response, error)
	Exec(ctx context.Context, args ExecArgs) (*websocket.Conn, *http.Response, error)
}
