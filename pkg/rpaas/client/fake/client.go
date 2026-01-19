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

var _ client.Client = (*FakeClient)(nil)

type FakeClient struct {
	FakeGetPlans                func(instance string) ([]types.Plan, error)
	FakeGetFlavors              func(instance string) ([]types.Flavor, error)
	FakeScale                   func(args client.ScaleArgs) error
	FakeStart                   func(instance string) error
	FakeRestart                 func(instance string) error
	FakeStop                    func(instance string) error
	FakeUpdateCertificate       func(args client.UpdateCertificateArgs) error
	FakeDeleteCertificate       func(args client.DeleteCertificateArgs) error
	FakeUpdateBlock             func(args client.UpdateBlockArgs) error
	FakeDeleteBlock             func(args client.DeleteBlockArgs) error
	FakeListBlocks              func(args client.ListBlocksArgs) ([]types.Block, error)
	FakeDeleteRoute             func(args client.DeleteRouteArgs) error
	FakeListRoutes              func(args client.ListRoutesArgs) ([]types.Route, error)
	FakeUpdateRoute             func(args client.UpdateRouteArgs) error
	FakeInfo                    func(args client.InfoArgs) (*types.InstanceInfo, error)
	FakeExec                    func(ctx context.Context, args client.ExecArgs) (*websocket.Conn, error)
	FakeDebug                   func(ctx context.Context, args client.DebugArgs) (*websocket.Conn, error)
	FakeAddAccessControlList    func(instance, host string, port int) error
	FakeListAccessControlList   func(instance string) ([]types.AllowedUpstream, error)
	FakeRemoveAccessControlList func(instance, host string, port int) error
	FakeSetService              func(service string) error
	FakeListCertManagerRequests func(instance string) ([]types.CertManager, error)
	FakeUpdateCertManager       func(args client.UpdateCertManagerArgs) error
	FakeLog                     func(args client.LogArgs) error
	FakeAddExtraFiles           func(args client.ExtraFilesArgs) error
	FakeUpdateExtraFiles        func(args client.ExtraFilesArgs) error
	FakeDeleteExtraFiles        func(args client.DeleteExtraFilesArgs) error
	FakeListExtraFiles          func(args client.ListExtraFilesArgs) ([]types.RpaasFile, error)
	FakeGetExtraFile            func(args client.GetExtraFileArgs) (types.RpaasFile, error)
	FakeGetMetadata             func(instance string) (*types.Metadata, error)
	FakeSetMetadata             func(instance string, metadata *types.Metadata) error
	FakeUnsetMetadata           func(instance string, metadata *types.Metadata) error

	FakeDeleteCertManagerByName   func(instance, name string) error
	FakeDeleteCertManagerByIssuer func(instance, issuer string) error
	FakeListUpstreamOptions       func(args client.ListUpstreamOptionsArgs) ([]types.UpstreamOptions, error)
	FakeAddUpstreamOptions        func(args client.UpstreamOptionsArgs) error
	FakeUpdateUpstreamOptions     func(args client.UpstreamOptionsArgs) error
	FakeDeleteUpstreamOptions     func(args client.DeleteUpstreamOptionsArgs) error
	FakePurgeCache                func(args client.PurgeCacheArgs) (int, error)
	FakePurgeCacheBulk            func(args client.PurgeCacheBulkArgs) ([]client.PurgeBulkResult, error)
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

func (f *FakeClient) Start(ctx context.Context, instance string) error {
	if f.FakeStart != nil {
		return f.FakeStart(instance)
	}

	return nil
}

func (f *FakeClient) Restart(ctx context.Context, instance string) error {
	if f.FakeRestart != nil {
		return f.FakeRestart(instance)
	}

	return nil
}

func (f *FakeClient) Stop(ctx context.Context, instance string) error {
	if f.FakeStop != nil {
		return f.FakeStop(instance)
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

func (f *FakeClient) Debug(ctx context.Context, args client.DebugArgs) (*websocket.Conn, error) {
	if f.FakeDebug != nil {
		return f.FakeDebug(ctx, args)
	}

	return nil, nil
}

func (f *FakeClient) AddAccessControlList(ctx context.Context, instance, host string, port int) error {
	if f.FakeAddAccessControlList != nil {
		return f.FakeAddAccessControlList(instance, host, port)
	}
	return nil
}

func (f *FakeClient) ListAccessControlList(ctx context.Context, instance string) ([]types.AllowedUpstream, error) {
	if f.FakeListAccessControlList != nil {
		return f.FakeListAccessControlList(instance)
	}
	return nil, nil
}

func (f *FakeClient) RemoveAccessControlList(ctx context.Context, instance, host string, port int) error {
	if f.FakeRemoveAccessControlList != nil {
		return f.FakeRemoveAccessControlList(instance, host, port)
	}
	return nil
}

func (f *FakeClient) SetService(service string) (client.Client, error) {
	if f.FakeSetService != nil {
		return f, f.FakeSetService(service)
	}

	return f, nil
}

func (f *FakeClient) ListCertManagerRequests(ctx context.Context, instance string) ([]types.CertManager, error) {
	if f.FakeListCertManagerRequests != nil {
		return f.FakeListCertManagerRequests(instance)
	}

	return nil, nil
}

func (f *FakeClient) UpdateCertManager(ctx context.Context, args client.UpdateCertManagerArgs) error {
	if f.FakeUpdateCertManager != nil {
		return f.FakeUpdateCertManager(args)
	}

	return nil
}

func (f *FakeClient) DeleteCertManagerByName(ctx context.Context, instance, name string) error {
	if f.FakeDeleteCertManagerByName != nil {
		return f.FakeDeleteCertManagerByName(instance, name)
	}

	return nil
}

func (f *FakeClient) DeleteCertManagerByIssuer(ctx context.Context, instance, issuer string) error {
	if f.FakeDeleteCertManagerByName != nil {
		return f.FakeDeleteCertManagerByName(instance, issuer)
	}

	return nil
}

func (f *FakeClient) Log(ctx context.Context, args client.LogArgs) error {
	if f.FakeLog != nil {
		return f.FakeLog(args)
	}

	return nil
}

func (f *FakeClient) AddExtraFiles(ctx context.Context, args client.ExtraFilesArgs) error {
	if f.FakeAddExtraFiles != nil {
		return f.FakeAddExtraFiles(args)
	}

	return nil
}

func (f *FakeClient) UpdateExtraFiles(ctx context.Context, args client.ExtraFilesArgs) error {
	if f.FakeUpdateExtraFiles != nil {
		return f.FakeUpdateExtraFiles(args)
	}

	return nil
}

func (f *FakeClient) DeleteExtraFiles(ctx context.Context, args client.DeleteExtraFilesArgs) error {
	if f.FakeDeleteExtraFiles != nil {
		return f.FakeDeleteExtraFiles(args)
	}

	return nil
}

func (f *FakeClient) ListExtraFiles(ctx context.Context, args client.ListExtraFilesArgs) ([]types.RpaasFile, error) {
	if f.FakeListExtraFiles != nil {
		return f.FakeListExtraFiles(args)
	}

	return nil, nil
}

func (f *FakeClient) GetExtraFile(ctx context.Context, args client.GetExtraFileArgs) (types.RpaasFile, error) {
	if f.FakeGetExtraFile != nil {
		return f.FakeGetExtraFile(args)
	}

	return types.RpaasFile{}, nil
}

func (f *FakeClient) GetMetadata(ctx context.Context, instance string) (*types.Metadata, error) {
	if f.FakeGetMetadata != nil {
		return f.FakeGetMetadata(instance)
	}

	return nil, nil
}

func (f *FakeClient) SetMetadata(ctx context.Context, instance string, metadata *types.Metadata) error {
	if f.FakeSetMetadata != nil {
		return f.FakeSetMetadata(instance, metadata)
	}

	return nil
}

func (f *FakeClient) UnsetMetadata(tx context.Context, instance string, metadata *types.Metadata) error {
	if f.FakeUnsetMetadata != nil {
		return f.FakeUnsetMetadata(instance, metadata)
	}

	return nil
}

func (f *FakeClient) ListUpstreamOptions(ctx context.Context, args client.ListUpstreamOptionsArgs) ([]types.UpstreamOptions, error) {
	if f.FakeListUpstreamOptions != nil {
		return f.FakeListUpstreamOptions(args)
	}

	return nil, nil
}

func (f *FakeClient) AddUpstreamOptions(ctx context.Context, args client.UpstreamOptionsArgs) error {
	if f.FakeAddUpstreamOptions != nil {
		return f.FakeAddUpstreamOptions(args)
	}

	return nil
}

func (f *FakeClient) UpdateUpstreamOptions(ctx context.Context, args client.UpstreamOptionsArgs) error {
	if f.FakeUpdateUpstreamOptions != nil {
		return f.FakeUpdateUpstreamOptions(args)
	}

	return nil
}

func (f *FakeClient) DeleteUpstreamOptions(ctx context.Context, args client.DeleteUpstreamOptionsArgs) error {
	if f.FakeDeleteUpstreamOptions != nil {
		return f.FakeDeleteUpstreamOptions(args)
	}

	return nil
}

func (f *FakeClient) PurgeCache(ctx context.Context, args client.PurgeCacheArgs) (int, error) {
	if f.FakePurgeCache != nil {
		return f.FakePurgeCache(args)
	}

	return 0, nil
}

func (f *FakeClient) PurgeCacheBulk(ctx context.Context, args client.PurgeCacheBulkArgs) ([]client.PurgeBulkResult, error) {
	if f.FakePurgeCacheBulk != nil {
		return f.FakePurgeCacheBulk(args)
	}

	return nil, nil
}
