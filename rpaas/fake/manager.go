package fake

import (
	"crypto/tls"
)

type RpaasManager struct {
	FakeUpdateCertificate func(string, *tls.Certificate) error
}

func (m *RpaasManager) UpdateCertificate(instance string, c *tls.Certificate) error {
	if m.FakeUpdateCertificate != nil {
		return m.FakeUpdateCertificate(instance, c)
	}
	return nil
}
