// Copyright 2024 tsuru authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package v1alpha1

import (
	nginxv1alpha1 "github.com/tsuru/nginx-operator/api/v1alpha1"
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
	Spec   RpaasValidationSpec   `json:"spec,omitempty"`
}

// RpaasValidationSpec defines the desired state of RpaasInstance
type RpaasValidationSpec struct {
	// Flavors are references to RpaasFlavors resources. When provided, each flavor
	// merges its instance template spec with this instance spec.
	// +optional
	Flavors []string `json:"flavors,omitempty"`

	// PlanTemplate allow overriding fields in the specified plan.
	// +optional
	PlanTemplate *RpaasPlanSpec `json:"planTemplate,omitempty"`

	// Binds is the list of apps bounded to the instance
	// +optional
	Binds []Bind `json:"binds,omitempty"`

	// Blocks are configuration file fragments added to the generated nginx
	// config.
	Blocks map[BlockType]Value `json:"blocks,omitempty"`

	// Locations hold paths that can be configured to forward resquests to
	// one destination app or include raw NGINX configurations itself.
	// +optional
	Locations []Location `json:"locations,omitempty"`

	// TLS configuration.
	// +optional
	TLS []nginxv1alpha1.NginxTLS `json:"tls,omitempty"`

	// Files is a list of regular files of general purpose to be mounted on
	// Nginx pods. As ConfigMap stores the file content, a file cannot exceed 1MiB.
	// +optional
	Files []File `json:"files,omitempty"`

	// PodTemplate used to configure the NGINX pod template.
	// +optional
	PodTemplate nginxv1alpha1.NginxPodTemplateSpec `json:"podTemplate,omitempty"`

	// DynamicCertificates enables automatic issuing and renewal for TLS certificates.
	// +optional
	DynamicCertificates *DynamicCertificates `json:"dynamicCertificates,omitempty"`
}

// RpaasValidationStatus defines the observed state of RpaasValidation
type RpaasValidationStatus struct {
	//Revision hash calculated for the current spec of rpaasvalidation
	RevisionHash string `json:"revisionHash,omitempty"`

	// The most recent generation observed by the rpaas operator controller.
	ObservedGeneration int64 `json:"observedGeneration,omitempty"`

	// Valid determines whether validation is valid
	Valid bool `json:"valid"`

	// Feedback of validation of nginx
	Error string `json:"error"`
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
