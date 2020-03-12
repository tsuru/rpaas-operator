// Copyright 2019 tsuru authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package fake

import (
	"context"
	"net/http"

	"github.com/tsuru/rpaas-operator/pkg/rpaas/client"
	"github.com/tsuru/rpaas-operator/pkg/rpaas/client/types"
)

type FakeClient struct {
	FakeGetPlans          func(instance string) ([]types.Plan, *http.Response, error)
	FakeGetFlavors        func(instance string) ([]types.Flavor, *http.Response, error)
	FakeScale             func(args client.ScaleArgs) (*http.Response, error)
	FakeUpdateCertificate func(args client.UpdateCertificateArgs) (*http.Response, error)
	FakeUpdateBlock       func(args client.UpdateBlockArgs) (*http.Response, error)
	FakeDeleteBlock       func(args client.DeleteBlockArgs) (*http.Response, error)
	FakeListBlocks        func(args client.ListBlocksArgs) ([]client.Block, *http.Response, error)
	FakeDeleteRoute       func(args client.DeleteRouteArgs) (*http.Response, error)
	FakeListRoutes        func(args client.ListRoutesArgs) ([]client.Route, *http.Response, error)
	FakeUpdateRoute       func(args client.UpdateRouteArgs) (*http.Response, error)
	FakeInfo              func(args client.InfoArgs) (*types.InstanceInfo, *http.Response, error)
}

var _ client.Client = &FakeClient{}

func (f *FakeClient) Info(ctx context.Context, args client.InfoArgs) (*types.InstanceInfo, *http.Response, error) {
	if f.FakeInfo != nil {
		return f.FakeInfo(args)
	}

	return nil, nil, nil
}

func (f *FakeClient) GetPlans(ctx context.Context, instance string) ([]types.Plan, *http.Response, error) {
	if f.FakeGetPlans != nil {
		return f.FakeGetPlans(instance)
	}

	return nil, nil, nil
}

func (f *FakeClient) GetFlavors(ctx context.Context, instance string) ([]types.Flavor, *http.Response, error) {
	if f.FakeGetFlavors != nil {
		return f.FakeGetFlavors(instance)
	}

	return nil, nil, nil
}

func (f *FakeClient) Scale(ctx context.Context, args client.ScaleArgs) (*http.Response, error) {
	if f.FakeScale != nil {
		return f.FakeScale(args)
	}

	return nil, nil
}

func (f *FakeClient) UpdateCertificate(ctx context.Context, args client.UpdateCertificateArgs) (*http.Response, error) {
	if f.FakeUpdateCertificate != nil {
		return f.FakeUpdateCertificate(args)
	}

	return nil, nil
}

func (f *FakeClient) UpdateBlock(ctx context.Context, args client.UpdateBlockArgs) (*http.Response, error) {
	if f.FakeUpdateBlock != nil {
		return f.FakeUpdateBlock(args)
	}

	return nil, nil
}

func (f *FakeClient) DeleteBlock(ctx context.Context, args client.DeleteBlockArgs) (*http.Response, error) {
	if f.FakeDeleteBlock != nil {
		return f.FakeDeleteBlock(args)
	}

	return nil, nil
}

func (f *FakeClient) ListBlocks(ctx context.Context, args client.ListBlocksArgs) ([]client.Block, *http.Response, error) {
	if f.FakeListBlocks != nil {
		return f.FakeListBlocks(args)
	}

	return nil, nil, nil
}

func (f *FakeClient) DeleteRoute(ctx context.Context, args client.DeleteRouteArgs) (*http.Response, error) {
	if f.FakeDeleteRoute != nil {
		return f.FakeDeleteRoute(args)
	}

	return nil, nil
}

func (f *FakeClient) ListRoutes(ctx context.Context, args client.ListRoutesArgs) ([]client.Route, *http.Response, error) {
	if f.FakeListRoutes != nil {
		return f.FakeListRoutes(args)
	}

	return nil, nil, nil
}

func (f *FakeClient) UpdateRoute(ctx context.Context, args client.UpdateRouteArgs) (*http.Response, error) {
	if f.FakeUpdateRoute != nil {
		return f.FakeUpdateRoute(args)
	}

	return nil, nil
}
