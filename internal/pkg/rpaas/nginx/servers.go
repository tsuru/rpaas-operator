// Copyright 2025 tsuru authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package nginx

import (
	"sort"
	"strings"

	nginxv1alpha1 "github.com/tsuru/nginx-operator/api/v1alpha1"
	"github.com/tsuru/rpaas-operator/api/v1alpha1"
)

type Server struct {
	Name          string `json:"names"`
	TLS           bool   `json:"tls"`
	TLSSecretName string `json:"secretName"`
	Default       bool   `json:"default,omitempty"`
	Wildcard      bool   `json:"wildcard,omitempty"`

	Blocks    map[v1alpha1.BlockType]v1alpha1.Value
	Locations []v1alpha1.Location `json:"locations,omitempty"`
}

func ProduceServers(spec *v1alpha1.RpaasInstanceSpec) []*Server {
	defaultServer := &Server{
		Default: true,
		Blocks:  deepCopyBlocks(spec.Blocks),
	}

	mapServerNames := make(map[string]*Server)
	defaultLocationsIndex := map[string]int{}

	for _, server := range spec.Locations {
		if server.ServerName != "" {
			mapServerNames[server.ServerName] = &Server{
				Name:   server.ServerName,
				Blocks: deepCopyBlocks(spec.Blocks),
			}
		}
	}

	for _, serverBlock := range spec.ServerBlocks {
		if serverBlock.ServerName != "" {
			if _, ok := mapServerNames[serverBlock.ServerName]; !ok {
				mapServerNames[serverBlock.ServerName] = &Server{
					Name:   serverBlock.ServerName,
					Blocks: deepCopyBlocks(spec.Blocks),
				}
			}

			_, hasDefaultBlock := defaultServer.Blocks[serverBlock.Type]
			if serverBlock.Extend && hasDefaultBlock {
				mapServerNames[serverBlock.ServerName].Blocks[serverBlock.Type] = v1alpha1.Value{
					Value: defaultServer.Blocks[v1alpha1.BlockTypeServer].Value + "\n" + serverBlock.Content.Value,
				}
			} else {
				mapServerNames[serverBlock.ServerName].Blocks[serverBlock.Type] = *serverBlock.Content.DeepCopy()
			}
		}
	}

	for _, location := range spec.Locations {
		if location.ServerName != "" {
			continue
		}
		defaultServer.Locations = append(defaultServer.Locations, *location.DeepCopy())
		defaultLocationsIndex[location.Path] = len(defaultLocationsIndex)

		for _, server := range mapServerNames {
			server.Locations = append(server.Locations, location)
		}

	}

	for _, location := range spec.Locations {
		if location.ServerName == "" {
			continue
		}

		if index, ok := defaultLocationsIndex[location.Path]; ok {
			mapServerNames[location.ServerName].Locations[index] = *location.DeepCopy()
		} else {
			mapServerNames[location.ServerName].Locations = append(mapServerNames[location.ServerName].Locations, *location.DeepCopy())
		}

	}

	wildCardTLS := map[string]nginxv1alpha1.NginxTLS{}

	for _, tls := range spec.TLS {
		for _, host := range tls.Hosts {
			if isWildCard(host) {
				wildCardTLS[host] = tls
			}
		}
	}

	var appendTLSServer = func(host string, tlsSecretName string) {
		if _, ok := mapServerNames[host]; !ok {
			mapServerNames[host] = &Server{
				Name:      host,
				Locations: deepCopyLocations(defaultServer.Locations),
				Blocks:    deepCopyBlocks(spec.Blocks),
			}
		}

		server := mapServerNames[host]
		server.TLS = true
		server.TLSSecretName = tlsSecretName
		server.Wildcard = isWildCard(host)

	}

	for _, tls := range spec.TLS {
		for _, host := range tls.Hosts {
			appendTLSServer(host, tls.SecretName)
		}
	}

	for host, server := range mapServerNames {
		if server.TLS {
			continue
		}

		// find possible wildcard
		possibleWildcard := "*." + strings.SplitN(host, ".", 2)[1]
		if tls, ok := wildCardTLS[possibleWildcard]; ok {
			appendTLSServer(host, tls.SecretName)
		}
	}

	exactServers := []*Server{}
	wildcardServers := []*Server{}

	for _, server := range mapServerNames {
		if server.Wildcard {
			wildcardServers = append(wildcardServers, server)
		} else {
			exactServers = append(exactServers, server)
		}
	}

	sortServers(exactServers)
	sortServers(wildcardServers)

	result := []*Server{defaultServer}
	result = append(result, exactServers...)
	result = append(result, wildcardServers...)

	return result
}

func sortServers(servers []*Server) {
	sort.Slice(servers, func(i, j int) bool {
		return servers[i].Name < servers[j].Name
	})
}

func deepCopyLocations(locations []v1alpha1.Location) []v1alpha1.Location {
	if len(locations) == 0 {
		return nil
	}
	copiedLocations := make([]v1alpha1.Location, len(locations))
	copy(copiedLocations, locations)
	return copiedLocations
}

func isWildCard(host string) bool {
	return strings.HasPrefix(host, "*.")
}

func deepCopyBlocks(blocks map[v1alpha1.BlockType]v1alpha1.Value) map[v1alpha1.BlockType]v1alpha1.Value {
	if len(blocks) == 0 {
		return nil
	}
	copiedBlocks := make(map[v1alpha1.BlockType]v1alpha1.Value, len(blocks))
	for k, v := range blocks {
		copiedBlocks[k] = *v.DeepCopy()
	}
	return copiedBlocks
}
