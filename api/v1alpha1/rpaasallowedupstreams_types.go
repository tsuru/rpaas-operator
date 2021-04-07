// Copyright 2021 tsuru authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// RpaasAllowedUpstreamsSpec defines the desired state of RpaasAllowedUpstreams
type RpaasAllowedUpstreamsSpec struct {
	Upstreams []RpaasAllowedUpstream `json:"upstreams"`
}

type RpaasAllowedUpstream struct {
	Host string `json:"host"`
	Port *int   `json:"port,omitempty"`
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status

// RpaasAllowedUpstreams is the Schema for the rpaasallowedupstreams API
type RpaasAllowedUpstreams struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec RpaasAllowedUpstreamsSpec `json:"spec,omitempty"`
}

//+kubebuilder:object:root=true

// RpaasAllowedUpstreamsList contains a list of RpaasAllowedUpstreams
type RpaasAllowedUpstreamsList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []RpaasAllowedUpstreams `json:"items"`
}

func init() {
	SchemeBuilder.Register(&RpaasAllowedUpstreams{}, &RpaasAllowedUpstreamsList{})
}
