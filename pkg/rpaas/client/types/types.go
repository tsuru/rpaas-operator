// Copyright 2019 tsuru authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package types

import (
	"github.com/tsuru/rpaas-operator/pkg/apis/extensions/v1alpha1"
)

type Block struct {
	Name    string `json:"block_name"`
	Content string `json:"content"`
}

type Route struct {
	Path        string `json:"path"`
	Destination string `json:"destination,omitempty"`
	HTTPSOnly   bool   `json:"https_only,omitempty"`
	Content     string `json:"content,omitempty"`
}

type Autoscale struct {
	MinReplicas *int32 `json:"minReplicas,omitempty" form:"min"`
	MaxReplicas *int32 `json:"maxReplicas,omitempty" form:"max"`
	CPU         *int32 `json:"cpu,omitempty" form:"cpu"`
	Memory      *int32 `json:"memory,omitempty" form:"memory"`
}

type Flavor struct {
	Name        string `json:"name"`
	Description string `json:"description"`
}

type Plan struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Default     bool   `json:"default"`
}

type InstanceAddress struct {
	Hostname string `json:"hostname,omitempty"`
	IP       string `json:"ip,omitempty"`
}

type InstanceInfo struct {
	Addresses   []InstanceAddress `json:"addresses,omitempty"`
	Replicas    *int32            `json:"replicas,omitempty"`
	Plan        string            `json:"plan,omitempty"`
	Routes      []Route           `json:"routes,omitempty"`
	Autoscale   *Autoscale        `json:"autoscale,omitempty"`
	Binds       []v1alpha1.Bind   `json:"binds,omitempty"`
	Team        string            `json:"team,omitempty"`
	Name        string            `json:"name,omitempty"`
	Description string            `json:"description,omitempty"`
	Tags        []string          `json:"tags,omitempty"`
}
