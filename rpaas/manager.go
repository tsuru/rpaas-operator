package rpaas

import (
	"crypto/tls"

	"github.com/tsuru/rpaas-operator/pkg/apis/extensions/v1alpha1"
)

type CreateArgs struct {
	Name        string   `json:"name" form:"name"`
	Plan        string   `json:"plan" form:"plan"`
	Team        string   `json:"team" form:"team"`
	User        string   `json:"user" form:"user"`
	Tags        []string `json:"tags" form:"tags"`
	EventID     string   `json:"eventid" form:"eventid"`
	Description string   `json:"description" form:"description"`
	Flavor      string   `json:"flavor" form:"flavor"`
	IP          string   `json:"ip" form:"ip"`
}

type RpaasManager interface {
	UpdateCertificate(instance string, cert tls.Certificate) error
	CreateInstance(args CreateArgs) error
	DeleteInstance(name string) error
	GetInstance(name string) (*v1alpha1.RpaasInstance, error)
	DeleteBlock(instanceName, block string) error
	ListBlocks(instanceName string) (map[string]string, error)
	UpdateBlock(instanceName, block, content string) error
}
