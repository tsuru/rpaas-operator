// Copyright 2024 tsuru authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package rpaas

import (
	"context"
	"fmt"
	"slices"
	"strings"

	clientTypes "github.com/tsuru/rpaas-operator/pkg/rpaas/client/types"
)

func filterMetadata(meta map[string]string) map[string]string {
	filterAnnotations := make(map[string]string)
	for key, val := range meta {
		if !strings.HasPrefix(key, defaultKeyLabelPrefix) {
			filterAnnotations[key] = val
		}
	}
	return filterAnnotations
}

func flattenMetadata(meta map[string]string) []string {
	var result []string
	for k, v := range meta {
		result = append(result, fmt.Sprintf("%s=%s", k, v))
	}
	slices.Sort(result)
	return result
}

func (m *k8sRpaasManager) GetMetadata(ctx context.Context, instanceName string) (*clientTypes.Metadata, error) {
	instance, err := m.GetInstance(ctx, instanceName)
	if err != nil {
		return nil, err
	}

	filteredLabels := filterMetadata(instance.Labels)
	filteredAnnotations := filterMetadata(instance.Annotations)

	metadata := &clientTypes.Metadata{}

	for k, v := range filteredLabels {
		item := clientTypes.MetadataItem{Name: k, Value: v}
		metadata.Labels = append(metadata.Labels, item)
	}

	for k, v := range filteredAnnotations {
		item := clientTypes.MetadataItem{Name: k, Value: v}
		metadata.Annotations = append(metadata.Annotations, item)
	}

	return metadata, nil
}

func validateMetadata(items []clientTypes.MetadataItem) error {
	for _, item := range items {
		if strings.HasPrefix(item.Name, defaultKeyLabelPrefix) {
			return &ValidationError{Msg: fmt.Sprintf("metadata key %q is reserved", item.Name)}
		}
	}
	return nil
}

func (m *k8sRpaasManager) SetMetadata(ctx context.Context, instanceName string, metadata *clientTypes.Metadata) error {
	instance, err := m.GetInstance(ctx, instanceName)
	if err != nil {
		return err
	}

	if err = validateMetadata(metadata.Labels); err != nil {
		return err
	}

	if err = validateMetadata(metadata.Annotations); err != nil {
		return err
	}

	originalInstance := instance.DeepCopy()

	if metadata.Labels != nil {
		if instance.Labels == nil {
			instance.Labels = make(map[string]string)
		}
		for _, item := range metadata.Labels {
			instance.Labels[item.Name] = item.Value
		}
	}

	if metadata.Annotations != nil {
		if instance.Annotations == nil {
			instance.Annotations = make(map[string]string)
		}
		for _, item := range metadata.Annotations {
			instance.Annotations[item.Name] = item.Value
		}
	}

	return m.patchInstance(ctx, originalInstance, instance)
}

func (m *k8sRpaasManager) UnsetMetadata(ctx context.Context, instanceName string, metadata *clientTypes.Metadata) error {
	instance, err := m.GetInstance(ctx, instanceName)
	if err != nil {
		return err
	}
	originalInstance := instance.DeepCopy()

	if metadata.Labels != nil {
		for _, item := range metadata.Labels {
			if _, ok := instance.Labels[item.Name]; !ok {
				return &NotFoundError{Msg: fmt.Sprintf("label %q not found in instance %q", item.Name, instanceName)}
			}
			delete(instance.Labels, item.Name)
		}
	}

	if metadata.Annotations != nil {
		for _, item := range metadata.Annotations {
			if _, ok := instance.Annotations[item.Name]; !ok {
				return &NotFoundError{Msg: fmt.Sprintf("annotation %q not found in instance %q", item.Name, instanceName)}
			}
			delete(instance.Annotations, item.Name)
		}
	}

	return m.patchInstance(ctx, originalInstance, instance)
}
