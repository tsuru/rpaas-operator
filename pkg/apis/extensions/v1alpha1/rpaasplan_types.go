package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// RpaasPlanSpec defines the desired state of RpaasPlan
type RpaasPlanSpec struct {
	// INSERT ADDITIONAL SPEC FIELDS - desired state of cluster
	// Important: Run "operator-sdk generate k8s" to regenerate code after modifying this file
	Image  string      `json:"image"`
	Config NginxConfig `json:"config"`
}

// RpaasPlanStatus defines the observed state of RpaasPlan
type RpaasPlanStatus struct {
	// INSERT ADDITIONAL STATUS FIELD - define observed state of cluster
	// Important: Run "operator-sdk generate k8s" to regenerate code after modifying this file
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
	User              string              `json:"user,omitempty"`
	WorkerProcesses   int                 `json:"workerProcesses,omitempty"`
	WorkerConnections int                 `json:"workerConnections,omitempty"`
	RequestIDEnabled  bool                `json:"requestIdEnabled,omitempty"`
	LocalLog          bool                `json:"localLog,omitempty"`
	SyslogServer      string              `json:"syslogServer,omitempty"`
	SyslogTag         string              `json:"syslogTag,omitempty"`
	CustomErrorCodes  map[string][]string `json:"customErrorCodes,omitempty"`

	KeyZoneSize   string `json:"keyZoneSize,omitempty"`
	CacheInactive string `json:"cacheInactive,omitempty"`
	CacheSize     string `json:"cacheSize,omitempty"`
	LoaderFiles   int    `json:"loaderFiles,omitempty"`

	VtsEnabled bool `json:"vtsEnabled,omitempty"`
	Lua        bool `json:"lua,omitempty"`

	AdminListen        string `json:"adminListen,omitempty"`
	AdminEnableSsl     bool   `json:"adminEnableSsl,omitempty"`
	AdminLocationPurge bool   `json:"adminLocationPurge,omitempty"`

	Listen        string `json:"listen,omitempty"`
	ListenBacklog int    `json:"listenBacklog,omitempty"`

	DisableResponseRequestID bool `json:"disableResponseRequestId,omitempty"`

	CustomErrorDir  string `json:"customErrorDir,omitempty"`
	InterceptErrors bool   `json:"interceptErrors,omitempty"`
}

func init() {
	SchemeBuilder.Register(&RpaasPlan{}, &RpaasPlanList{})
}
