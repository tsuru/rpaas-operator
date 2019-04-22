package fake

import (
	"crypto/tls"

	"github.com/tsuru/rpaas-operator/pkg/apis/extensions/v1alpha1"
	"github.com/tsuru/rpaas-operator/rpaas"
)

type RpaasManager struct {
	FakeUpdateCertificate func(string, tls.Certificate) error
	FakeCreateInstance    func(args rpaas.CreateArgs) error
	FakeDeleteInstance    func(name string) error
	FakeGetInstance       func(name string) (*v1alpha1.RpaasInstance, error)
	FakeGetPlan           func(name string) (*v1alpha1.RpaasPlan, error)
}

func (m *RpaasManager) UpdateCertificate(instance string, c tls.Certificate) error {
	if m.FakeUpdateCertificate != nil {
		return m.FakeUpdateCertificate(instance, c)
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

func (m *RpaasManager) GetPlan(name string) (*v1alpha1.RpaasPlan, error) {
	if m.FakeGetPlan != nil {
		return m.FakeGetPlan(name)
	}
	return nil, nil
}
