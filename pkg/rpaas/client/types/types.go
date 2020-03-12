// Copyright 2019 tsuru authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package types

import "github.com/tsuru/rpaas-operator/pkg/apis/extensions/v1alpha1"

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
	Ip       string `json:"ip,omitempty"`
}

type InstanceInfo struct {
	Address     []InstanceAddress                    `json:"address,omitempty"`
	Replicas    *int32                               `json:"replicas,omitempty"`
	Plan        string                               `json:"plan,omitempty"`
	Locations   []v1alpha1.Location                  `json:"locations,omitempty"`
	Autoscale   *v1alpha1.RpaasInstanceAutoscaleSpec `json:"autoscale,omitempty"`
	Binds       []v1alpha1.Bind                      `json:"binds,omitempty"`
	Team        string                               `json:"team,omitempty"`
	Name        string                               `json:"name,omitempty"`
	Description string                               `json:"description,omitempty"`
	Tags        []string                             `json:"tags,omitempty"`
}
