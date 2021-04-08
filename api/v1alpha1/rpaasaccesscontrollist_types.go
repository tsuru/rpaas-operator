// Copyright 2021 tsuru authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// RpaasAccessControlListSpec defines the desired state of RpaasAccessControlList
type RpaasAccessControlListSpec struct {
	Items []RpaasAccessControlListItem `json:"upstreams"`
}

type RpaasAccessControlListItem struct {
	Host string `json:"host"`
	Port *int   `json:"port,omitempty"`
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status

// RpaasAccessControlList is the Schema for the RpaasAccessControlList API
type RpaasAccessControlList struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec RpaasAccessControlListSpec `json:"spec,omitempty"`
}

//+kubebuilder:object:root=true

// RpaasACLList contains a list of RpaasAccessControlList
type RpaasACLList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []RpaasAccessControlList `json:"items"`
}

func init() {
	SchemeBuilder.Register(&RpaasAccessControlList{}, &RpaasACLList{})
}
