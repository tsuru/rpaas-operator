// Copyright 2019 tsuru authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// RpaasFlavorSpec defines the desired state of RpaasFlavor
// +k8s:openapi-gen=true
type RpaasFlavorSpec struct {
	// Description provides a human readable description about this flavor.
	// +optional
	Description string `json:"description,omitempty"`

	// InstanceTemplate defines a template which allows to override the
	// associated RpaasInstance.
	// +optional
	InstanceTemplate *RpaasInstanceSpec `json:"instanceTemplate,omitempty"`

	// PlanSpecTemplate defines a template which allows to override the
	// associated RpaasInstance's RpaasPlan.
	// +optional
	PlanTemplate *RpaasPlanSpec `json:"planTemplate,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// RpaasFlavor is the Schema for the rpaasflavors API
// +k8s:openapi-gen=true
type RpaasFlavor struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec RpaasFlavorSpec `json:"spec,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// RpaasFlavorList contains a list of RpaasFlavor
type RpaasFlavorList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []RpaasFlavor `json:"items"`
}

func init() {
	SchemeBuilder.Register(&RpaasFlavor{}, &RpaasFlavorList{})
}
