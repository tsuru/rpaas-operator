// Copyright 2020 tsuru authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package v1alpha1

import (
	nginxv1alpha1 "github.com/tsuru/nginx-operator/api/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// RpaasInstanceSpec defines the desired state of RpaasInstance
type RpaasInstanceSpec struct {
	// Number of desired pods. This is a pointer to distinguish between explicit
	// zero and not specified. Defaults to 1.
	// +optional
	Replicas *int32 `json:"replicas,omitempty"`

	// PlanName is the name of the rpaasplan instance.
	PlanName string `json:"planName"`

	// Flavors are references to RpaasFlavors resources. When provided, each flavor
	// merges its instance template spec with this instance spec.
	// +optional
	Flavors []string `json:"flavors,omitempty"`

	// PlanTemplate allow overriding fields in the specified plan.
	PlanTemplate *RpaasPlanSpec `json:"planTemplate,omitempty"`

	// Binds is the list of apps bounded to the instance
	Binds []Bind `json:"binds,omitempty"`

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

	// AllocateContainerPorts enabled causes the operator to allocate a random
	// port to be used as container ports for nginx containers. This is useful
	// when combined with HostNetwork config to avoid conflicts between
	// multiple nginx instances.
	// +optional
	AllocateContainerPorts *bool `json:"allocateContainerPorts,omitempty"`

	// Autoscale holds the infos used to configure the HorizontalPodAutoscaler
	// for this instance.
	// +optional
	Autoscale *RpaasInstanceAutoscaleSpec `json:"autoscale,omitempty"`

	// Lifecycle describes actions that should be executed when
	// some event happens to nginx container.
	// +optional
	Lifecycle *nginxv1alpha1.NginxLifecycle `json:"lifecycle,omitempty"`

	// TLSSessionResumption configures the instance to support session resumption
	// using either session tickets or session ID (in the future). Defaults to
	// disabled.
	// +optional
	TLSSessionResumption *TLSSessionResumption `json:"tlsSessionResumption,omitempty"`

	// RolloutNginxOnce causes a rollout of the nginx object if changes are
	// detected only once. After updating the nginx object the controller will
	// set this value to false.
	// +optional
	RolloutNginxOnce bool `json:"rolloutNginxOnce,omitempty"`

	// RolloutNginx causes the controller to always update the nginx object
	// regardless of the default behavior on the controller.
	// +optional
	RolloutNginx bool `json:"rolloutNginx,omitempty"`
}

type Bind struct {
	Name string `json:"name"`
	Host string `json:"host"`
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

type TLSSessionResumption struct {
	// SessionTicket defines the parameters to set the TLS session tickets.
	// +optional
	SessionTicket *TLSSessionTicket `json:"sessionTicket,omitempty"`
}

const (
	// DefaultSessionTicketKeyRotationInteval holds the default time interval to
	// rotate the session tickets: 1 hour.
	DefaultSessionTicketKeyRotationInteval uint32 = 60
)

type SessionTicketKeyLength uint16

const (
	// SessionTicketKeyLength48 represents 48 bytes of session ticket key length.
	SessionTicketKeyLength48 = SessionTicketKeyLength(48)

	// SessionTicketKeyLength80 represents 80 bytes of session ticket key length.
	SessionTicketKeyLength80 = SessionTicketKeyLength(80)

	// DefaultSessionTicketKeyLength holds the default session ticket key length.
	DefaultSessionTicketKeyLength = SessionTicketKeyLength48
)

type TLSSessionTicket struct {
	// KeepLastKeys defines how many session ticket encryption keys should be
	// kept in addition to the current one. Zero means no old encryption keys.
	// Defaults to zero.
	// +optional
	KeepLastKeys uint32 `json:"keepLastKeys,omitempty"`

	// KeyRotationInterval defines the time interval, in minutes, that a
	// key rotation job should occurs. Defaults to 60 minutes (an hour).
	// +optional
	KeyRotationInterval uint32 `json:"keyRotationInterval,omitempty"`

	// KeyLength defines the length of bytes for a session tickets. Should be
	// either 48 or 80 bytes. Defaults to 48 bytes.
	// +optional
	KeyLength SessionTicketKeyLength `json:"keyLength,omitempty"`

	// Image is the container image name used to execute the session ticket
	// rotation job. It requires either "bash", "base64", "openssl" and "kubectl"
	// programs be installed into. Defaults to "bitnami/kubectl:latest".
	// +optional
	Image string `json:"image,omitempty"`
}

// RpaasInstanceStatus defines the observed state of RpaasInstance
type RpaasInstanceStatus struct {
	// Revision hash calculated for the current spec.
	// +optional
	WantedNginxRevisionHash string `json:"wantedNginxRevisionHash,omitempty"`

	// The revision hash observed by the controller in the nginx object.
	// +optional
	ObservedNginxRevisionHash string `json:"observedNginxRevisionHash,omitempty"`

	// The most recent generation observed by the rpaas operator controller.
	// +optional
	ObservedGeneration int64 `json:"observedGeneration,omitempty"`

	// CurrentReplicas is the last observed number of pods.
	CurrentReplicas int32 `json:"currentReplicas,omitempty"`

	// PodSelector is the NGINX's pod label selector.
	PodSelector string `json:"podSelector,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:subresource:scale:specpath=.spec.replicas,statuspath=.status.currentReplicas,selectorpath=.status.podSelector

// RpaasInstance is the Schema for the rpaasinstances API
type RpaasInstance struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Status RpaasInstanceStatus `json:"status,omitempty"`
	Spec   RpaasInstanceSpec   `json:"spec,omitempty"`
}

// +kubebuilder:object:root=true

// RpaasInstanceList contains a list of RpaasInstance
type RpaasInstanceList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []RpaasInstance `json:"items"`
}

func init() {
	SchemeBuilder.Register(&RpaasInstance{}, &RpaasInstanceList{})
}
