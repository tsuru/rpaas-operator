// Copyright 2024 tsuru authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package rpaas

import (
	"fmt"
	"net/url"
	"regexp"
	"strings"

	"k8s.io/apimachinery/pkg/util/validation"

	"github.com/tsuru/rpaas-operator/api/v1alpha1"
)

type mutation struct {
	spec *v1alpha1.RpaasInstanceSpec
}

// mutation represents all atomic operations that user can change the RpaasInstanceSpec
func NewMutation(spec *v1alpha1.RpaasInstanceSpec) *mutation {
	return &mutation{spec}
}

func (m *mutation) UpdateBlock(block ConfigurationBlock) error {
	err := validateBlock(block)
	if err != nil {
		return err
	}

	if block.ServerName == "" {
		if m.spec.Blocks == nil {
			m.spec.Blocks = make(map[v1alpha1.BlockType]v1alpha1.Value)
		}

		blockType := v1alpha1.BlockType(block.Name)
		m.spec.Blocks[blockType] = v1alpha1.Value{Value: block.Content}
	} else {
		if m.spec.ServerBlocks == nil {
			m.spec.ServerBlocks = make([]v1alpha1.ServerBlock, 0)
		}

		blockType := v1alpha1.BlockType(block.Name)

		serverBlock := v1alpha1.ServerBlock{
			ServerName: block.ServerName,
			Type:       blockType,
			Content:    &v1alpha1.Value{Value: block.Content},
			Extend:     block.Extend,
		}

		if index, found := hasServerBlock(m.spec, block.ServerName, blockType); found {
			m.spec.ServerBlocks[index] = serverBlock
		} else {
			m.spec.ServerBlocks = append(m.spec.ServerBlocks, serverBlock)
		}
	}

	return nil
}

func (m *mutation) DeleteBlock(serverName, blockName string) error {
	blockType := v1alpha1.BlockType(blockName)

	if serverName == "" {
		if m.spec.Blocks == nil {
			return NotFoundError{Msg: fmt.Sprintf("block %q not found", blockName)}
		}

		if _, ok := m.spec.Blocks[blockType]; !ok {
			return NotFoundError{Msg: fmt.Sprintf("block %q not found", blockName)}
		}

		delete(m.spec.Blocks, blockType)
	} else {
		index, found := hasServerBlock(m.spec, serverName, blockType)
		if !found {
			return NotFoundError{Msg: fmt.Sprintf("block %q with serverName %q not found", blockName, serverName)}
		}

		m.spec.ServerBlocks = append(m.spec.ServerBlocks[:index], m.spec.ServerBlocks[index+1:]...)
	}
	return nil
}

func (m *mutation) UpdateRoute(route Route) error {
	if err := validateRoute(route); err != nil {
		return err
	}

	var content *v1alpha1.Value
	if route.Content != "" {
		content = &v1alpha1.Value{Value: route.Content}
	}

	newLocation := v1alpha1.Location{
		ServerName:  route.ServerName,
		Path:        route.Path,
		Destination: route.Destination,
		ForceHTTPS:  route.HTTPSOnly,
		Content:     content,
	}

	if index, found := hasPath(m.spec, route.ServerName, route.Path); found {
		m.spec.Locations[index] = newLocation
	} else {
		m.spec.Locations = append(m.spec.Locations, newLocation)
	}

	return nil
}

func (m *mutation) DeleteRoute(serverName, path string) error {
	index, found := hasPath(m.spec, serverName, path)
	if !found {
		if serverName != "" {
			return &NotFoundError{Msg: fmt.Sprintf("path %q with serverName %q does not exist", path, serverName)}
		}
		return &NotFoundError{Msg: fmt.Sprintf("path %q does not exist", path)}
	}

	m.spec.Locations = append(m.spec.Locations[:index], m.spec.Locations[index+1:]...)
	return nil
}

