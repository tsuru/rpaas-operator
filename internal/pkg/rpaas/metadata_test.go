// Copyright 2024 tsuru authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package rpaas

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	clientTypes "github.com/tsuru/rpaas-operator/pkg/rpaas/client/types"
)

func Test_k8sRpaasManager_GetMetadata(t *testing.T) {
	scheme := newScheme()

	instance := newEmptyRpaasInstance()
	instance.ObjectMeta = metav1.ObjectMeta{
		Name:      "my-instance",
		Namespace: "rpaasv2",
		Labels: map[string]string{
			"rpaas.extensions.tsuru.io/cluster-name":  "my-cluster",
			"rpaas.extensions.tsuru.io/instance-name": "my-instance",
			"rpaas.extensions.tsuru.io/service-name":  "my-service",
			"rpaas.extensions.tsuru.io/team-owner":    "my-team",
			"rpaas_instance":                          "my-instance",
			"rpaas_service":                           "my-service",
		},
		Annotations: map[string]string{
			"rpaas.extensions.tsuru.io/cluster-name": "my-cluster",
			"rpaas.extensions.tsuru.io/description":  "my-description",
			"rpaas.extensions.tsuru.io/tags":         "my-tag=my-value",
			"rpaas.extensions.tsuru.io/team-owner":   "my-team",
			"custom-annotation":                      "custom-value",
		},
	}

	manager := &k8sRpaasManager{cli: fake.NewClientBuilder().WithScheme(scheme).WithRuntimeObjects(instance).Build()}
	meta, err := manager.GetMetadata(context.Background(), "my-instance")
	require.NoError(t, err)

	assert.Equal(t, len(meta.Labels), 2)
	assert.Contains(t, meta.Labels, clientTypes.MetadataItem{Name: "rpaas_instance", Value: "my-instance"})
	assert.Contains(t, meta.Labels, clientTypes.MetadataItem{Name: "rpaas_service", Value: "my-service"})

	assert.Equal(t, len(meta.Annotations), 1)
	assert.Contains(t, meta.Annotations, clientTypes.MetadataItem{Name: "custom-annotation", Value: "custom-value"})
}

func Test_k8sRpaasManager_SetMetadata(t *testing.T) {
	scheme := newScheme()
	testCases := []struct {
		name        string
		meta        *clientTypes.Metadata
		expectedErr string
	}{
		{
			name: "set metadata",
			meta: &clientTypes.Metadata{
				Labels: []clientTypes.MetadataItem{
					{Name: "rpaas_instance", Value: "my-instance"},
					{Name: "rpaas_service", Value: "my-service"},
				},
				Annotations: []clientTypes.MetadataItem{
					{Name: "custom-annotation", Value: "custom-value"},
				},
			},
		},
		{
			name: "set reserved metadata for labels",
			meta: &clientTypes.Metadata{
				Labels: []clientTypes.MetadataItem{
					{Name: "rpaas.extensions.tsuru.io/custom-key", Value: "custom-value"},
				},
			},
			expectedErr: "metadata key \"rpaas.extensions.tsuru.io/custom-key\" is reserved",
		},
		{
			name: "set reserved metadata for annotations",
			meta: &clientTypes.Metadata{
				Annotations: []clientTypes.MetadataItem{
					{Name: "rpaas.extensions.tsuru.io/custom-key", Value: "custom-value"},
				},
			},
			expectedErr: "metadata key \"rpaas.extensions.tsuru.io/custom-key\" is reserved",
		},
	}

	for _, tt := range testCases {
		t.Run(tt.name, func(t *testing.T) {
			instance := newEmptyRpaasInstance()
			instance.Name = "my-instance"

			manager := &k8sRpaasManager{cli: fake.NewClientBuilder().WithScheme(scheme).WithRuntimeObjects(instance).Build()}

			err := manager.SetMetadata(context.Background(), "my-instance", tt.meta)
			if tt.expectedErr != "" {
				assert.EqualError(t, err, tt.expectedErr)
				return
			}

			require.NoError(t, err)

			instance = newEmptyRpaasInstance()

			err = manager.cli.Get(context.Background(), types.NamespacedName{Name: "my-instance", Namespace: "rpaasv2"}, instance)
			require.NoError(t, err)

			for _, item := range tt.meta.Labels {
				assert.Equal(t, item.Value, instance.Labels[item.Name])
			}

			for _, item := range tt.meta.Annotations {
				assert.Equal(t, item.Value, instance.Annotations[item.Name])
			}
		})
	}
}

func Test_k8sRpaasManager_UnsetMetadata(t *testing.T) {
	testCases := []struct {
		name        string
		objMeta     metav1.ObjectMeta
		meta        clientTypes.Metadata
		expectedErr string
	}{
		{
			name: "unset label",
			objMeta: metav1.ObjectMeta{
				Name:      "my-instance",
				Namespace: "rpaasv2",
				Labels: map[string]string{
					"my-label":       "my-value",
					"my-other-label": "my-other-value",
				},
				Annotations: map[string]string{
					"my-annotation":       "my-value",
					"my-other-annotation": "my-other-value",
				},
			},
			meta: clientTypes.Metadata{
				Labels: []clientTypes.MetadataItem{
					{Name: "my-label"},
				},
				Annotations: []clientTypes.MetadataItem{
					{Name: "my-other-annotation"},
				},
			},
		},
		{
			name: "unset invalid label",
			objMeta: metav1.ObjectMeta{
				Name:      "my-instance",
				Namespace: "rpaasv2",
				Labels: map[string]string{
					"my-label": "my-label-value",
				},
			},
			meta: clientTypes.Metadata{
				Labels: []clientTypes.MetadataItem{
					{Name: "invalid-label"},
				},
			},
			expectedErr: "label \"invalid-label\" not found in instance \"my-instance\"",
		},
		{
			name: "unset invalid annotation",
			objMeta: metav1.ObjectMeta{
				Name:      "my-instance",
				Namespace: "rpaasv2",
				Annotations: map[string]string{
					"my-annotation": "my-annotation-value",
				},
			},
			meta: clientTypes.Metadata{
				Annotations: []clientTypes.MetadataItem{
					{Name: "invalid-annotation"},
				},
			},
			expectedErr: "annotation \"invalid-annotation\" not found in instance \"my-instance\"",
		},
	}

	for _, tt := range testCases {
		t.Run(tt.name, func(t *testing.T) {
			scheme := newScheme()

			instance := newEmptyRpaasInstance()
			instance.Name = "my-instance"
			instance.ObjectMeta = tt.objMeta

			manager := &k8sRpaasManager{cli: fake.NewClientBuilder().WithScheme(scheme).WithRuntimeObjects(instance).Build()}

			err := manager.UnsetMetadata(context.Background(), "my-instance", &tt.meta)
			if tt.expectedErr != "" {
				assert.EqualError(t, err, tt.expectedErr)
				return
			}

			require.NoError(t, err)

			instance = newEmptyRpaasInstance()

			err = manager.cli.Get(context.Background(), types.NamespacedName{Name: "my-instance", Namespace: "rpaasv2"}, instance)
			require.NoError(t, err)

			for _, item := range tt.meta.Labels {
				assert.NotContains(t, instance.Labels, item.Name)
			}

			for _, item := range tt.meta.Annotations {
				assert.NotContains(t, instance.Annotations, item.Name)
			}
		})
	}
}
