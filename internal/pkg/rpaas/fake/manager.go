// Copyright 2019 tsuru authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package fake

import (
	"context"
	"crypto/tls"

	nginxv1alpha1 "github.com/tsuru/nginx-operator/api/v1alpha1"

	"github.com/tsuru/rpaas-operator/api/v1alpha1"
	"github.com/tsuru/rpaas-operator/internal/pkg/rpaas"
	clientTypes "github.com/tsuru/rpaas-operator/pkg/rpaas/client/types"
)

var _ rpaas.RpaasManager = &RpaasManager{}

type RpaasManager struct {
	FakeUpdateCertificate  func(instance, name string, cert tls.Certificate) error
	FakeGetCertificates    func(instanceName string) ([]rpaas.CertificateData, error)
	FakeDeleteCertificate  func(instance, name string) error
	FakeCreateInstance     func(args rpaas.CreateArgs) error
	FakeDeleteInstance     func(instanceName string) error
	FakeUpdateInstance     func(instanceName string, args rpaas.UpdateInstanceArgs) error
	FakeGetInstance        func(instanceName string) (*v1alpha1.RpaasInstance, error)
	FakeDeleteBlock        func(instanceName, blockName string) error
	FakeListBlocks         func(instanceName string) ([]rpaas.ConfigurationBlock, error)
	FakeUpdateBlock        func(instanceName string, block rpaas.ConfigurationBlock) error
	FakeInstanceAddress    func(name string) (string, error)
	FakeInstanceStatus     func(name string) (*nginxv1alpha1.Nginx, rpaas.PodStatusMap, error)
	FakeScale              func(instanceName string, replicas int32) error
	FakeGetPlans           func() ([]rpaas.Plan, error)
	FakeGetFlavors         func() ([]rpaas.Flavor, error)
	FakeCreateExtraFiles   func(instanceName string, files ...rpaas.File) error
	FakeDeleteExtraFiles   func(instanceName string, filenames ...string) error
	FakeGetExtraFiles      func(instanceName string) ([]rpaas.File, error)
	FakeUpdateExtraFiles   func(instanceName string, files ...rpaas.File) error
	FakeBindApp            func(instanceName string, args rpaas.BindAppArgs) error
	FakeUnbindApp          func(instanceName, appName string) error
	FakePurgeCache         func(instanceName string, args rpaas.PurgeCacheArgs) (int, error)
	FakeDeleteRoute        func(instanceName, path string) error
	FakeGetRoutes          func(instanceName string) ([]rpaas.Route, error)
	FakeUpdateRoute        func(instanceName string, route rpaas.Route) error
	FakeGetAutoscale       func(name string) (*clientTypes.Autoscale, error)
	FakeCreateAutoscale    func(instanceName string, autoscale *clientTypes.Autoscale) error
	FakeUpdateAutoscale    func(instanceName string, autoscale *clientTypes.Autoscale) error
	FakeDeleteAutoscale    func(name string) error
	FakeGetInstanceInfo    func(instanceName string) (*clientTypes.InstanceInfo, error)
	FakeExec               func(instanceName string, args rpaas.ExecArgs) error
	FakeAddAllowedUpstream func(instanceName string, upstream v1alpha1.RpaasAllowedUpstream) error
}

func (m *RpaasManager) GetInstanceInfo(ctx context.Context, instanceName string) (*clientTypes.InstanceInfo, error) {
	if m.FakeGetInstanceInfo != nil {
		return m.FakeGetInstanceInfo(instanceName)
	}
	return nil, nil
}

func (m *RpaasManager) GetCertificates(ctx context.Context, instanceName string) ([]rpaas.CertificateData, error) {
	if m.FakeGetCertificates != nil {
		return m.FakeGetCertificates(instanceName)
	}

	return nil, nil
}

func (m *RpaasManager) DeleteCertificate(ctx context.Context, instance, name string) error {
	if m.FakeDeleteCertificate != nil {
		return m.FakeDeleteCertificate(instance, name)
	}
	return nil
}

func (m *RpaasManager) UpdateCertificate(ctx context.Context, instance, name string, c tls.Certificate) error {
	if m.FakeUpdateCertificate != nil {
		return m.FakeUpdateCertificate(instance, name, c)
	}
	return nil
}

func (m *RpaasManager) CreateInstance(ctx context.Context, args rpaas.CreateArgs) error {
	if m.FakeCreateInstance != nil {
		return m.FakeCreateInstance(args)
	}
	return nil
}

func (m *RpaasManager) DeleteInstance(ctx context.Context, name string) error {
	if m.FakeDeleteInstance != nil {
		return m.FakeDeleteInstance(name)
	}
	return nil
}

func (m *RpaasManager) UpdateInstance(ctx context.Context, name string, args rpaas.UpdateInstanceArgs) error {
	if m.FakeUpdateInstance != nil {
		return m.FakeUpdateInstance(name, args)
	}
	return nil
}

func (m *RpaasManager) GetInstance(ctx context.Context, name string) (*v1alpha1.RpaasInstance, error) {
	if m.FakeGetInstance != nil {
		return m.FakeGetInstance(name)
	}
	return nil, nil
}

func (m *RpaasManager) DeleteBlock(ctx context.Context, instanceName, blockName string) error {
	if m.FakeDeleteBlock != nil {
		return m.FakeDeleteBlock(instanceName, blockName)
	}
	return nil
}

func (m *RpaasManager) ListBlocks(ctx context.Context, instanceName string) ([]rpaas.ConfigurationBlock, error) {
	if m.FakeListBlocks != nil {
		return m.FakeListBlocks(instanceName)
	}
	return nil, nil
}

