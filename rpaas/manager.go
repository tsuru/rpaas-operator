package rpaas

import (
	"context"
	"crypto/tls"

	"github.com/tsuru/rpaas-operator/pkg/apis/extensions/v1alpha1"
)

type ConfigurationBlock struct {
	Name    string `form:"block_name" json:"block_name"`
	Content string `form:"content" json:"content"`
}

// ConfigurationBlockHandler defines some functions to handle the custom
// configuration blocks from an instance.
type ConfigurationBlockHandler interface {
	// DeleteBlock removes the configuration block named by blockName. It returns
	// a nil error meaning it was successful, otherwise a non-nil one which
	// describes the reached problem.
	DeleteBlock(ctx context.Context, instanceName, blockName string) error

	// ListBlocks returns all custom configuration blocks from instance (which
	// name is instanceName). It returns a nil error meaning it was successful,
	// otherwise a non-nil one which describes the reached problem.
	ListBlocks(ctx context.Context, instanceName string) ([]ConfigurationBlock, error)

	// UpdateBlock overwrites the older configuration block content with the one.
	// Whether the configuration block entry does not exist, it will already be
	// created with the new content. It returns a nil error meaning it was
	// successful, otherwise a non-nil one which describes the reached problem.
	UpdateBlock(ctx context.Context, instanceName string, block ConfigurationBlock) error
}

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
	ConfigurationBlockHandler

	UpdateCertificate(ctx context.Context, instance, name string, cert tls.Certificate) error
	CreateInstance(ctx context.Context, args CreateArgs) error
	DeleteInstance(ctx context.Context, name string) error
	GetInstance(ctx context.Context, name string) (*v1alpha1.RpaasInstance, error)
	GetInstanceAddress(ctx context.Context, name string) (string, error)
	Scale(ctx context.Context, name string, replicas int32) error
	GetPlans(ctx context.Context) ([]v1alpha1.RpaasPlan, error)
}
