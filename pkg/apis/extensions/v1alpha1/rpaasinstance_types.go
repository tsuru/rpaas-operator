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

	// Host is the application address where all incoming HTTP will be
	// forwarded for.
	// +optional
	Host string `json:"host,omitempty"`

	// Blocks are configuration file fragments added to the generated nginx
	// config.
	Blocks map[BlockType]ConfigRef `json:"blocks,omitempty"`

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

	// The number of old ReplicaSets to retain to allow rollback.
	// +optional
	ConfigHistoryLimit *int32 `json:"ConfigHistoryLimit,omitempty"`
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

	Spec   RpaasInstanceSpec   `json:"spec,omitempty"`
	Status RpaasInstanceStatus `json:"status,omitempty"`
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
	BlockTypeHTTP          = "http"
	BlockTypeServer        = "server"
	BlockTypeRoot          = "root"
	BlockTypeHTTPDefault   = "http-default"
	BlockTypeServerDefault = "server-default"
	BlockTypeRootDefault   = "root-default"
)

type ConfigRef struct {
	// Name of config reference.
	Name string `json:"name"`
	// Kind of config referece.
	Kind ConfigKind `json:"kind"`
	// Value is optional and can be set for inline config kind.
	Value string `json:"value,omitempty"`
}

type ConfigKind string

type Location struct {
	Path        string       `json:"path"`
	Destination string       `json:"destination,omitempty"`
	ForceHTTPS  bool         `json:"forceHTTPS,omitempty"`
	Value       string       `json:"value,omitempty"`
	ValueFrom   *ValueSource `json:"valueFrom,omitempty"`
}

type ValueSource struct {
	ConfigMapKeyRef *corev1.ConfigMapKeySelector `json:"configMapKeyRef,omitempty"`
}

const (
	ConfigKindInline    = "Inline"
	ConfigKindConfigMap = "ConfigMap"
)

const CertificateNameDefault = "default"

func init() {
	SchemeBuilder.Register(&RpaasInstance{}, &RpaasInstanceList{})
}
