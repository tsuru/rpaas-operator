// Copyright 2020 tsuru authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// RpaasFlavorSpec defines the desired state of RpaasFlavor
type RpaasFlavorSpec struct {
	// Description provides a human readable description about this flavor.
	// +optional
	Description string `json:"description,omitempty"`

	// InstanceTemplate defines a template which allows to override the
	// associated RpaasInstance.
	// +optional
	InstanceTemplate *RpaasInstanceSpec `json:"instanceTemplate,omitempty"`

	// Default defines if the flavor should be applied by default on
	// every service instance. Default flavors cannot be listed on RpaasFlavorList.
	// +optional
	Default bool `json:"default,omitempty"`
}

// +kubebuilder:object:root=true

// RpaasFlavor is the Schema for the rpaasflavors API
// +k8s:openapi-gen=true
type RpaasFlavor struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec RpaasFlavorSpec `json:"spec,omitempty"`
}

// +kubebuilder:object:root=true

// RpaasFlavorList contains a list of RpaasFlavor
type RpaasFlavorList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []RpaasFlavor `json:"items"`
}

func init() {
	SchemeBuilder.Register(&RpaasFlavor{}, &RpaasFlavorList{})
}
