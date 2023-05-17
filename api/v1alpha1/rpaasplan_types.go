// Copyright 2020 tsuru authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package v1alpha1

import (
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// RpaasPlanSpec defines the desired state of RpaasPlan
type RpaasPlanSpec struct {
	// Image is the NGINX container image name. Defaults to Nginx image value.
	// +optional
	Image string `json:"image,omitempty"`
	// Config defines some NGINX configurations values that can be used in the
	// configuration template.
	// +optional
	Config NginxConfig `json:"config,omitempty"`
	// Template contains the main NGINX configuration template.
	// +optional
	Template *Value `json:"template,omitempty"`
	// Description describes the plan.
	// +optional
	Description string `json:"description,omitempty"`
	// Default indicates whether plan is default.
	// +optional
	Default bool `json:"default,omitempty"`
	// Resources requirements to be set on the NGINX container.
	// +optional
	Resources corev1.ResourceRequirements `json:"resources,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status

// RpaasPlan is the Schema for the rpaasplans API
type RpaasPlan struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec RpaasPlanSpec `json:"spec,omitempty"`
}

// +kubebuilder:object:root=true

// RpaasPlanList contains a list of RpaasPlan
type RpaasPlanList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`

	Items []RpaasPlan `json:"items"`
}

type NginxConfig struct {
	User string `json:"user,omitempty"`

	UpstreamKeepalive int `json:"upstreamKeepalive,omitempty"`

	CacheEnabled     *bool              `json:"cacheEnabled,omitempty"`
	CacheInactive    string             `json:"cacheInactive,omitempty"`
	CacheLoaderFiles int                `json:"cacheLoaderFiles,omitempty"`
	CachePath        string             `json:"cachePath,omitempty"`
	CacheSize        *resource.Quantity `json:"cacheSize,omitempty"`
	CacheZoneSize    *resource.Quantity `json:"cacheZoneSize,omitempty"`

	LogFormat            string            `json:"logFormat,omitempty"`
	LogFormatEscape      string            `json:"logFormatEscape,omitempty"`
	LogFormatName        string            `json:"logFormatName,omitempty"`
	LogAdditionalHeaders []string          `json:"logAdditionalHeaders,omitempty"`
	LogAdditionalFields  map[string]string `json:"logAdditionalFields,omitempty"`

	MapHashBucketSize int `json:"mapHashBucketSize,omitempty"`

	HTTPListenOptions  string `json:"httpListenOptions,omitempty"`
	HTTPSListenOptions string `json:"httpsListenOptions,omitempty"`

	VTSEnabled                *bool  `json:"vtsEnabled,omitempty"`
	VTSStatusHistogramBuckets string `json:"vtsStatusHistogramBuckets,omitempty"`

	SyslogEnabled       *bool  `json:"syslogEnabled,omitempty"`
	SyslogServerAddress string `json:"syslogServerAddress,omitempty"`
	SyslogFacility      string `json:"syslogFacility,omitempty"`
	SyslogTag           string `json:"syslogTag,omitempty"`

	WorkerProcesses   int `json:"workerProcesses,omitempty"`
	WorkerConnections int `json:"workerConnections,omitempty"`
}

type CacheSnapshotSyncSpec struct {
	// Schedule is the the cron time string format, see https://en.wikipedia.org/wiki/Cron.
	Schedule string `json:"schedule,omitempty"`

	// Container is the image used to sync the containers
	// default is bitnami/kubectl:latest
	Image string `json:"image,omitempty"`

	// CmdPodToPVC is used to customize command used to sync memory cache (POD) to persistent storage (PVC)
	CmdPodToPVC []string `json:"cmdPodToPVC,omitempty"`

	// CmdPVCToPod is used to customize command used to sync persistent storage (PVC) to memory cache (POD)
	CmdPVCToPod []string `json:"cmdPVCToPod,omitempty"`
}

type CacheSnapshotStorage struct {
	StorageClassName *string            `json:"storageClassName,omitempty"`
	StorageSize      *resource.Quantity `json:"storageSize,omitempty"`
	VolumeLabels     map[string]string  `json:"volumeLabels,omitempty"`
}

func Bool(v bool) *bool {
	return &v
}

func BoolValue(v *bool) bool {
	if v == nil {
		return false
	}
	return *v
}

func init() {
	SchemeBuilder.Register(&RpaasPlan{}, &RpaasPlanList{})
}