func (m *mutation) BindApp(args BindAppArgs, internalBind bool) error {
	var host string
	if args.AppClusterName != "" && internalBind {
		if len(args.AppInternalHosts) == 0 || args.AppInternalHosts[0] == "" {
			return &ValidationError{Msg: "application internal hosts cannot be empty"}
		}

		host = args.AppInternalHosts[0]
	} else {
		if len(args.AppHosts) == 0 || args.AppHosts[0] == "" {
			return &ValidationError{Msg: "application hosts cannot be empty"}
		}

		host = args.AppHosts[0]
	}

	u, err := url.Parse(host)
	if err != nil {
		return err
	}
	if u.Scheme == "tcp" {
		host = u.Host
	}

	if u.Scheme == "udp" {
		return &ValidationError{Msg: fmt.Sprintf("Unsupported host: %q", host)}
	}

	if len(m.spec.Binds) > 0 {
		for _, value := range m.spec.Binds {
			if value.Host == host {
				return &ConflictError{Msg: "instance already bound with this application"}
			}
		}
	}
	if m.spec.Binds == nil {
		m.spec.Binds = make([]v1alpha1.Bind, 0)
	}

	m.spec.Binds = append(m.spec.Binds, v1alpha1.Bind{Host: host, Name: args.AppName})

	return nil
}

func (m *mutation) UnbindApp(appName string) error {
	if appName == "" {
		return &ValidationError{Msg: "must specify an app name"}
	}

	var found bool
	for i, bind := range m.spec.Binds {
		if bind.Name == appName {
			found = true
			binds := m.spec.Binds
			// Remove the element at index i from instance.Spec.Binds *maintaining it's order! -> O(n)*.
			m.spec.Binds = append(binds[:i], binds[i+1:]...)
			break
		}
	}

	if !found {
		return &NotFoundError{Msg: "app not found in instance bind list"}
	}

	return nil
}

func validateBlock(block ConfigurationBlock) error {
	blockType := v1alpha1.BlockType(block.Name)
	if !isBlockTypeAllowed(blockType) {
		return ValidationError{Msg: fmt.Sprintf("block %q is not allowed", block.Name)}
	}
	if block.Content == "" {
		return &ValidationError{Msg: "content is required"}
	}
	err := validateContent(block.Content)
	if err != nil {
		return err
	}

	if block.ServerName != "" {
		if blockType != v1alpha1.BlockTypeServer {
			return &ValidationError{Msg: "serverName is only allowed for server block"}
		}

		if len(block.ServerName) > 64 {
			return &ValidationError{Msg: "serverName must be less than 64 characters"}
		}

		nameToValidate := strings.TrimPrefix(block.ServerName, "*.")
		if errs := validation.IsDNS1123Subdomain(nameToValidate); len(errs) > 0 {
			return &ValidationError{Msg: "serverName must be a valid DNS-1123 domain"}
		}

	}

	return nil
}

func validateRoute(r Route) error {
	if r.Path == "" {
		return &ValidationError{Msg: "path is required"}
	}

	if !regexp.MustCompile(`^/[^ ]*`).MatchString(r.Path) {
		return &ValidationError{Msg: "invalid path format"}
	}

	if r.Content == "" && r.Destination == "" {
		return &ValidationError{Msg: "either content or destination are required"}
	}

	if r.Content != "" && r.Destination != "" {
		return &ValidationError{Msg: "cannot set both content and destination"}
	}

	if r.Content != "" && r.HTTPSOnly {
		return &ValidationError{Msg: "cannot set both content and httpsonly"}
	}

	if r.Content != "" {
		err := validateContent(r.Content)
		if err != nil {
			return err
		}
	}

	return nil
}

func hasPath(spec *v1alpha1.RpaasInstanceSpec, serverName, path string) (index int, found bool) {
	for i, location := range spec.Locations {
		if location.ServerName == serverName && location.Path == path {
			return i, true
		}
	}

	return
}

func hasServerBlock(spec *v1alpha1.RpaasInstanceSpec, serverName string, blockType v1alpha1.BlockType) (index int, found bool) {
	for i, block := range spec.ServerBlocks {
		if block.ServerName == serverName && block.Type == blockType {
			return i, true
		}
	}

	return
}
