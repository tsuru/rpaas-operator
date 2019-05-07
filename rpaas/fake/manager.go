package fake

import (
	"crypto/tls"

	"github.com/tsuru/rpaas-operator/pkg/apis/extensions/v1alpha1"
	"github.com/tsuru/rpaas-operator/rpaas"
)

type RpaasManager struct {
	FakeUpdateCertificate func(instance, name string, cert tls.Certificate) error
	FakeCreateInstance    func(args rpaas.CreateArgs) error
	FakeDeleteInstance    func(name string) error
	FakeGetInstance       func(name string) (*v1alpha1.RpaasInstance, error)
	FakeDeleteBlock       func(instanceName, blockName string) error
	FakeListBlocks        func(instanceName string) ([]rpaas.ConfigurationBlock, error)
	FakeUpdateBlock       func(instanceName string, block rpaas.ConfigurationBlock) error
	FakeInstanceAddress   func(name string) (string, error)
	FakeScale             func(instanceName string, replicas int32) error
}

func (m *RpaasManager) UpdateCertificate(instance, name string, c tls.Certificate) error {
	if m.FakeUpdateCertificate != nil {
		return m.FakeUpdateCertificate(instance, name, c)
	}
	return nil
}

func (m *RpaasManager) CreateInstance(args rpaas.CreateArgs) error {
	if m.FakeCreateInstance != nil {
		return m.FakeCreateInstance(args)
	}
	return nil
}

func (m *RpaasManager) DeleteInstance(name string) error {
	if m.FakeDeleteInstance != nil {
		return m.FakeDeleteInstance(name)
	}
	return nil
}

func (m *RpaasManager) GetInstance(name string) (*v1alpha1.RpaasInstance, error) {
	if m.FakeGetInstance != nil {
		return m.FakeGetInstance(name)
	}
	return nil, nil
}

func (m *RpaasManager) DeleteBlock(instanceName, blockName string) error {
	if m.FakeDeleteBlock != nil {
		return m.FakeDeleteBlock(instanceName, blockName)
	}
	return nil
}

func (m *RpaasManager) ListBlocks(instanceName string) ([]rpaas.ConfigurationBlock, error) {
	if m.FakeListBlocks != nil {
		return m.FakeListBlocks(instanceName)
	}
	return nil, nil
}

func (m *RpaasManager) UpdateBlock(instanceName string, block rpaas.ConfigurationBlock) error {
	if m.FakeUpdateBlock != nil {
		return m.FakeUpdateBlock(instanceName, block)
	}
	return nil
}

func (m *RpaasManager) GetInstanceAddress(name string) (string, error) {
	if m.FakeInstanceAddress != nil {
		return m.FakeInstanceAddress(name)
	}
	return "", nil
}

func (m *RpaasManager) Scale(instanceName string, replicas int32) error {
	if m.FakeScale != nil {
		return m.FakeScale(instanceName, replicas)
	}
	return nil
}
