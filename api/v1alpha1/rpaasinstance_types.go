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
	// +optional
	PlanName string `json:"planName"`

	// PlanNamespace is the namespace of target plan and their flavors, when empty uses the same namespace of instance.
	// +optional
	PlanNamespace string `json:"planNamespace"`

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

	// DNS Configuration for the current flavor
	// +optional
	DNS *DNSConfig `json:"dns,omitempty"`

	// TLS configuration.
	// +optional
	TLS []nginxv1alpha1.NginxTLS `json:"tls,omitempty"`

	// Service to expose the nginx instance
	// +optional
	Service *nginxv1alpha1.NginxService `json:"service,omitempty"`

	// ExtraFiles points to a ConfigMap where the files are stored.
	//
	// Deprecated: ExtraFiles stores all files in a single ConfigMap. In other
	// words, they share the limit of 1MiB due to etcd limitations. In order to
	// get around it, use the Field field.
	//
	// +optional
	ExtraFiles *nginxv1alpha1.FilesRef `json:"extraFiles,omitempty"`

	// Files is a list of regular files of general purpose to be mounted on
	// Nginx pods. As ConfigMap stores the file content, a file cannot exceed 1MiB.
	// +optional
	Files []File `json:"files,omitempty"`

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

	// Lifecycle describes actions that should be executed when
	// some event happens to nginx container.
	// +optional
	Lifecycle *nginxv1alpha1.NginxLifecycle `json:"lifecycle,omitempty"`

	// TLSSessionResumption configures the instance to support session resumption
	// using either session tickets or session ID (in the future). Defaults to
	// disabled.
	// +optional
	TLSSessionResumption *TLSSessionResumption `json:"tlsSessionResumption,omitempty"`

	// AllowedUpstreams holds the endpoints to which the RpaasInstance should be able to access
	// +optional
	AllowedUpstreams []AllowedUpstream `json:"allowedUpstreams,omitempty"`

	// DynamicCertificates enables automatic issuing and renewal for TLS certificates.
	// +optional
	DynamicCertificates *DynamicCertificates `json:"dynamicCertificates,omitempty"`

	// Ingress defines a minimal set of configurations to expose the instance over
	// an Ingress.
	// +optional
	Ingress *nginxv1alpha1.NginxIngress `json:"ingress,omitempty"`

	// EnablePodDisruptionBudget defines whether a PodDisruptionBudget should be attached
	// to Nginx or not. Defaults to disabled.
	//
	// If enabled, PDB's min available is calculated as:
	//  minAvailable = floor(N * 90%), where
	//  N:
	//   - rpaasinstance.spec.autoscale.minReplicas (if set and less than maxReplicas);
	//   - rpaasinstance.spec.autoscale.maxReplicas (if set);
	//   - rpaasinstance.spec.replicas (if set);
	//   - zero, otherwise.
	//
	// +optional
	EnablePodDisruptionBudget *bool `json:"enablePodDisruptionBudget,omitempty"`

	// ProxyProtocol defines whether allocate additional ports to expose via proxy protocol
	ProxyProtocol bool `json:"proxyProtocol,omitempty"`
}

type DynamicCertificates struct {
	// CertManager contains specific configurations to enable Cert Manager integration.
	// +optional
	CertManager *CertManager `json:"certManager,omitempty"`

	// CertManagerRequests is similar to CertManager field but for several requests.
	// +optional
	CertManagerRequests []CertManager `json:"certManagerRequests,omitempty"`
}

type CertManager struct {
	// Issuer refers either to Issuer or ClusterIssuer resource.
	//
	// NOTE: when there's no Issuer on this name, it tries using ClusterIssuer instead.
	Issuer string `json:"issuer,omitempty"`

	// DNSNames is a list of DNS names to be set in Subject Alternative Names.
	// +optional
	DNSNames []string `json:"dnsNames,omitempty"`

	// IPAddresses is a list of IP addresses to be set in Subject Alternative Names.
	// +optional
	IPAddresses []string `json:"ipAddresses,omitempty"`

	// DNSNamesDefault when is set use the provided DNSName from DNS Zone field.
	// +optional
	DNSNamesDefault bool `json:"dnsNamesDefault,omitempty"`
}

type AllowedUpstream struct {
	Host string `json:"host,omitempty"`
	Port int    `json:"port,omitempty"`
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

type DNSConfig struct {
	// Zone is the suffix which all DNS entries will be formed with
	// using the rule `instance_name.zone`
	Zone string `json:"zone"`
	// TTL is the DNS entry time to live in seconds (default is 60s)
	// +optional
	TTL *int32 `json:"ttl"`
}

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

type File struct {
	// Name is the filaname of the file.
	Name string `json:"name"`
	// ConfigMap is a reference to ConfigMap in the namespace that contains the
	// file content.
	ConfigMap *corev1.ConfigMapKeySelector `json:"configMap,omitempty"`
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
	WantedNginxRevisionHash string `json:"wantedNginxRevisionHash,omitempty"`

	// The revision hash observed by the controller in the nginx object.
	ObservedNginxRevisionHash string `json:"observedNginxRevisionHash,omitempty"`

	// PodSelector is the NGINX's pod label selector.
	PodSelector string `json:"podSelector,omitempty"`

	// The most recent generation observed by the rpaas operator controller.
	ObservedGeneration int64 `json:"observedGeneration,omitempty"`

	// CurrentReplicas is the last observed number of pods.
	CurrentReplicas int32 `json:"currentReplicas,omitempty"`

	// NginxUpdated is true if the wanted nginx revision hash equals the
	// observed nginx revision hash.
	NginxUpdated bool `json:"nginxUpdated"`
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
