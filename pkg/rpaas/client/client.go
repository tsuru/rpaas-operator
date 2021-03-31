// Copyright 2019 tsuru authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package client

import (
	"context"
	"io"

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

type DeleteCertificateArgs struct {
	Instance string
	Name     string
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
	GetPlans(ctx context.Context, instance string) ([]types.Plan, error)
	GetFlavors(ctx context.Context, instance string) ([]types.Flavor, error)
	Scale(ctx context.Context, args ScaleArgs) error
	Info(ctx context.Context, args InfoArgs) (*types.InstanceInfo, error)
	UpdateCertificate(ctx context.Context, args UpdateCertificateArgs) error
	DeleteCertificate(ctx context.Context, args DeleteCertificateArgs) error
	UpdateBlock(ctx context.Context, args UpdateBlockArgs) error
	DeleteBlock(ctx context.Context, args DeleteBlockArgs) error
	ListBlocks(ctx context.Context, args ListBlocksArgs) ([]types.Block, error)
	DeleteRoute(ctx context.Context, args DeleteRouteArgs) error
	ListRoutes(ctx context.Context, args ListRoutesArgs) ([]types.Route, error)
	UpdateRoute(ctx context.Context, args UpdateRouteArgs) error
	GetAutoscale(ctx context.Context, args GetAutoscaleArgs) (*types.Autoscale, error)
	UpdateAutoscale(ctx context.Context, args UpdateAutoscaleArgs) error
	RemoveAutoscale(ctx context.Context, args RemoveAutoscaleArgs) error
	Exec(ctx context.Context, args ExecArgs) (*websocket.Conn, error)
	SetService(service string) error
}
