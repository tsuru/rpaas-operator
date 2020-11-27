// Copyright 2020 tsuru authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

// RpaasPortAllocationSpec defines the state of port allocations
type RpaasPortAllocationSpec struct {
	Ports []AllocatedPort `json:"ports,omitempty"`
}

type AllocatedPort struct {
	Port  int32           `json:"port,omitempty"`
	Owner NamespacedOwner `json:"owner,omitempty"`
}

type NamespacedOwner struct {
	Namespace string    `json:"namespace,omitempty"`
	RpaasName string    `json:"rpaasName,omitempty"`
	UID       types.UID `json:"uid,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:resource:scope=Cluster

// RpaasPortAllocation is the Schema for the Rpaasportallocations API
type RpaasPortAllocation struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec RpaasPortAllocationSpec `json:"spec,omitempty"`
}

// +kubebuilder:object:root=true

// NginxList contains a list of Nginx
type RpaasPortAllocationList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []RpaasPortAllocation `json:"items"`
}

func init() {
	SchemeBuilder.Register(&RpaasPortAllocation{}, &RpaasPortAllocationList{})
}
