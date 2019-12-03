// Copyright 2019 tsuru authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package v1alpha1

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// RpaasPlanSpec defines the desired state of RpaasPlan
// +k8s:openapi-gen=true
type RpaasPlanSpec struct {
	// Image is the NGINX container image name. Defaults to Nginx image value.
	// +optional
	Image string `json:"image,omitempty"`
	// Config defines some NGINX configurations values that can be used in the
	// configuration template.
	// +optional
	Config NginxConfig `json:"config,omitempty"`
	// Template contains the main NGINX configuration template.
	// +optional
	Template *Value `json:"template,omitempty"`
	// Description describes the plan.
	// +optional
	Description string `json:"description,omitempty"`
	// Default indicates whether plan is default.
	// +optional
	Default bool `json:"default,omitempty"`
	// Resources requirements to be set on the NGINX container.
	// +optional
	Resources corev1.ResourceRequirements `json:"resources,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// RpaasPlan is the Schema for the rpaasplans API
// +k8s:openapi-gen=true
type RpaasPlan struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec RpaasPlanSpec `json:"spec,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// RpaasPlanList contains a list of RpaasPlan
type RpaasPlanList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`

	Items []RpaasPlan `json:"items"`
}

type NginxConfig struct {
	User string `json:"user,omitempty"`

	UpstreamKeepalive int `json:"upstreamKeepAlive,omitempty"`

	CacheEnabled     *bool  `json:"cacheEnabled,omitempty"`
	CacheInactive    string `json:"cacheInactive,omitempty"`
	CacheLoaderFiles int    `json:"cacheLoaderFiles,omitempty"`
	CachePath        string `json:"cachePath,omitempty"`
	CacheSize        string `json:"cacheSize,omitempty"`
	CacheZoneSize    string `json:"cacheZoneSize,omitempty"`

	HTTPListenOptions  string `json:"httpListenOptions,omitempty"`
	HTTPSListenOptions string `json:"httpsListenOptions,omitempty"`

	VTSEnabled                *bool  `json:"vtsEnabled,omitempty"`
	VTSStatusHistogramBuckets string `json:"vtsStatusHistogramBuckets,omitempty"`

	SyslogEnabled       *bool  `json:"syslogEnabled,omitempty"`
	SyslogServerAddress string `json:"syslogServerAddress,omitempty"`
	SyslogFacility      string `json:"syslogFacility,omitempty"`
	SyslogTag           string `json:"syslogTag,omitempty"`

	WorkerProcesses   int `json:"workerProcesses,omitempty"`
	WorkerConnections int `json:"workerConnections,omitempty"`
}

func Bool(v bool) *bool {
	return &v
}

func BoolValue(v *bool) bool {
	if v == nil {
		return false
	}
	return *v
}

func init() {
	SchemeBuilder.Register(&RpaasPlan{}, &RpaasPlanList{})
}
