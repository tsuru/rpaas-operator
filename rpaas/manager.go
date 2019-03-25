package rpaas

import (
	"crypto/tls"
	"sync"

	"sigs.k8s.io/controller-runtime/pkg/client"
)

type RpaasManager interface {
	UpdateCertificate(string, *tls.Certificate) error
}

var rpaasManagerHolder = &struct {
	sync.Mutex
	m RpaasManager
}{}

func GetRpaasManager() RpaasManager {
	rpaasManagerHolder.Lock()
	defer rpaasManagerHolder.Unlock()
	return rpaasManagerHolder.m
}

func SetRpaasManager(m RpaasManager) {
	rpaasManagerHolder.Lock()
	defer rpaasManagerHolder.Unlock()
	rpaasManagerHolder.m = m
}

type k8sRpaasManager struct {
	c client.Client
}

func NewK8SRpaasManager(cli client.Client) RpaasManager {
	return &k8sRpaasManager{
		c: cli,
	}
}

func (m *k8sRpaasManager) UpdateCertificate(instance string, c *tls.Certificate) error {
	return nil
}