func (m *RpaasManager) UpdateBlock(ctx context.Context, instanceName string, block rpaas.ConfigurationBlock) error {
	if m.FakeUpdateBlock != nil {
		return m.FakeUpdateBlock(instanceName, block)
	}
	return nil
}

func (m *RpaasManager) GetInstanceAddress(ctx context.Context, name string) (string, error) {
	if m.FakeInstanceAddress != nil {
		return m.FakeInstanceAddress(name)
	}
	return "", nil
}

func (m *RpaasManager) GetInstanceStatus(ctx context.Context, name string) (*nginxv1alpha1.Nginx, rpaas.PodStatusMap, error) {
	if m.FakeInstanceStatus != nil {
		return m.FakeInstanceStatus(name)
	}
	return nil, nil, nil
}

func (m *RpaasManager) Scale(ctx context.Context, instanceName string, replicas int32) error {
	if m.FakeScale != nil {
		return m.FakeScale(instanceName, replicas)
	}
	return nil
}

func (m *RpaasManager) GetPlans(ctx context.Context) ([]rpaas.Plan, error) {
	if m.FakeGetPlans != nil {
		return m.FakeGetPlans()
	}
	return nil, nil
}

func (m *RpaasManager) GetFlavors(ctx context.Context) ([]rpaas.Flavor, error) {
	if m.FakeGetFlavors != nil {
		return m.FakeGetFlavors()
	}
	return nil, nil
}

func (m *RpaasManager) CreateExtraFiles(ctx context.Context, instanceName string, files ...rpaas.File) error {
	if m.FakeCreateExtraFiles != nil {
		return m.FakeCreateExtraFiles(instanceName, files...)
	}
	return nil
}

func (m *RpaasManager) DeleteExtraFiles(ctx context.Context, instanceName string, filenames ...string) error {
	if m.FakeDeleteExtraFiles != nil {
		return m.FakeDeleteExtraFiles(instanceName, filenames...)
	}
	return nil
}

func (m *RpaasManager) GetExtraFiles(ctx context.Context, instanceName string) ([]rpaas.File, error) {
	if m.FakeGetExtraFiles != nil {
		return m.FakeGetExtraFiles(instanceName)
	}
	return nil, nil
}

func (m *RpaasManager) UpdateExtraFiles(ctx context.Context, instanceName string, files ...rpaas.File) error {
	if m.FakeUpdateExtraFiles != nil {
		return m.FakeUpdateExtraFiles(instanceName, files...)
	}
	return nil
}

func (m *RpaasManager) BindApp(ctx context.Context, instanceName string, args rpaas.BindAppArgs) error {
	if m.FakeBindApp != nil {
		return m.FakeBindApp(instanceName, args)
	}
	return nil
}

func (m *RpaasManager) UnbindApp(ctx context.Context, instanceName, appName string) error {
	if m.FakeUnbindApp != nil {
		return m.FakeUnbindApp(instanceName, appName)
	}
	return nil
}

func (m *RpaasManager) PurgeCache(ctx context.Context, instanceName string, args rpaas.PurgeCacheArgs) (int, error) {
	if m.FakePurgeCache != nil {
		return m.FakePurgeCache(instanceName, args)
	}
	return 0, nil
}

func (m *RpaasManager) DeleteRoute(ctx context.Context, instanceName, path string) error {
	if m.FakeDeleteRoute != nil {
		return m.FakeDeleteRoute(instanceName, path)
	}
	return nil
}

func (m *RpaasManager) GetRoutes(ctx context.Context, instanceName string) ([]rpaas.Route, error) {
	if m.FakeGetRoutes != nil {
		return m.FakeGetRoutes(instanceName)
	}
	return nil, nil
}

func (m *RpaasManager) UpdateRoute(ctx context.Context, instanceName string, route rpaas.Route) error {
	if m.FakeUpdateRoute != nil {
		return m.FakeUpdateRoute(instanceName, route)
	}
	return nil
}

func (m *RpaasManager) GetAutoscale(ctx context.Context, instanceName string) (*clientTypes.Autoscale, error) {
	if m.FakeGetAutoscale != nil {
		return m.FakeGetAutoscale(instanceName)
	}
	return nil, nil
}

func (m *RpaasManager) CreateAutoscale(ctx context.Context, instanceName string, autoscale *clientTypes.Autoscale) error {
	if m.FakeCreateAutoscale != nil {
		return m.FakeCreateAutoscale(instanceName, autoscale)
	}
	return nil
}

func (m *RpaasManager) UpdateAutoscale(ctx context.Context, instanceName string, autoscale *clientTypes.Autoscale) error {
	if m.FakeUpdateAutoscale != nil {
		return m.FakeUpdateAutoscale(instanceName, autoscale)
	}
	return nil
}

func (m *RpaasManager) DeleteAutoscale(ctx context.Context, instanceName string) error {
	if m.FakeDeleteAutoscale != nil {
		return m.FakeDeleteAutoscale(instanceName)
	}
	return nil
}

func (m *RpaasManager) Exec(ctx context.Context, instanceName string, args rpaas.ExecArgs) error {
	if m.FakeExec != nil {
		return m.FakeExec(instanceName, args)
	}
	return nil
}

func (m *RpaasManager) AddAllowedUpstream(ctx context.Context, instanceName string, upstream v1alpha1.RpaasAllowedUpstream) error {
	if m.FakeAddAllowedUpstream != nil {
		return m.FakeAddAllowedUpstream(instanceName, upstream)
	}
	return nil
}
