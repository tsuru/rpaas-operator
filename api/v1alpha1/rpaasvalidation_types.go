// Copyright 2024 tsuru authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// +kubebuilder:object:root=true
// +kubebuilder:resource:shortName=rpaas-validation
// +kubebuilder:subresource:status
// RpaasInstance is the Schema for the rpaasinstances API
type RpaasValidation struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Status RpaasValidationStatus `json:"status,omitempty"`
	// Spec reuse the same properties of RpaasInstance, just to avoid duplication of code
	Spec RpaasInstanceSpec `json:"spec,omitempty"`
}

// RpaasValidationStatus defines the observed state of RpaasValidation
type RpaasValidationStatus struct {
	//Revision hash calculated for the current spec of rpaasvalidation
	RevisionHash string `json:"revisionHash,omitempty"`

	// The most recent generation observed by the rpaas operator controller.
	ObservedGeneration int64 `json:"observedGeneration,omitempty"`

	// Valid determines whether validation is valid
	Valid *bool `json:"valid,omitempty"`

	// Feedback of validation of nginx
	Error string `json:"error,omitempty"`
}

// +kubebuilder:object:root=true

// RpaasValidationList contains a list of RpaasInstance
type RpaasValidationList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []RpaasValidation `json:"items"`
}

func init() {
	SchemeBuilder.Register(&RpaasValidation{}, &RpaasValidationList{})
}
