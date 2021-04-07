// Copyright 2019 tsuru authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package fake

import (
	"context"

	"github.com/gorilla/websocket"
	"github.com/tsuru/rpaas-operator/pkg/rpaas/client"
	"github.com/tsuru/rpaas-operator/pkg/rpaas/client/types"
)

type FakeClient struct {
	FakeGetPlans          func(instance string) ([]types.Plan, error)
	FakeGetFlavors        func(instance string) ([]types.Flavor, error)
	FakeScale             func(args client.ScaleArgs) error
	FakeUpdateCertificate func(args client.UpdateCertificateArgs) error
	FakeDeleteCertificate func(args client.DeleteCertificateArgs) error
	FakeUpdateBlock       func(args client.UpdateBlockArgs) error
	FakeDeleteBlock       func(args client.DeleteBlockArgs) error
	FakeListBlocks        func(args client.ListBlocksArgs) ([]types.Block, error)
	FakeDeleteRoute       func(args client.DeleteRouteArgs) error
	FakeListRoutes        func(args client.ListRoutesArgs) ([]types.Route, error)
	FakeUpdateRoute       func(args client.UpdateRouteArgs) error
	FakeInfo              func(args client.InfoArgs) (*types.InstanceInfo, error)
	FakeGetAutoscale      func(args client.GetAutoscaleArgs) (*types.Autoscale, error)
	FakeUpdateAutoscale   func(args client.UpdateAutoscaleArgs) error
	FakeRemoveAutoscale   func(args client.RemoveAutoscaleArgs) error
	FakeExec              func(ctx context.Context, args client.ExecArgs) (*websocket.Conn, error)
	FakeSetService        func(service string) error
}

var _ client.Client = &FakeClient{}

func (f *FakeClient) RemoveAutoscale(ctx context.Context, args client.RemoveAutoscaleArgs) error {
	if f.FakeRemoveAutoscale != nil {
		return f.FakeRemoveAutoscale(args)
	}

	return nil
}

func (f *FakeClient) UpdateAutoscale(ctx context.Context, args client.UpdateAutoscaleArgs) error {
	if f.FakeUpdateAutoscale != nil {
		return f.FakeUpdateAutoscale(args)
	}

	return nil
}

func (f *FakeClient) GetAutoscale(ctx context.Context, args client.GetAutoscaleArgs) (*types.Autoscale, error) {
	if f.FakeGetAutoscale != nil {
		return f.FakeGetAutoscale(args)
	}

	return nil, nil
}

func (f *FakeClient) Info(ctx context.Context, args client.InfoArgs) (*types.InstanceInfo, error) {
	if f.FakeInfo != nil {
		return f.FakeInfo(args)
	}

	return nil, nil
}

func (f *FakeClient) GetPlans(ctx context.Context, instance string) ([]types.Plan, error) {
	if f.FakeGetPlans != nil {
		return f.FakeGetPlans(instance)
	}

	return nil, nil
}

func (f *FakeClient) GetFlavors(ctx context.Context, instance string) ([]types.Flavor, error) {
	if f.FakeGetFlavors != nil {
		return f.FakeGetFlavors(instance)
	}

	return nil, nil
}

func (f *FakeClient) Scale(ctx context.Context, args client.ScaleArgs) error {
	if f.FakeScale != nil {
		return f.FakeScale(args)
	}

	return nil
}

func (f *FakeClient) UpdateCertificate(ctx context.Context, args client.UpdateCertificateArgs) error {
	if f.FakeUpdateCertificate != nil {
		return f.FakeUpdateCertificate(args)
	}

	return nil
}

func (f *FakeClient) DeleteCertificate(ctx context.Context, args client.DeleteCertificateArgs) error {
	if f.FakeDeleteCertificate != nil {
		return f.FakeDeleteCertificate(args)
	}

	return nil
}

func (f *FakeClient) UpdateBlock(ctx context.Context, args client.UpdateBlockArgs) error {
	if f.FakeUpdateBlock != nil {
		return f.FakeUpdateBlock(args)
	}

	return nil
}

func (f *FakeClient) DeleteBlock(ctx context.Context, args client.DeleteBlockArgs) error {
	if f.FakeDeleteBlock != nil {
		return f.FakeDeleteBlock(args)
	}

	return nil
}

func (f *FakeClient) ListBlocks(ctx context.Context, args client.ListBlocksArgs) ([]types.Block, error) {
	if f.FakeListBlocks != nil {
		return f.FakeListBlocks(args)
	}

	return nil, nil
}

func (f *FakeClient) DeleteRoute(ctx context.Context, args client.DeleteRouteArgs) error {
	if f.FakeDeleteRoute != nil {
		return f.FakeDeleteRoute(args)
	}

	return nil
}

func (f *FakeClient) ListRoutes(ctx context.Context, args client.ListRoutesArgs) ([]types.Route, error) {
	if f.FakeListRoutes != nil {
		return f.FakeListRoutes(args)
	}

	return nil, nil
}

func (f *FakeClient) UpdateRoute(ctx context.Context, args client.UpdateRouteArgs) error {
	if f.FakeUpdateRoute != nil {
		return f.FakeUpdateRoute(args)
	}

	return nil
}

func (f *FakeClient) Exec(ctx context.Context, args client.ExecArgs) (*websocket.Conn, error) {
	if f.FakeExec != nil {
		return f.FakeExec(ctx, args)
	}

	return nil, nil
}

func (f *FakeClient) SetService(service string) error {
	if f.FakeSetService != nil {
		return f.FakeSetService(service)
	}

	return nil
}
