package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

type RpaasInstanceList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata"`
	Items           []RpaasInstance `json:"items"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

type RpaasInstance struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata"`
	Spec              RpaasInstanceSpec   `json:"spec"`
	Status            RpaasInstanceStatus `json:"status,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

type RpaasPlanList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata"`
	Items           []RpaasPlan `json:"items"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

type RpaasPlan struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata"`
	Spec              RpaasPlanSpec `json:"spec"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

type RpaasBindList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata"`
	Items           []RpaasBind `json:"items"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

type RpaasBind struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata"`
	Spec              RpaasBindSpec `json:"spec"`
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

type RpaasInstanceSpec struct {
	// Number of desired pods. This is a pointer to distinguish between explicit
	// zero and not specified. Defaults to 1.
	// +optional
	Replicas *int32 `json:"replicas,omitempty"`

	// PlanName is the name of the rpaasplan instance.
	PlanName string `json:"planName"`

	// Blocks are configuration file fragments added to the generated nginx
	// config.
	Blocks map[BlockType]ConfigRef `json:"blocks,omitempty"`

	// Locations are configuration file fragments used as location blocks in
	// nginx config.
	Locations []Location `json:"locations,omitempty"`
}

type ConfigRef struct {
	// Name of config reference.
	Name string `json:"name"`

	// Kind of config referece.
	Kind ConfigKind `json:"kind"`

	// Value is optional and can be set for inline config kind.
	Value string `json:"value,omitempty"`
}

type ConfigKind string

const (
	ConfigKindInline    = "Inline"
	ConfigKindConfigMap = "ConfigMap"
)

type Location struct {
	Config      ConfigRef `json:"config"`
	Destination string    `json:"destination,omitempty"`
}

type RpaasInstanceStatus struct {
	// Fill me
}

type RpaasBindSpec struct {
}

type RpaasPlanSpec struct {
	Image  string      `json:"image"`
	Config NginxConfig `json:"config"`
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
