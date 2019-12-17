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

type Client interface {
	GetPlans(ctx context.Context, instance string) ([]types.Plan, *http.Response, error)
	GetFlavors(ctx context.Context, instance string) ([]types.Flavor, *http.Response, error)
	Scale(ctx context.Context, args ScaleArgs) (*http.Response, error)
}
