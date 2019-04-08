package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// RpaasPlanSpec defines the desired state of RpaasPlan
// +k8s:openapi-gen=true
type RpaasPlanSpec struct {
	// INSERT ADDITIONAL SPEC FIELDS - desired state of cluster
	// Important: Run "operator-sdk generate k8s" to regenerate code after modifying this file
	Image  string      `json:"image"`
	Config NginxConfig `json:"config"`

	// Add custom validation using kubebuilder tags: https://book.kubebuilder.io/beyond_basics/generating_crd.html
}

// RpaasPlanStatus defines the observed state of RpaasPlan
// +k8s:openapi-gen=true
type RpaasPlanStatus struct {
	// INSERT ADDITIONAL STATUS FIELD - define observed state of cluster
	// Important: Run "operator-sdk generate k8s" to regenerate code after modifying this file
	// Add custom validation using kubebuilder tags: https://book.kubebuilder.io/beyond_basics/generating_crd.html
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// RpaasPlan is the Schema for the rpaasplans API
// +k8s:openapi-gen=true
type RpaasPlan struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   RpaasPlanSpec   `json:"spec,omitempty"`
	Status RpaasPlanStatus `json:"status,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// RpaasPlanList contains a list of RpaasPlan
type RpaasPlanList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []RpaasPlan `json:"items"`
}

type NginxConfig struct {
	User string `json:"user,omitempty"`

	RequestIDEnabled bool `json:"requestIDEnabled,omitempty"`

	CacheEnabled     bool   `json:"cacheEnabled,omitempty"`
	CacheInactive    string `json:"cacheInactive,omitempty"`
	CacheLoaderFiles int    `json:"cacheLoaderFiles,omitempty"`
	CachePath        string `json:"cachePath,omitempty"`
	CacheSize        string `json:"cacheSize,omitempty"`
	CacheZoneSize    string `json:"cacheZoneSize,omitempty"`

	HTTPListenOptions  string `json:"httpListenOptions,omitempty"`
	HTTPSListenOptions string `json:"httpsListenOptions,omitempty"`

	VTSEnabled bool `json:"vtsEnabled,omitempty"`

	SyslogEnabled       bool   `json:"syslogEnabled,omitempty"`
	SyslogServerAddress string `json:"syslogServerAddress,omitempty"`
	SyslogFacility      string `json:"syslogFacility,omitempty"`
	SyslogTag           string `json:"syslogTag,omitempty"`

	WorkerProcesses   int `json:"workerProcesses,omitempty"`
	WorkerConnections int `json:"workerConnections,omitempty"`
}

func init() {
	SchemeBuilder.Register(&RpaasPlan{}, &RpaasPlanList{})
}
