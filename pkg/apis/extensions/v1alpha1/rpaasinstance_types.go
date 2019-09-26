// Copyright 2019 tsuru authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package v1alpha1

import (
	nginxv1alpha1 "github.com/tsuru/nginx-operator/pkg/apis/nginx/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// RpaasInstanceSpec defines the desired state of RpaasInstance
// +k8s:openapi-gen=true
type RpaasInstanceSpec struct {
	// Number of desired pods. This is a pointer to distinguish between explicit
	// zero and not specified. Defaults to 1.
	// +optional
	Replicas *int32 `json:"replicas,omitempty"`

	// PlanName is the name of the rpaasplan instance.
	PlanName string `json:"planName"`

	// PlanTemplate allow overriding fields in the specified plan.
	PlanTemplate *RpaasPlanSpec `json:"planTemplate,omitempty"`

	// Host is the application address where all incoming HTTP will be
	// forwarded for.
	// +optional
	Host string `json:"host,omitempty"`

	// Blocks are configuration file fragments added to the generated nginx
	// config.
	Blocks map[BlockType]Value `json:"blocks,omitempty"`

	// Locations hold paths that can be configured to forward resquests to
	// one destination app or include raw NGINX configurations itself.
	// +optional
	Locations []Location `json:"locations,omitempty"`

	// Certificates are a set of attributes that relate the certificate's
	// location in the cluster (Secret resource name) and its destination into
	// Pods.
	// +optional
	Certificates *nginxv1alpha1.TLSSecret `json:"certificates,omitempty"`

	// Service to expose the nginx instance
	// +optional
	Service *nginxv1alpha1.NginxService `json:"service,omitempty"`

	// ExtraFiles points to a ConfigMap where the files are stored.
	// +optional
	ExtraFiles *nginxv1alpha1.FilesRef `json:"extraFiles,omitempty"`

	// The number of old Configs to retain to allow rollback.
	// +optional
	ConfigHistoryLimit *int `json:"configHistoryLimit,omitempty"`

	// PodTemplate used to configure the NGINX pod template.
	// +optional
	PodTemplate nginxv1alpha1.NginxPodTemplateSpec `json:"podTemplate,omitempty"`

	// Autoscale holds the infos used to configure the HorizontalPodAutoscaler
	// for this instance.
	// +optional
	Autoscale *RpaasInstanceAutoscaleSpec `json:"autoscale,omitempty"`
}

// RpaasInstanceStatus defines the observed state of RpaasInstance
// +k8s:openapi-gen=true
type RpaasInstanceStatus struct{}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// RpaasInstance is the Schema for the rpaasinstances API
// +k8s:openapi-gen=true
type RpaasInstance struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Status RpaasInstanceStatus `json:"status,omitempty"`
	Spec   RpaasInstanceSpec   `json:"spec,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// RpaasInstanceList contains a list of RpaasInstance
type RpaasInstanceList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []RpaasInstance `json:"items"`
}

type BlockType string

const (
	BlockTypeRoot      = "root"
	BlockTypeHTTP      = "http"
	BlockTypeServer    = "server"
	BlockTypeLuaServer = "lua-server"
	BlockTypeLuaWorker = "lua-worker"
)

type Location struct {
	Path        string `json:"path"`
	Destination string `json:"destination,omitempty"`
	Content     *Value `json:"content,omitempty"`
	ForceHTTPS  bool   `json:"forceHTTPS,omitempty"`
}

type ValueSource struct {
	ConfigMapKeyRef *corev1.ConfigMapKeySelector `json:"configMapKeyRef,omitempty"`
	Namespace       string                       `json:"namespace,omitempty"`
}

type Value struct {
	Value     string       `json:"value,omitempty"`
	ValueFrom *ValueSource `json:"valueFrom,omitempty"`
}

const CertificateNameDefault = "default"

// RpaasInstanceAutoscaleSpec describes the behavior of HorizontalPodAutoscaler.
type RpaasInstanceAutoscaleSpec struct {
	// MaxReplicas is the upper limit for the number of replicas that can be set
	// by the HorizontalPodAutoscaler.
	MaxReplicas int32 `json:"maxReplicas"`
	// MinReplicas is the lower limit for the number of replicas that can be set
	// by the HorizontalPodAutoscaler.
	// Defaults to the RpaasInstance replicas value.
	// +optional
	MinReplicas *int32 `json:"minReplicas,omitempty"`
	// TargetCPUUtilizationPercentage is the target average CPU utilization over
	// all the pods. Represented as a percentage of requested CPU, e.g. int32(80)
	// equals to 80%.
	// +optional
	TargetCPUUtilizationPercentage *int32 `json:"targetCPUUtilizationPercentage,omitempty"`
	// TargetMemoryUtilizationPercentage is the target average memory utilization
	// over all the pods. Represented as a percentage of requested memory, e.g.
	// int32(80) equals to 80%.
	// +optional
	TargetMemoryUtilizationPercentage *int32 `json:"targetMemoryUtilizationPercentage,omitempty"`
}

func init() {
	SchemeBuilder.Register(&RpaasInstance{}, &RpaasInstanceList{})
}
