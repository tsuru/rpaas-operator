package fake

import (
	"context"
	"crypto/tls"

	"github.com/tsuru/rpaas-operator/pkg/apis/extensions/v1alpha1"
	"github.com/tsuru/rpaas-operator/rpaas"
)

type RpaasManager struct {
	FakeUpdateCertificate func(instance, name string, cert tls.Certificate) error
	FakeCreateInstance    func(args rpaas.CreateArgs) error
	FakeDeleteInstance    func(instanceName string) error
	FakeGetInstance       func(instanceName string) (*v1alpha1.RpaasInstance, error)
	FakeDeleteBlock       func(instanceName, blockName string) error
	FakeListBlocks        func(instanceName string) ([]rpaas.ConfigurationBlock, error)
	FakeUpdateBlock       func(instanceName string, block rpaas.ConfigurationBlock) error
	FakeInstanceAddress   func(name string) (string, error)
	FakeScale             func(instanceName string, replicas int32) error
	FakeGetPlans          func() ([]v1alpha1.RpaasPlan, error)
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

func (m *RpaasManager) Scale(ctx context.Context, instanceName string, replicas int32) error {
	if m.FakeScale != nil {
		return m.FakeScale(instanceName, replicas)
	}
	return nil
}

func (m *RpaasManager) GetPlans(ctx context.Context) ([]v1alpha1.RpaasPlan, error) {
	if m.FakeGetPlans != nil {
		return m.FakeGetPlans()
	}
	return nil, nil
}
