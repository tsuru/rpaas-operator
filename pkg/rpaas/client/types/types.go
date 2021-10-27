// Copyright 2019 tsuru authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package types

import (
	"fmt"
	"time"

	corev1 "k8s.io/api/core/v1"

	"github.com/tsuru/rpaas-operator/api/v1alpha1"
)

type Block struct {
	Name    string `json:"block_name" form:"block_name"`
	Content string `json:"content" form:"content"`
}

type Route struct {
	Path        string `json:"path" form:"path"`
	Destination string `json:"destination,omitempty" form:"destination,omitempty"`
	HTTPSOnly   bool   `json:"https_only,omitempty" form:"https_only,omitempty"`
	Content     string `json:"content,omitempty" form:"content,omitempty"`
}

type Autoscale struct {
	MinReplicas *int32 `json:"minReplicas,omitempty" form:"min"`
	MaxReplicas *int32 `json:"maxReplicas,omitempty" form:"max"`
	CPU         *int32 `json:"cpu,omitempty" form:"cpu"`
	Memory      *int32 `json:"memory,omitempty" form:"memory"`
}

type Flavor struct {
	Name        string `json:"name"`
	Description string `json:"description"`
}

type Plan struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Default     bool   `json:"default"`
}

type InstanceAddressType string

var (
	InstanceAddressTypeClusterExternal = InstanceAddressType("cluster-external")
	InstanceAddressTypeClusterInternal = InstanceAddressType("cluster-internal")
)

type InstanceAddress struct {
	Type        InstanceAddressType `json:"type"`
	ServiceName string              `json:"serviceName,omitempty"`
	IngressName string              `json:"ingressName,omitempty"`
	Hostname    string              `json:"hostname,omitempty"`
	IP          string              `json:"ip,omitempty"`
	Status      string              `json:"status"`
}

type PodPort corev1.ContainerPort

func (p PodPort) String() string {
	protocol := p.Protocol
	if protocol == "" {
		protocol = corev1.ProtocolTCP
	}

	port := p.HostPort
	if port == int32(0) {
		port = p.ContainerPort
	}

	return fmt.Sprintf("%s(%d/%s)", p.Name, port, protocol)
}

type PodError struct {
	First   time.Time `json:"first"`
	Last    time.Time `json:"last"`
	Message string    `json:"message"`
	Count   int32     `json:"count"`
}

type Pod struct {
	CreatedAt time.Time   `json:"createdAt,omitempty"`
	Name      string      `json:"name"`
	IP        string      `json:"ip"`
	HostIP    string      `json:"host"`
	Status    string      `json:"status"`
	Ports     []PodPort   `json:"ports,omitempty"`
	Errors    []PodError  `json:"errors,omitempty"`
	Restarts  int32       `json:"restarts"`
	Ready     bool        `json:"ready"`
	Metrics   *PodMetrics `json:"metrics,omitempty"`
}

type PodMetrics struct {
	CPU    string `json:"cpu"`
	Memory string `json:"memory"`
}

type CertificateInfo struct {
	Name               string
	ValidFrom          time.Time
	ValidUntil         time.Time
	DNSNames           []string
	PublicKeyAlgorithm string
	PublicKeyBitSize   int
}

type InstanceInfo struct {
	Dashboard    string            `json:"dashboard,omitempty"`
	Addresses    []InstanceAddress `json:"addresses,omitempty"`
	Replicas     *int32            `json:"replicas,omitempty"`
	Plan         string            `json:"plan,omitempty"`
	Blocks       []Block           `json:"blocks,omitempty"`
	Routes       []Route           `json:"routes,omitempty"`
	Autoscale    *Autoscale        `json:"autoscale,omitempty"`
	Binds        []v1alpha1.Bind   `json:"binds,omitempty"`
	Team         string            `json:"team,omitempty"`
	Name         string            `json:"name,omitempty"`
	Service      string            `json:"service,omitempty"`
	Description  string            `json:"description,omitempty"`
	Cluster      string            `json:"cluster,omitempty"` // for multi-cluster environments
	Tags         []string          `json:"tags,omitempty"`
	Pods         []Pod             `json:"pods,omitempty"`
	Flavors      []string          `json:"flavors,omitempty"`
	Certificates []CertificateInfo `json:"certificates,omitempty"`
}

type AllowedUpstream struct {
	Host string `json:"host,omitempty"`
	Port int    `json:"port,omitempty"`
}

type CertManager struct {
	Issuer      string   `json:"issuer"`
	DNSNames    []string `json:"dnsNames,omitempty"`
	IPAddresses []string `json:"ipAddresses,omitempty"`
}
