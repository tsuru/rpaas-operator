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

type Client interface {
	GetPlans(ctx context.Context, instance string) ([]types.Plan, *http.Response, error)
	GetFlavors(ctx context.Context, instance string) ([]types.Flavor, *http.Response, error)
	Scale(ctx context.Context, args ScaleArgs) (*http.Response, error)
	UpdateCertificate(ctx context.Context, args UpdateCertificateArgs) (*http.Response, error)
	UpdateBlock(ctx context.Context, args UpdateBlockArgs) (*http.Response, error)
	DeleteBlock(ctx context.Context, args DeleteBlockArgs) (*http.Response, error)
}
