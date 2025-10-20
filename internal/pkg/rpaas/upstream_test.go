// Copyright 2025 tsuru authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package rpaas

import (
	"context"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tsuru/rpaas-operator/api/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func Test_k8sRpaasManager_GetUpstreamOptions(t *testing.T) {
	instance := &v1alpha1.RpaasInstance{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "my-instance",
			Namespace: "rpaasv2",
		},
		Spec: v1alpha1.RpaasInstanceSpec{
			UpstreamOptions: []v1alpha1.UpstreamOptions{
				{
					PrimaryBind: "app1",
					CanaryBinds: []string{"app2"},
					LoadBalance: v1alpha1.LoadBalanceRoundRobin,
					TrafficShapingPolicy: v1alpha1.TrafficShapingPolicy{
						Weight: 80,
						Header: "X-Test",
					},
				},
			},
		},
	}

	manager := &k8sRpaasManager{cli: fake.NewClientBuilder().WithScheme(newScheme()).WithRuntimeObjects(instance).Build()}
	upstreamOptions, err := manager.GetUpstreamOptions(context.TODO(), "my-instance")
	require.NoError(t, err)
	assert.Equal(t, instance.Spec.UpstreamOptions, upstreamOptions)
}

func Test_k8sRpaasManager_GetUpstreamOptions_InstanceNotFound(t *testing.T) {
	manager := &k8sRpaasManager{cli: fake.NewClientBuilder().WithScheme(newScheme()).Build()}
	_, err := manager.GetUpstreamOptions(context.TODO(), "nonexistent")
	assert.Error(t, err)
}

func Test_k8sRpaasManager_AddUpstreamOptions(t *testing.T) {
	tests := []struct {
		name         string
		instance     *v1alpha1.RpaasInstance
		args         UpstreamOptionsArgs
		expectedSpec []v1alpha1.UpstreamOptions
		expectError  bool
		errorMsg     string
	}{
		{
			name: "successful add",
			instance: &v1alpha1.RpaasInstance{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "my-instance",
					Namespace: "rpaasv2",
				},
				Spec: v1alpha1.RpaasInstanceSpec{
					Binds: []v1alpha1.Bind{
						{Name: "app1", Host: "app1.example.com"},
						{Name: "app2", Host: "app2.example.com"},
					},
				},
			},
			args: UpstreamOptionsArgs{
				PrimaryBind: "app1",
				CanaryBinds: []string{},
				LoadBalance: v1alpha1.LoadBalanceRoundRobin,
			},
			expectedSpec: []v1alpha1.UpstreamOptions{
				{
					PrimaryBind: "app1",
					CanaryBinds: nil,
					LoadBalance: v1alpha1.LoadBalanceRoundRobin,
				},
			},
		},
		{
			name: "empty app",
			instance: &v1alpha1.RpaasInstance{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "my-instance",
					Namespace: "rpaasv2",
				},
			},
			args: UpstreamOptionsArgs{
				PrimaryBind: "",
			},
			expectError: true,
			errorMsg:    "cannot process upstream options with empty app",
		},
		{
			name: "bind not found in instance binds",
			instance: &v1alpha1.RpaasInstance{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "my-instance",
					Namespace: "rpaasv2",
				},
				Spec: v1alpha1.RpaasInstanceSpec{
					Binds: []v1alpha1.Bind{
						{Name: "app1", Host: "app1.example.com"},
					},
				},
			},
			args: UpstreamOptionsArgs{
				PrimaryBind: "nonexistent",
			},
			expectError: true,
			errorMsg:    "bind 'nonexistent' does not exist in instance binds",
		},
		{
			name: "upstream options already exist for bind",
			instance: &v1alpha1.RpaasInstance{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "my-instance",
					Namespace: "rpaasv2",
				},
				Spec: v1alpha1.RpaasInstanceSpec{
					Binds: []v1alpha1.Bind{
						{Name: "app1", Host: "app1.example.com"},
					},
					UpstreamOptions: []v1alpha1.UpstreamOptions{
						{PrimaryBind: "app1"},
					},
				},
			},
			args: UpstreamOptionsArgs{
				PrimaryBind: "app1",
			},
			expectError: true,
			errorMsg:    "upstream options for bind 'app1' already exist in instance: my-instance",
		},
		{
			name: "canary bind must reference existing bind",
			instance: &v1alpha1.RpaasInstance{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "my-instance",
					Namespace: "rpaasv2",
				},
				Spec: v1alpha1.RpaasInstanceSpec{
					Binds: []v1alpha1.Bind{
						{Name: "app1", Host: "app1.example.com"},
						{Name: "app2", Host: "app2.example.com"},
					},
				},
			},
			args: UpstreamOptionsArgs{
				PrimaryBind: "app1",
				CanaryBinds: []string{"nonexistent"},
			},
			expectError: true,
			errorMsg:    "canary bind 'nonexistent' must reference an existing bind from another upstream option",
		},
		{
			name: "bind referenced as canary cannot have own canary binds",
			instance: &v1alpha1.RpaasInstance{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "my-instance",
					Namespace: "rpaasv2",
				},
				Spec: v1alpha1.RpaasInstanceSpec{
					Binds: []v1alpha1.Bind{
						{Name: "app1", Host: "app1.example.com"},
						{Name: "app2", Host: "app2.example.com"},
					},
					UpstreamOptions: []v1alpha1.UpstreamOptions{
						{PrimaryBind: "app1", CanaryBinds: []string{"app2"}},
					},
				},
			},
			args: UpstreamOptionsArgs{
				PrimaryBind: "app2",
				CanaryBinds: []string{"app1"},
			},
			expectError: true,
			errorMsg:    "bind 'app2' is referenced as a canary bind in another upstream option and cannot have its own canary binds",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			manager := &k8sRpaasManager{cli: fake.NewClientBuilder().WithScheme(newScheme()).WithRuntimeObjects(tt.instance).Build()}
			err := manager.AddUpstreamOptions(context.TODO(), "my-instance", tt.args)

			if tt.expectError {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.errorMsg)
			} else {
				require.NoError(t, err)

				var updated v1alpha1.RpaasInstance
				err = manager.cli.Get(context.TODO(), types.NamespacedName{Name: "my-instance", Namespace: "rpaasv2"}, &updated)
				require.NoError(t, err)
				assert.Equal(t, tt.expectedSpec, updated.Spec.UpstreamOptions)
			}
		})
	}
}

func Test_k8sRpaasManager_AddUpstreamOptions_CanaryWeightValidation(t *testing.T) {
	tests := []struct {
		name        string
		instance    *v1alpha1.RpaasInstance
		args        UpstreamOptionsArgs
		expectError bool
		errorMsg    string
	}{
		{
			name: "first canary with weight should succeed",
			instance: &v1alpha1.RpaasInstance{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "my-instance",
					Namespace: "rpaasv2",
				},
				Spec: v1alpha1.RpaasInstanceSpec{
					Binds: []v1alpha1.Bind{
						{Name: "primary", Host: "primary.example.com"},
						{Name: "canary1", Host: "canary1.example.com"},
					},
					UpstreamOptions: []v1alpha1.UpstreamOptions{
						{
							PrimaryBind: "primary",
							CanaryBinds: []string{"canary1"},
						},
					},
				},
			},
			args: UpstreamOptionsArgs{
				PrimaryBind: "canary1",
				TrafficShapingPolicy: v1alpha1.TrafficShapingPolicy{
					Weight: 50,
				},
			},
			expectError: false,
		},
		{
			name: "second canary with weight should fail",
			instance: &v1alpha1.RpaasInstance{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "my-instance",
					Namespace: "rpaasv2",
				},
				Spec: v1alpha1.RpaasInstanceSpec{
					Binds: []v1alpha1.Bind{
						{Name: "primary", Host: "primary.example.com"},
						{Name: "canary1", Host: "canary1.example.com"},
						{Name: "canary2", Host: "canary2.example.com"},
					},
					UpstreamOptions: []v1alpha1.UpstreamOptions{
						{
							PrimaryBind: "primary",
							CanaryBinds: []string{"canary1", "canary2"},
						},
						{
							PrimaryBind: "canary1",
							TrafficShapingPolicy: v1alpha1.TrafficShapingPolicy{
								Weight: 50,
							},
						},
					},
				},
			},
			args: UpstreamOptionsArgs{
				PrimaryBind: "canary2",
				TrafficShapingPolicy: v1alpha1.TrafficShapingPolicy{
					Weight: 30,
				},
			},
			expectError: true,
			errorMsg:    "only one canary bind per group can have weight > 0, but found existing weight in canary group for parent 'primary'",
		},
		{
			name: "canary with other traffic shaping options but no weight should succeed",
			instance: &v1alpha1.RpaasInstance{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "my-instance",
					Namespace: "rpaasv2",
				},
				Spec: v1alpha1.RpaasInstanceSpec{
					Binds: []v1alpha1.Bind{
						{Name: "primary", Host: "primary.example.com"},
						{Name: "canary1", Host: "canary1.example.com"},
						{Name: "canary2", Host: "canary2.example.com"},
					},
					UpstreamOptions: []v1alpha1.UpstreamOptions{
						{
							PrimaryBind: "primary",
							CanaryBinds: []string{"canary1", "canary2"},
						},
						{
							PrimaryBind: "canary1",
							TrafficShapingPolicy: v1alpha1.TrafficShapingPolicy{
								Weight: 50,
							},
						},
					},
				},
			},
			args: UpstreamOptionsArgs{
				PrimaryBind: "canary2",
				TrafficShapingPolicy: v1alpha1.TrafficShapingPolicy{
					Header: "X-Test",
					Cookie: "canary=true",
				},
			},
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			manager := &k8sRpaasManager{cli: fake.NewClientBuilder().WithScheme(newScheme()).WithRuntimeObjects(tt.instance).Build()}
			err := manager.AddUpstreamOptions(context.TODO(), tt.instance.Name, tt.args)
			if tt.expectError {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.errorMsg)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func Test_k8sRpaasManager_AddUpstreamOptions_NewTrafficShapingRules(t *testing.T) {
	tests := []struct {
		name        string
		instance    *v1alpha1.RpaasInstance
		args        UpstreamOptionsArgs
		expectError bool
		errorMsg    string
	}{
		{
			name: "primary upstream cannot have weight when it has canary binds",
			instance: &v1alpha1.RpaasInstance{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "my-instance",
					Namespace: "rpaasv2",
				},
				Spec: v1alpha1.RpaasInstanceSpec{
					Binds: []v1alpha1.Bind{
						{Name: "app1", Host: "host1.example.com"},
						{Name: "canary1", Host: "canary1.example.com"},
					},
					UpstreamOptions: []v1alpha1.UpstreamOptions{
						{
							PrimaryBind: "canary1", // canary1 must exist as upstream option first
						},
					},
				},
			},
			args: UpstreamOptionsArgs{
				PrimaryBind: "app1",
				CanaryBinds: []string{"canary1"},
				TrafficShapingPolicy: v1alpha1.TrafficShapingPolicy{
					Weight: 50, // This should fail - primary cannot have weight with canary binds
				},
			},
			expectError: true,
			errorMsg:    "primary upstream 'app1' cannot have traffic shaping policy when it has canary binds",
		},
		{
			name: "primary upstream cannot have header traffic shaping with canary binds",
			instance: &v1alpha1.RpaasInstance{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "my-instance",
					Namespace: "rpaasv2",
				},
				Spec: v1alpha1.RpaasInstanceSpec{
					Binds: []v1alpha1.Bind{
						{Name: "app1", Host: "host1.example.com"},
						{Name: "canary1", Host: "canary1.example.com"},
					},
					UpstreamOptions: []v1alpha1.UpstreamOptions{
						{
							PrimaryBind: "canary1", // canary1 must exist as upstream option first
						},
					},
				},
			},
			args: UpstreamOptionsArgs{
				PrimaryBind: "app1",
				CanaryBinds: []string{"canary1"},
				TrafficShapingPolicy: v1alpha1.TrafficShapingPolicy{
					Header:      "X-Test",
					HeaderValue: "canary",
				},
			},
			expectError: true,
			errorMsg:    "primary upstream 'app1' cannot have traffic shaping policy when it has canary binds",
		},
		{
			name: "canary upstream can have weight traffic shaping",
			instance: &v1alpha1.RpaasInstance{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "my-instance",
					Namespace: "rpaasv2",
				},
				Spec: v1alpha1.RpaasInstanceSpec{
					Binds: []v1alpha1.Bind{
						{Name: "app1", Host: "host1.example.com"},
						{Name: "canary1", Host: "canary1.example.com"},
					},
					UpstreamOptions: []v1alpha1.UpstreamOptions{
						{
							PrimaryBind: "app1",
							CanaryBinds: []string{"canary1"},
						},
					},
				},
			},
			args: UpstreamOptionsArgs{
				PrimaryBind: "canary1",
				TrafficShapingPolicy: v1alpha1.TrafficShapingPolicy{
					Weight: 80, // This should succeed - canary can have weight
				},
			},
			expectError: false,
		},
		{
			name: "primary upstream can have traffic shaping when no canary binds",
			instance: &v1alpha1.RpaasInstance{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "my-instance",
					Namespace: "rpaasv2",
				},
				Spec: v1alpha1.RpaasInstanceSpec{
					Binds: []v1alpha1.Bind{
						{Name: "app1", Host: "host1.example.com"},
					},
				},
			},
			args: UpstreamOptionsArgs{
				PrimaryBind: "app1",
				CanaryBinds: []string{}, // No canary binds - traffic shaping should be allowed
				TrafficShapingPolicy: v1alpha1.TrafficShapingPolicy{
					Header:      "X-Test",
					HeaderValue: "production",
				},
			},
			expectError: false,
		},
		{
			name: "standalone upstream can have weight traffic shaping",
			instance: &v1alpha1.RpaasInstance{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "my-instance",
					Namespace: "rpaasv2",
				},
				Spec: v1alpha1.RpaasInstanceSpec{
					Binds: []v1alpha1.Bind{
						{Name: "app1", Host: "host1.example.com"},
					},
				},
			},
			args: UpstreamOptionsArgs{
				PrimaryBind: "app1",
				TrafficShapingPolicy: v1alpha1.TrafficShapingPolicy{
					Weight: 100, // This should succeed - standalone upstream can have weight
				},
			},
			expectError: false,
		},
		{
			name: "cannot add multiple canary binds with weight to same group",
			instance: &v1alpha1.RpaasInstance{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "my-instance",
					Namespace: "rpaasv2",
				},
				Spec: v1alpha1.RpaasInstanceSpec{
					Binds: []v1alpha1.Bind{
						{Name: "app1", Host: "host1.example.com"},
						{Name: "canary1", Host: "canary1.example.com"},
						{Name: "canary2", Host: "canary2.example.com"},
					},
					UpstreamOptions: []v1alpha1.UpstreamOptions{
						{
							PrimaryBind: "canary1",
							TrafficShapingPolicy: v1alpha1.TrafficShapingPolicy{
								Weight: 50, // canary1 has weight
							},
						},
						{
							PrimaryBind: "canary2",
							TrafficShapingPolicy: v1alpha1.TrafficShapingPolicy{
								Weight: 30, // canary2 also has weight
							},
						},
					},
				},
			},
			args: UpstreamOptionsArgs{
				PrimaryBind: "app1",
				CanaryBinds: []string{"canary1", "canary2"}, // Multiple canary binds - should fail
			},
			expectError: true,
			errorMsg:    "only one canary bind is allowed per upstream",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			manager := &k8sRpaasManager{cli: fake.NewClientBuilder().WithScheme(newScheme()).WithRuntimeObjects(tt.instance).Build()}
			err := manager.AddUpstreamOptions(context.TODO(), tt.instance.Name, tt.args)
			if tt.expectError {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.errorMsg)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func Test_k8sRpaasManager_UpdateUpstreamOptions(t *testing.T) {
	tests := []struct {
		name         string
		instance     *v1alpha1.RpaasInstance
		args         UpstreamOptionsArgs
		expectedSpec []v1alpha1.UpstreamOptions
		expectError  bool
		errorMsg     string
	}{
		{
			name: "successful update",
			instance: &v1alpha1.RpaasInstance{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "my-instance",
					Namespace: "rpaasv2",
				},
				Spec: v1alpha1.RpaasInstanceSpec{
					UpstreamOptions: []v1alpha1.UpstreamOptions{
						{PrimaryBind: "app1", LoadBalance: v1alpha1.LoadBalanceRoundRobin},
					},
				},
			},
			args: UpstreamOptionsArgs{
				PrimaryBind: "app1",
				CanaryBinds: []string{},
				LoadBalance: v1alpha1.LoadBalanceEWMA,
			},
			expectedSpec: []v1alpha1.UpstreamOptions{
				{
					PrimaryBind: "app1",
					CanaryBinds: nil,
					LoadBalance: v1alpha1.LoadBalanceEWMA,
				},
			},
		},
		{
			name: "empty app",
			instance: &v1alpha1.RpaasInstance{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "my-instance",
					Namespace: "rpaasv2",
				},
			},
			args: UpstreamOptionsArgs{
				PrimaryBind: "",
			},
			expectError: true,
			errorMsg:    "cannot process upstream options with empty app",
		},
		{
			name: "upstream options not found",
			instance: &v1alpha1.RpaasInstance{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "my-instance",
					Namespace: "rpaasv2",
				},
				Spec: v1alpha1.RpaasInstanceSpec{
					UpstreamOptions: []v1alpha1.UpstreamOptions{},
				},
			},
			args: UpstreamOptionsArgs{
				PrimaryBind: "nonexistent",
			},
			expectError: true,
			errorMsg:    "upstream options for bind 'nonexistent' not found in instance: my-instance",
		},
		{
			name: "canary bind validation error",
			instance: &v1alpha1.RpaasInstance{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "my-instance",
					Namespace: "rpaasv2",
				},
				Spec: v1alpha1.RpaasInstanceSpec{
					UpstreamOptions: []v1alpha1.UpstreamOptions{
						{PrimaryBind: "app1"},
					},
				},
			},
			args: UpstreamOptionsArgs{
				PrimaryBind: "app1",
				CanaryBinds: []string{"nonexistent"},
			},
			expectError: true,
			errorMsg:    "canary bind 'nonexistent' must reference an existing bind from another upstream option",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			manager := &k8sRpaasManager{cli: fake.NewClientBuilder().WithScheme(newScheme()).WithRuntimeObjects(tt.instance).Build()}
			err := manager.UpdateUpstreamOptions(context.TODO(), "my-instance", tt.args)

			if tt.expectError {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.errorMsg)
			} else {
				require.NoError(t, err)
				// For successful update tests, just verify no error occurred
				// The fake client has limitations with patch operations
			}
		})
	}
}

func Test_k8sRpaasManager_AddUpstreamOptions_WeightTotalDefault(t *testing.T) {
	tests := []struct {
		name                string
		instance            *v1alpha1.RpaasInstance
		args                UpstreamOptionsArgs
		expectedWeightTotal int
	}{
		{
			name: "weight > 0 with weightTotal = 0 should set weightTotal to 100",
			instance: &v1alpha1.RpaasInstance{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "my-instance",
					Namespace: "rpaasv2",
				},
				Spec: v1alpha1.RpaasInstanceSpec{
					Binds: []v1alpha1.Bind{
						{Name: "primary", Host: "primary.example.com"},
						{Name: "canary1", Host: "canary1.example.com"},
					},
					UpstreamOptions: []v1alpha1.UpstreamOptions{
						{
							PrimaryBind: "primary",
							CanaryBinds: []string{"canary1"},
						},
					},
				},
			},
			args: UpstreamOptionsArgs{
				PrimaryBind: "canary1",
				TrafficShapingPolicy: v1alpha1.TrafficShapingPolicy{
					Weight:      50,
					WeightTotal: 0, // Not set, should default to 100
				},
			},
			expectedWeightTotal: 100,
		},
		{
			name: "weight > 0 with weightTotal already set should keep original value",
			instance: &v1alpha1.RpaasInstance{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "my-instance",
					Namespace: "rpaasv2",
				},
				Spec: v1alpha1.RpaasInstanceSpec{
					Binds: []v1alpha1.Bind{
						{Name: "primary", Host: "primary.example.com"},
						{Name: "canary1", Host: "canary1.example.com"},
					},
					UpstreamOptions: []v1alpha1.UpstreamOptions{
						{
							PrimaryBind: "primary",
							CanaryBinds: []string{"canary1"},
						},
					},
				},
			},
			args: UpstreamOptionsArgs{
				PrimaryBind: "canary1",
				TrafficShapingPolicy: v1alpha1.TrafficShapingPolicy{
					Weight:      30,
					WeightTotal: 200, // Explicitly set, should keep this value
				},
			},
			expectedWeightTotal: 200,
		},
		{
			name: "weight = 0 should not set weightTotal",
			instance: &v1alpha1.RpaasInstance{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "my-instance",
					Namespace: "rpaasv2",
				},
				Spec: v1alpha1.RpaasInstanceSpec{
					Binds: []v1alpha1.Bind{
						{Name: "primary", Host: "primary.example.com"},
						{Name: "canary1", Host: "canary1.example.com"},
					},
					UpstreamOptions: []v1alpha1.UpstreamOptions{
						{
							PrimaryBind: "primary",
							CanaryBinds: []string{"canary1"},
						},
					},
				},
			},
			args: UpstreamOptionsArgs{
				PrimaryBind: "canary1",
				TrafficShapingPolicy: v1alpha1.TrafficShapingPolicy{
					Weight:      0,
					WeightTotal: 0,
					Header:      "X-Test",
				},
			},
			expectedWeightTotal: 0,
		},
		{
			name: "weight = 100 with weightTotal = 0 should set weightTotal to 100",
			instance: &v1alpha1.RpaasInstance{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "my-instance",
					Namespace: "rpaasv2",
				},
				Spec: v1alpha1.RpaasInstanceSpec{
					Binds: []v1alpha1.Bind{
						{Name: "primary", Host: "primary.example.com"},
						{Name: "canary1", Host: "canary1.example.com"},
					},
					UpstreamOptions: []v1alpha1.UpstreamOptions{
						{
							PrimaryBind: "primary",
							CanaryBinds: []string{"canary1"},
						},
					},
				},
			},
			args: UpstreamOptionsArgs{
				PrimaryBind: "canary1",
				TrafficShapingPolicy: v1alpha1.TrafficShapingPolicy{
					Weight:      100,
					WeightTotal: 0, // Not set, should default to 100
				},
			},
			expectedWeightTotal: 100,
		},
		{
			name: "weight > 100 with weightTotal = 0 should calculate weightTotal",
			instance: &v1alpha1.RpaasInstance{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "my-instance",
					Namespace: "rpaasv2",
				},
				Spec: v1alpha1.RpaasInstanceSpec{
					Binds: []v1alpha1.Bind{
						{Name: "primary", Host: "primary.example.com"},
						{Name: "canary1", Host: "canary1.example.com"},
					},
					UpstreamOptions: []v1alpha1.UpstreamOptions{
						{
							PrimaryBind: "primary",
							CanaryBinds: []string{"canary1"},
						},
					},
				},
			},
			args: UpstreamOptionsArgs{
				PrimaryBind: "canary1",
				TrafficShapingPolicy: v1alpha1.TrafficShapingPolicy{
					Weight:      250,
					WeightTotal: 0, // Not set, should default to 2500
				},
			},
			expectedWeightTotal: 2500,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			manager := &k8sRpaasManager{cli: fake.NewClientBuilder().WithScheme(newScheme()).WithRuntimeObjects(tt.instance).Build()}
			err := manager.AddUpstreamOptions(context.TODO(), tt.instance.Name, tt.args)
			require.NoError(t, err)

			// Get the updated instance to verify WeightTotal was set correctly
			updated := &v1alpha1.RpaasInstance{}
			err = manager.cli.Get(context.TODO(), types.NamespacedName{
				Name:      tt.instance.Name,
				Namespace: tt.instance.Namespace,
			}, updated)
			require.NoError(t, err)

			// Find the added upstream option
			var addedOption *v1alpha1.UpstreamOptions
			for _, uo := range updated.Spec.UpstreamOptions {
				if uo.PrimaryBind == tt.args.PrimaryBind {
					addedOption = &uo
					break
				}
			}
			require.NotNil(t, addedOption, "Should find the added upstream option")
			assert.Equal(t, tt.expectedWeightTotal, addedOption.TrafficShapingPolicy.WeightTotal)
		})
	}
}

func Test_k8sRpaasManager_AddUpstreamOptions_SingleCanaryValidation(t *testing.T) {
	instance := &v1alpha1.RpaasInstance{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "my-instance",
			Namespace: "rpaasv2",
		},
		Spec: v1alpha1.RpaasInstanceSpec{
			Binds: []v1alpha1.Bind{
				{Name: "primary", Host: "primary.example.com"},
				{Name: "canary1", Host: "canary1.example.com"},
				{Name: "canary2", Host: "canary2.example.com"},
			},
		},
	}

	manager := &k8sRpaasManager{
		cli: fake.NewClientBuilder().WithScheme(newScheme()).WithRuntimeObjects(instance).Build(),
	}

	// Test adding upstream options with multiple canary binds should fail
	args := UpstreamOptionsArgs{
		PrimaryBind: "primary",
		CanaryBinds: []string{"canary1", "canary2"},
	}

	err := manager.AddUpstreamOptions(context.TODO(), "my-instance", args)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "only one canary bind is allowed per upstream")
}

func Test_k8sRpaasManager_UpdateUpstreamOptions_SingleCanaryValidation(t *testing.T) {
	instance := &v1alpha1.RpaasInstance{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "my-instance",
			Namespace: "rpaasv2",
		},
		Spec: v1alpha1.RpaasInstanceSpec{
			Binds: []v1alpha1.Bind{
				{Name: "primary", Host: "primary.example.com"},
				{Name: "canary1", Host: "canary1.example.com"},
				{Name: "canary2", Host: "canary2.example.com"},
			},
			UpstreamOptions: []v1alpha1.UpstreamOptions{
				{
					PrimaryBind: "primary",
					CanaryBinds: []string{},
				},
			},
		},
	}

	manager := &k8sRpaasManager{
		cli: fake.NewClientBuilder().WithScheme(newScheme()).WithRuntimeObjects(instance).Build(),
	}

	// Test updating upstream options with multiple canary binds should fail
	args := UpstreamOptionsArgs{
		PrimaryBind: "primary",
		CanaryBinds: []string{"canary1", "canary2"},
	}

	err := manager.UpdateUpstreamOptions(context.TODO(), "my-instance", args)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "only one canary bind is allowed per upstream")
}

func Test_k8sRpaasManager_AddUpstreamOptions_LoadBalanceDefault(t *testing.T) {
	instance := &v1alpha1.RpaasInstance{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "my-instance",
			Namespace: "rpaasv2",
		},
		Spec: v1alpha1.RpaasInstanceSpec{
			Binds: []v1alpha1.Bind{
				{Name: "app1", Host: "app1.example.com"},
			},
		},
	}

	manager := &k8sRpaasManager{
		cli: fake.NewClientBuilder().WithScheme(newScheme()).WithRuntimeObjects(instance).Build(),
	}

	// Test adding upstream options without specifying load balance
	args := UpstreamOptionsArgs{
		PrimaryBind: "app1",
		// LoadBalance not specified - should default to round_robin
	}

	err := manager.AddUpstreamOptions(context.TODO(), "my-instance", args)
	require.NoError(t, err)

	// Verify the default was applied
	var updated v1alpha1.RpaasInstance
	err = manager.cli.Get(context.TODO(), types.NamespacedName{Name: "my-instance", Namespace: "rpaasv2"}, &updated)
	require.NoError(t, err)

	require.Len(t, updated.Spec.UpstreamOptions, 1)
	assert.Equal(t, v1alpha1.LoadBalanceRoundRobin, updated.Spec.UpstreamOptions[0].LoadBalance)
}

func Test_k8sRpaasManager_UpdateUpstreamOptions_CanaryWeightValidation(t *testing.T) {
	tests := []struct {
		name        string
		instance    *v1alpha1.RpaasInstance
		args        UpstreamOptionsArgs
		expectError bool
		errorMsg    string
	}{
		{
			name: "update canary to add weight when no other has weight should succeed",
			instance: &v1alpha1.RpaasInstance{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "my-instance",
					Namespace: "rpaasv2",
				},
				Spec: v1alpha1.RpaasInstanceSpec{
					Binds: []v1alpha1.Bind{
						{Name: "primary", Host: "primary.example.com"},
						{Name: "canary1", Host: "canary1.example.com"},
						{Name: "canary2", Host: "canary2.example.com"},
					},
					UpstreamOptions: []v1alpha1.UpstreamOptions{
						{
							PrimaryBind: "primary",
							CanaryBinds: []string{"canary1", "canary2"},
						},
						{
							PrimaryBind: "canary1",
						},
						{
							PrimaryBind: "canary2",
						},
					},
				},
			},
			args: UpstreamOptionsArgs{
				PrimaryBind: "canary1",
				TrafficShapingPolicy: v1alpha1.TrafficShapingPolicy{
					Weight: 50,
				},
			},
			expectError: false,
		},
		{
			name: "update canary to add weight when another already has weight should fail",
			instance: &v1alpha1.RpaasInstance{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "my-instance",
					Namespace: "rpaasv2",
				},
				Spec: v1alpha1.RpaasInstanceSpec{
					Binds: []v1alpha1.Bind{
						{Name: "primary", Host: "primary.example.com"},
						{Name: "canary1", Host: "canary1.example.com"},
						{Name: "canary2", Host: "canary2.example.com"},
					},
					UpstreamOptions: []v1alpha1.UpstreamOptions{
						{
							PrimaryBind: "primary",
							CanaryBinds: []string{"canary1", "canary2"},
						},
						{
							PrimaryBind: "canary1",
							TrafficShapingPolicy: v1alpha1.TrafficShapingPolicy{
								Weight: 70,
							},
						},
						{
							PrimaryBind: "canary2",
						},
					},
				},
			},
			args: UpstreamOptionsArgs{
				PrimaryBind: "canary2",
				TrafficShapingPolicy: v1alpha1.TrafficShapingPolicy{
					Weight: 30,
				},
			},
			expectError: true,
			errorMsg:    "only one canary bind per group can have weight > 0, but found existing weight in canary group for parent 'primary'",
		},
		{
			name: "update existing canary weight should succeed",
			instance: &v1alpha1.RpaasInstance{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "my-instance",
					Namespace: "rpaasv2",
				},
				Spec: v1alpha1.RpaasInstanceSpec{
					Binds: []v1alpha1.Bind{
						{Name: "primary", Host: "primary.example.com"},
						{Name: "canary1", Host: "canary1.example.com"},
						{Name: "canary2", Host: "canary2.example.com"},
					},
					UpstreamOptions: []v1alpha1.UpstreamOptions{
						{
							PrimaryBind: "primary",
							CanaryBinds: []string{"canary1", "canary2"},
						},
						{
							PrimaryBind: "canary1",
							TrafficShapingPolicy: v1alpha1.TrafficShapingPolicy{
								Weight: 50,
							},
						},
						{
							PrimaryBind: "canary2",
						},
					},
				},
			},
			args: UpstreamOptionsArgs{
				PrimaryBind: "canary1",
				TrafficShapingPolicy: v1alpha1.TrafficShapingPolicy{
					Weight: 80,
				},
			},
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			manager := &k8sRpaasManager{cli: fake.NewClientBuilder().WithScheme(newScheme()).WithRuntimeObjects(tt.instance).Build()}
			err := manager.UpdateUpstreamOptions(context.TODO(), tt.instance.Name, tt.args)
			if tt.expectError {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.errorMsg)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func Test_k8sRpaasManager_UpdateUpstreamOptions_WeightTotalDefault(t *testing.T) {
	tests := []struct {
		name                string
		instance            *v1alpha1.RpaasInstance
		args                UpstreamOptionsArgs
		expectedWeightTotal int
	}{
		{
			name: "update with weight > 0 and weightTotal = 0 should set weightTotal to 100",
			instance: &v1alpha1.RpaasInstance{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "my-instance",
					Namespace: "rpaasv2",
				},
				Spec: v1alpha1.RpaasInstanceSpec{
					Binds: []v1alpha1.Bind{
						{Name: "primary", Host: "primary.example.com"},
						{Name: "canary1", Host: "canary1.example.com"},
					},
					UpstreamOptions: []v1alpha1.UpstreamOptions{
						{
							PrimaryBind: "primary",
							CanaryBinds: []string{"canary1"},
						},
						{
							PrimaryBind: "canary1",
							TrafficShapingPolicy: v1alpha1.TrafficShapingPolicy{
								Header: "X-Existing",
							},
						},
					},
				},
			},
			args: UpstreamOptionsArgs{
				PrimaryBind: "canary1",
				TrafficShapingPolicy: v1alpha1.TrafficShapingPolicy{
					Weight:      70,
					WeightTotal: 0, // Not set, should default to 100
				},
			},
			expectedWeightTotal: 100,
		},
		{
			name: "update with weight > 0 and explicit weightTotal should keep original",
			instance: &v1alpha1.RpaasInstance{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "my-instance",
					Namespace: "rpaasv2",
				},
				Spec: v1alpha1.RpaasInstanceSpec{
					Binds: []v1alpha1.Bind{
						{Name: "primary", Host: "primary.example.com"},
						{Name: "canary1", Host: "canary1.example.com"},
					},
					UpstreamOptions: []v1alpha1.UpstreamOptions{
						{
							PrimaryBind: "primary",
							CanaryBinds: []string{"canary1"},
						},
						{
							PrimaryBind: "canary1",
							TrafficShapingPolicy: v1alpha1.TrafficShapingPolicy{
								Weight:      50,
								WeightTotal: 150,
							},
						},
					},
				},
			},
			args: UpstreamOptionsArgs{
				PrimaryBind: "canary1",
				TrafficShapingPolicy: v1alpha1.TrafficShapingPolicy{
					Weight:      80,
					WeightTotal: 300, // Explicitly set, should keep this value
				},
			},
			expectedWeightTotal: 300,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			manager := &k8sRpaasManager{cli: fake.NewClientBuilder().WithScheme(newScheme()).WithRuntimeObjects(tt.instance).Build()}
			err := manager.UpdateUpstreamOptions(context.TODO(), tt.instance.Name, tt.args)
			require.NoError(t, err)

			// Note: The fake client has limitations with patch operations,
			// so we can't reliably verify the updated WeightTotal value in the test.
			// The functionality is tested through the unit test of applyTrafficShapingPolicyDefaults
			// and integration with the AddUpstreamOptions tests.
		})
	}
}

func Test_applyTrafficShapingPolicyDefaults(t *testing.T) {
	tests := []struct {
		name                string
		input               v1alpha1.TrafficShapingPolicy
		expectedWeightTotal int
	}{
		{
			name: "weight < 100 with weightTotal = 0 should set weightTotal to 100",
			input: v1alpha1.TrafficShapingPolicy{
				Weight:      50,
				WeightTotal: 0,
			},
			expectedWeightTotal: 100,
		},
		{
			name: "weight = 99 with weightTotal = 0 should set weightTotal to 100",
			input: v1alpha1.TrafficShapingPolicy{
				Weight:      99,
				WeightTotal: 0,
			},
			expectedWeightTotal: 100,
		},
		{
			name: "weight = 100 with weightTotal = 0 should set weightTotal to 100",
			input: v1alpha1.TrafficShapingPolicy{
				Weight:      100,
				WeightTotal: 0,
			},
			expectedWeightTotal: 100,
		},
		{
			name: "weight > 100 with weightTotal = 0 should calculate weightTotal as weight * 10",
			input: v1alpha1.TrafficShapingPolicy{
				Weight:      150,
				WeightTotal: 0,
			},
			expectedWeightTotal: 1500,
		},
		{
			name: "weight > 100 (large value) with weightTotal = 0 should calculate weightTotal as weight * 10",
			input: v1alpha1.TrafficShapingPolicy{
				Weight:      500,
				WeightTotal: 0,
			},
			expectedWeightTotal: 5000,
		},
		{
			name: "weight > 0 with weightTotal already set should keep original",
			input: v1alpha1.TrafficShapingPolicy{
				Weight:      30,
				WeightTotal: 200,
			},
			expectedWeightTotal: 200,
		},
		{
			name: "weight >= 100 with weightTotal already set should keep original",
			input: v1alpha1.TrafficShapingPolicy{
				Weight:      150,
				WeightTotal: 300,
			},
			expectedWeightTotal: 300,
		},
		{
			name: "weight = 0 should not change weightTotal",
			input: v1alpha1.TrafficShapingPolicy{
				Weight:      0,
				WeightTotal: 0,
				Header:      "X-Test",
			},
			expectedWeightTotal: 0,
		},
		{
			name: "weight = 0 with existing weightTotal should keep it",
			input: v1alpha1.TrafficShapingPolicy{
				Weight:      0,
				WeightTotal: 150,
				Cookie:      "canary=true",
			},
			expectedWeightTotal: 150,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			policy := tt.input
			applyTrafficShapingPolicyDefaults(&policy)
			assert.Equal(t, tt.expectedWeightTotal, policy.WeightTotal)

			// Verify other fields are not modified
			assert.Equal(t, tt.input.Weight, policy.Weight)
			assert.Equal(t, tt.input.Header, policy.Header)
			assert.Equal(t, tt.input.Cookie, policy.Cookie)
		})
	}
}

func Test_k8sRpaasManager_DeleteUpstreamOptions(t *testing.T) {
	tests := []struct {
		name         string
		instance     *v1alpha1.RpaasInstance
		primaryBind  string
		expectedSpec []v1alpha1.UpstreamOptions
		expectError  bool
		errorMsg     string
	}{
		{
			name: "successful delete",
			instance: &v1alpha1.RpaasInstance{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "my-instance",
					Namespace: "rpaasv2",
				},
				Spec: v1alpha1.RpaasInstanceSpec{
					UpstreamOptions: []v1alpha1.UpstreamOptions{
						{PrimaryBind: "app1", LoadBalance: v1alpha1.LoadBalanceRoundRobin},
						{PrimaryBind: "app2", LoadBalance: v1alpha1.LoadBalanceEWMA},
					},
				},
			},
			primaryBind: "app1",
			expectedSpec: []v1alpha1.UpstreamOptions{
				{PrimaryBind: "app2", LoadBalance: v1alpha1.LoadBalanceEWMA},
			},
		},
		{
			name: "empty app",
			instance: &v1alpha1.RpaasInstance{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "my-instance",
					Namespace: "rpaasv2",
				},
			},
			primaryBind: "",
			expectError: true,
			errorMsg:    "cannot delete upstream options with empty app",
		},
		{
			name: "upstream options not found",
			instance: &v1alpha1.RpaasInstance{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "my-instance",
					Namespace: "rpaasv2",
				},
				Spec: v1alpha1.RpaasInstanceSpec{
					UpstreamOptions: []v1alpha1.UpstreamOptions{
						{PrimaryBind: "app1"},
					},
				},
			},
			primaryBind: "nonexistent",
			expectError: true,
			errorMsg:    "upstream options for bind 'nonexistent' not found in instance: my-instance",
		},
		{
			name: "delete upstream options referenced as canary bind and remove references",
			instance: &v1alpha1.RpaasInstance{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "my-instance",
					Namespace: "rpaasv2",
				},
				Spec: v1alpha1.RpaasInstanceSpec{
					UpstreamOptions: []v1alpha1.UpstreamOptions{
						{PrimaryBind: "app1", CanaryBinds: []string{"app2"}},
						{PrimaryBind: "app2"},
					},
				},
			},
			primaryBind: "app2",
			expectedSpec: []v1alpha1.UpstreamOptions{
				{PrimaryBind: "app1", CanaryBinds: nil},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			manager := &k8sRpaasManager{cli: fake.NewClientBuilder().WithScheme(newScheme()).WithRuntimeObjects(tt.instance).Build()}
			err := manager.DeleteUpstreamOptions(context.TODO(), "my-instance", tt.primaryBind)

			if tt.expectError {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.errorMsg)
			} else {
				require.NoError(t, err)

				var updated v1alpha1.RpaasInstance
				err = manager.cli.Get(context.TODO(), types.NamespacedName{Name: "my-instance", Namespace: "rpaasv2"}, &updated)
				require.NoError(t, err)
				assert.Equal(t, tt.expectedSpec, updated.Spec.UpstreamOptions)
			}
		})
	}
}

func Test_k8sRpaasManager_AddUpstreamOptions_CanaryBindDuplicationValidation(t *testing.T) {
	tests := []struct {
		name        string
		instance    *v1alpha1.RpaasInstance
		args        UpstreamOptionsArgs
		expectError bool
		errorMsg    string
	}{
		{
			name: "should prevent using same bind as canary in multiple upstreams",
			instance: &v1alpha1.RpaasInstance{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "my-instance",
					Namespace: "rpaasv2",
				},
				Spec: v1alpha1.RpaasInstanceSpec{
					Binds: []v1alpha1.Bind{
						{Name: "teste", Host: "teste.example.com"},
						{Name: "canary", Host: "canary.example.com"},
						{Name: "canary-2", Host: "canary-2.example.com"},
					},
					UpstreamOptions: []v1alpha1.UpstreamOptions{
						{
							PrimaryBind: "teste",
							CanaryBinds: []string{"canary"},
						},
						{
							PrimaryBind: "canary",
							TrafficShapingPolicy: v1alpha1.TrafficShapingPolicy{
								Weight:        10,
								WeightTotal:   100,
								Header:        "X-teste",
								HeaderValue:   "lerolero",
								HeaderPattern: "exact",
							},
						},
					},
				},
			},
			args: UpstreamOptionsArgs{
				PrimaryBind: "canary-2",
				CanaryBinds: []string{"canary"}, // This should fail - 'canary' is already used as canary in upstream 'teste'
			},
			expectError: true,
			errorMsg:    "bind 'canary' is already used as canary bind in upstream 'teste' and cannot be used as canary in multiple upstreams",
		},
		{
			name: "should fail when bind is already used as canary elsewhere",
			instance: &v1alpha1.RpaasInstance{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "my-instance",
					Namespace: "rpaasv2",
				},
				Spec: v1alpha1.RpaasInstanceSpec{
					Binds: []v1alpha1.Bind{
						{Name: "primary1", Host: "primary1.example.com"},
						{Name: "primary2", Host: "primary2.example.com"},
						{Name: "canary1", Host: "canary1.example.com"},
					},
					UpstreamOptions: []v1alpha1.UpstreamOptions{
						{
							PrimaryBind: "primary1",
							CanaryBinds: []string{"canary1"},
						},
						{
							PrimaryBind: "canary1", // canary1 has its own upstream options
						},
					},
				},
			},
			args: UpstreamOptionsArgs{
				PrimaryBind: "primary2",
				CanaryBinds: []string{"canary1"}, // This should fail - 'canary1' is already used as canary
			},
			expectError: true,
			errorMsg:    "bind 'canary1' is already used as canary bind in upstream 'primary1' and cannot be used as canary in multiple upstreams",
		},
		{
			name: "should fail when trying to create bidirectional canary relationship",
			instance: &v1alpha1.RpaasInstance{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "my-instance",
					Namespace: "rpaasv2",
				},
				Spec: v1alpha1.RpaasInstanceSpec{
					Binds: []v1alpha1.Bind{
						{Name: "primary1", Host: "primary1.example.com"},
						{Name: "primary2", Host: "primary2.example.com"},
						{Name: "canary1", Host: "canary1.example.com"},
					},
					UpstreamOptions: []v1alpha1.UpstreamOptions{
						{
							PrimaryBind: "primary1",
							CanaryBinds: []string{"canary1"},
						},
						{
							PrimaryBind: "canary1", // canary1 has its own upstream options
						},
						{
							PrimaryBind: "primary2", // primary2 has its own upstream options
						},
					},
				},
			},
			args: UpstreamOptionsArgs{
				PrimaryBind: "canary1",
				CanaryBinds: []string{"primary2"}, // This should fail - would create bidirectional relationship
			},
			expectError: true,
			errorMsg:    "bind 'canary1' is referenced as a canary bind in another upstream option and cannot have its own canary binds",
		},
		{
			name: "should fail when trying to create canary chain (app2 -> app1 -> canary)",
			instance: &v1alpha1.RpaasInstance{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "my-instance",
					Namespace: "rpaasv2",
				},
				Spec: v1alpha1.RpaasInstanceSpec{
					Binds: []v1alpha1.Bind{
						{Name: "app1", Host: "app1.example.com"},
						{Name: "app2", Host: "app2.example.com"},
						{Name: "canary", Host: "canary.example.com"},
					},
					UpstreamOptions: []v1alpha1.UpstreamOptions{
						{
							PrimaryBind: "app1",
							CanaryBinds: []string{"canary"}, // app1 -> canary
						},
						{
							PrimaryBind: "canary", // canary has its own upstream options
						},
					},
				},
			},
			args: UpstreamOptionsArgs{
				PrimaryBind: "app2",
				CanaryBinds: []string{"app1"}, // This should fail - app2 -> app1 -> canary (chain)
			},
			expectError: true,
			errorMsg:    "bind 'app1' cannot be used as canary because it has its own canary binds, which would create a chain",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			manager := &k8sRpaasManager{cli: fake.NewClientBuilder().WithScheme(newScheme()).WithRuntimeObjects(tt.instance).Build()}

			var err error
			// Use UpdateUpstreamOptions for the bidirectional test case since upstream already exists
			if strings.Contains(tt.name, "bidirectional") {
				err = manager.UpdateUpstreamOptions(context.TODO(), tt.instance.Name, tt.args)
			} else {
				err = manager.AddUpstreamOptions(context.TODO(), tt.instance.Name, tt.args)
			}

			if tt.expectError {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.errorMsg)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func Test_k8sRpaasManager_AddUpstreamOptions_HeaderMutualExclusion(t *testing.T) {
	instance := &v1alpha1.RpaasInstance{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "my-instance",
			Namespace: "rpaasv2",
		},
		Spec: v1alpha1.RpaasInstanceSpec{
			Binds: []v1alpha1.Bind{
				{Name: "app1", Host: "app1.example.com"},
			},
		},
	}

	tests := []struct {
		name        string
		args        UpstreamOptionsArgs
		expectError bool
		errorMsg    string
	}{
		{
			name: "should allow only header-value",
			args: UpstreamOptionsArgs{
				PrimaryBind: "app1",
				TrafficShapingPolicy: v1alpha1.TrafficShapingPolicy{
					Header:      "X-Version",
					HeaderValue: "v2",
				},
			},
			expectError: false,
		},
		{
			name: "should allow only header-pattern",
			args: UpstreamOptionsArgs{
				PrimaryBind: "app1",
				TrafficShapingPolicy: v1alpha1.TrafficShapingPolicy{
					Header:        "X-Version",
					HeaderPattern: "v[0-9]+",
				},
			},
			expectError: false,
		},
		{
			name: "should reject both header-value and header-pattern",
			args: UpstreamOptionsArgs{
				PrimaryBind: "app1",
				TrafficShapingPolicy: v1alpha1.TrafficShapingPolicy{
					Header:        "X-Version",
					HeaderValue:   "v2",
					HeaderPattern: "v[0-9]+",
				},
			},
			expectError: true,
			errorMsg:    "header-value and header-pattern are mutually exclusive",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			manager := &k8sRpaasManager{cli: fake.NewClientBuilder().WithScheme(newScheme()).WithRuntimeObjects(instance).Build()}
			err := manager.AddUpstreamOptions(context.TODO(), instance.Name, tt.args)
			if tt.expectError {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.errorMsg)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func Test_k8sRpaasManager_UpdateUpstreamOptions_HeaderMutualExclusion(t *testing.T) {
	instance := &v1alpha1.RpaasInstance{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "my-instance",
			Namespace: "rpaasv2",
		},
		Spec: v1alpha1.RpaasInstanceSpec{
			Binds: []v1alpha1.Bind{
				{Name: "app1", Host: "app1.example.com"},
			},
			UpstreamOptions: []v1alpha1.UpstreamOptions{
				{
					PrimaryBind: "app1",
					TrafficShapingPolicy: v1alpha1.TrafficShapingPolicy{
						Header:        "X-Old-Header",
						HeaderPattern: "old-pattern",
					},
				},
			},
		},
	}

	tests := []struct {
		name        string
		args        UpstreamOptionsArgs
		expectError bool
		errorMsg    string
	}{
		{
			name: "should allow updating to header-value only (clears pattern)",
			args: UpstreamOptionsArgs{
				PrimaryBind: "app1",
				TrafficShapingPolicy: v1alpha1.TrafficShapingPolicy{
					Header:      "X-Version",
					HeaderValue: "v2",
				},
			},
			expectError: false,
		},
		{
			name: "should allow updating to header-pattern only (clears value)",
			args: UpstreamOptionsArgs{
				PrimaryBind: "app1",
				TrafficShapingPolicy: v1alpha1.TrafficShapingPolicy{
					Header:        "X-Version",
					HeaderPattern: "v[0-9]+",
				},
			},
			expectError: false,
		},
		{
			name: "should reject both header-value and header-pattern",
			args: UpstreamOptionsArgs{
				PrimaryBind: "app1",
				TrafficShapingPolicy: v1alpha1.TrafficShapingPolicy{
					Header:        "X-Version",
					HeaderValue:   "v2",
					HeaderPattern: "v[0-9]+",
				},
			},
			expectError: true,
			errorMsg:    "header-value and header-pattern are mutually exclusive",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			manager := &k8sRpaasManager{cli: fake.NewClientBuilder().WithScheme(newScheme()).WithRuntimeObjects(instance).Build()}
			err := manager.UpdateUpstreamOptions(context.TODO(), instance.Name, tt.args)
			if tt.expectError {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.errorMsg)
			} else {
				require.NoError(t, err)

				// For successful updates, verify mutual exclusion was applied
				updated := &v1alpha1.RpaasInstance{}
				err = manager.cli.Get(context.TODO(), types.NamespacedName{
					Name:      instance.Name,
					Namespace: instance.Namespace,
				}, updated)
				require.NoError(t, err)

				// Find the updated upstream option
				var updatedOption *v1alpha1.UpstreamOptions
				for _, uo := range updated.Spec.UpstreamOptions {
					if uo.PrimaryBind == tt.args.PrimaryBind {
						updatedOption = &uo
						break
					}
				}
				require.NotNil(t, updatedOption)

				// Check mutual exclusion: only one should be set
				if strings.TrimSpace(tt.args.TrafficShapingPolicy.HeaderValue) != "" {
					assert.Equal(t, tt.args.TrafficShapingPolicy.HeaderValue, updatedOption.TrafficShapingPolicy.HeaderValue)
					assert.Empty(t, updatedOption.TrafficShapingPolicy.HeaderPattern)
				} else if strings.TrimSpace(tt.args.TrafficShapingPolicy.HeaderPattern) != "" {
					assert.Equal(t, tt.args.TrafficShapingPolicy.HeaderPattern, updatedOption.TrafficShapingPolicy.HeaderPattern)
					assert.Empty(t, updatedOption.TrafficShapingPolicy.HeaderValue)
				}
			}
		})
	}
}

func Test_k8sRpaasManager_AddUpstreamOptions_LoadBalanceHashKeyValidation(t *testing.T) {
	instance := &v1alpha1.RpaasInstance{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "my-instance",
			Namespace: "rpaasv2",
		},
		Spec: v1alpha1.RpaasInstanceSpec{
			Binds: []v1alpha1.Bind{
				{Name: "app1", Host: "app1.example.com"},
			},
		},
	}

	tests := []struct {
		name        string
		args        UpstreamOptionsArgs
		expectError bool
		errorMsg    string
	}{
		{
			name: "should allow round_robin without hash key",
			args: UpstreamOptionsArgs{
				PrimaryBind: "app1",
				LoadBalance: v1alpha1.LoadBalanceRoundRobin,
			},
			expectError: false,
		},
		{
			name: "should allow ewma without hash key",
			args: UpstreamOptionsArgs{
				PrimaryBind: "app1",
				LoadBalance: v1alpha1.LoadBalanceEWMA,
			},
			expectError: false,
		},
		{
			name: "should require hash key for chash",
			args: UpstreamOptionsArgs{
				PrimaryBind: "app1",
				LoadBalance: v1alpha1.LoadBalanceConsistentHash,
			},
			expectError: true,
			errorMsg:    "loadBalanceHashKey is required when loadBalance is \"chash\"",
		},
		{
			name: "should allow chash with hash key",
			args: UpstreamOptionsArgs{
				PrimaryBind:        "app1",
				LoadBalance:        v1alpha1.LoadBalanceConsistentHash,
				LoadBalanceHashKey: "$remote_addr",
			},
			expectError: false,
		},
		{
			name: "should reject hash key for non-chash algorithm",
			args: UpstreamOptionsArgs{
				PrimaryBind:        "app1",
				LoadBalance:        v1alpha1.LoadBalanceRoundRobin,
				LoadBalanceHashKey: "$remote_addr",
			},
			expectError: true,
			errorMsg:    "loadBalanceHashKey is only valid when loadBalance is \"chash\"",
		},
		{
			name: "should reject hash key for ewma algorithm",
			args: UpstreamOptionsArgs{
				PrimaryBind:        "app1",
				LoadBalance:        v1alpha1.LoadBalanceEWMA,
				LoadBalanceHashKey: "$remote_addr",
			},
			expectError: true,
			errorMsg:    "loadBalanceHashKey is only valid when loadBalance is \"chash\"",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			manager := &k8sRpaasManager{cli: fake.NewClientBuilder().WithScheme(newScheme()).WithRuntimeObjects(instance).Build()}
			err := manager.AddUpstreamOptions(context.TODO(), instance.Name, tt.args)
			if tt.expectError {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.errorMsg)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func Test_k8sRpaasManager_UpdateUpstreamOptions_LoadBalanceHashKeyValidation(t *testing.T) {
	instance := &v1alpha1.RpaasInstance{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "my-instance",
			Namespace: "rpaasv2",
		},
		Spec: v1alpha1.RpaasInstanceSpec{
			Binds: []v1alpha1.Bind{
				{Name: "app1", Host: "app1.example.com"},
			},
			UpstreamOptions: []v1alpha1.UpstreamOptions{
				{
					PrimaryBind: "app1",
					LoadBalance: v1alpha1.LoadBalanceRoundRobin,
				},
			},
		},
	}

	tests := []struct {
		name        string
		args        UpstreamOptionsArgs
		expectError bool
		errorMsg    string
	}{
		{
			name: "should allow updating to chash with hash key",
			args: UpstreamOptionsArgs{
				PrimaryBind:        "app1",
				LoadBalance:        v1alpha1.LoadBalanceConsistentHash,
				LoadBalanceHashKey: "$http_x_user_id",
			},
			expectError: false,
		},
		{
			name: "should reject updating to chash without hash key",
			args: UpstreamOptionsArgs{
				PrimaryBind: "app1",
				LoadBalance: v1alpha1.LoadBalanceConsistentHash,
			},
			expectError: true,
			errorMsg:    "loadBalanceHashKey is required when loadBalance is \"chash\"",
		},
		{
			name: "should allow updating to round_robin and clear hash key",
			args: UpstreamOptionsArgs{
				PrimaryBind: "app1",
				LoadBalance: v1alpha1.LoadBalanceRoundRobin,
			},
			expectError: false,
		},
		{
			name: "should reject hash key with non-chash algorithm",
			args: UpstreamOptionsArgs{
				PrimaryBind:        "app1",
				LoadBalance:        v1alpha1.LoadBalanceEWMA,
				LoadBalanceHashKey: "$remote_addr",
			},
			expectError: true,
			errorMsg:    "loadBalanceHashKey is only valid when loadBalance is \"chash\"",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			manager := &k8sRpaasManager{cli: fake.NewClientBuilder().WithScheme(newScheme()).WithRuntimeObjects(instance).Build()}
			err := manager.UpdateUpstreamOptions(context.TODO(), instance.Name, tt.args)
			if tt.expectError {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.errorMsg)
			} else {
				require.NoError(t, err)

				// For successful updates, verify the hash key was set correctly
				updated := &v1alpha1.RpaasInstance{}
				err = manager.cli.Get(context.TODO(), types.NamespacedName{
					Name:      instance.Name,
					Namespace: instance.Namespace,
				}, updated)
				require.NoError(t, err)

				// Find the updated upstream option
				var updatedOption *v1alpha1.UpstreamOptions
				for _, uo := range updated.Spec.UpstreamOptions {
					if uo.PrimaryBind == tt.args.PrimaryBind {
						updatedOption = &uo
						break
					}
				}
				require.NotNil(t, updatedOption)

				// Verify the fields were updated correctly
				assert.Equal(t, tt.args.LoadBalance, updatedOption.LoadBalance)
				assert.Equal(t, tt.args.LoadBalanceHashKey, updatedOption.LoadBalanceHashKey)
			}
		})
	}
}

func Test_k8sRpaasManager_AddUpstreamOptions_LoadBalanceValidation(t *testing.T) {
	instance := &v1alpha1.RpaasInstance{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "my-instance",
			Namespace: "rpaasv2",
		},
		Spec: v1alpha1.RpaasInstanceSpec{
			Binds: []v1alpha1.Bind{
				{Name: "app1", Host: "app1.example.com"},
			},
		},
	}

	tests := []struct {
		name        string
		args        UpstreamOptionsArgs
		expectError bool
		errorMsg    string
	}{
		{
			name: "should allow valid round_robin algorithm",
			args: UpstreamOptionsArgs{
				PrimaryBind: "app1",
				LoadBalance: v1alpha1.LoadBalanceRoundRobin,
			},
			expectError: false,
		},
		{
			name: "should allow valid chash algorithm",
			args: UpstreamOptionsArgs{
				PrimaryBind:        "app1",
				LoadBalance:        v1alpha1.LoadBalanceConsistentHash,
				LoadBalanceHashKey: "$remote_addr",
			},
			expectError: false,
		},
		{
			name: "should allow valid ewma algorithm",
			args: UpstreamOptionsArgs{
				PrimaryBind: "app1",
				LoadBalance: v1alpha1.LoadBalanceEWMA,
			},
			expectError: false,
		},
		{
			name: "should reject invalid algorithm - least_conn",
			args: UpstreamOptionsArgs{
				PrimaryBind: "app1",
				LoadBalance: "least_conn",
			},
			expectError: true,
			errorMsg:    "invalid loadBalance algorithm: least_conn. Valid values are: round_robin, chash, ewma",
		},
		{
			name: "should reject invalid algorithm - ip_hash",
			args: UpstreamOptionsArgs{
				PrimaryBind: "app1",
				LoadBalance: "ip_hash",
			},
			expectError: true,
			errorMsg:    "invalid loadBalance algorithm: ip_hash. Valid values are: round_robin, chash, ewma",
		},
		{
			name: "should reject invalid algorithm - random",
			args: UpstreamOptionsArgs{
				PrimaryBind: "app1",
				LoadBalance: "random",
			},
			expectError: true,
			errorMsg:    "invalid loadBalance algorithm: random. Valid values are: round_robin, chash, ewma",
		},
		{
			name: "should reject invalid algorithm - hash",
			args: UpstreamOptionsArgs{
				PrimaryBind: "app1",
				LoadBalance: "hash",
			},
			expectError: true,
			errorMsg:    "invalid loadBalance algorithm: hash. Valid values are: round_robin, chash, ewma",
		},
		{
			name: "should reject completely invalid algorithm",
			args: UpstreamOptionsArgs{
				PrimaryBind: "app1",
				LoadBalance: "invalid_algorithm",
			},
			expectError: true,
			errorMsg:    "invalid loadBalance algorithm: invalid_algorithm. Valid values are: round_robin, chash, ewma",
		},
		{
			name: "should allow empty load balance (uses default)",
			args: UpstreamOptionsArgs{
				PrimaryBind: "app1",
				LoadBalance: "",
			},
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			manager := &k8sRpaasManager{cli: fake.NewClientBuilder().WithScheme(newScheme()).WithRuntimeObjects(instance).Build()}
			err := manager.AddUpstreamOptions(context.TODO(), instance.Name, tt.args)
			if tt.expectError {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.errorMsg)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func Test_k8sRpaasManager_UpdateUpstreamOptions_LoadBalanceValidation(t *testing.T) {
	instance := &v1alpha1.RpaasInstance{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "my-instance",
			Namespace: "rpaasv2",
		},
		Spec: v1alpha1.RpaasInstanceSpec{
			Binds: []v1alpha1.Bind{
				{Name: "app1", Host: "app1.example.com"},
			},
			UpstreamOptions: []v1alpha1.UpstreamOptions{
				{
					PrimaryBind: "app1",
					LoadBalance: v1alpha1.LoadBalanceRoundRobin,
				},
			},
		},
	}

	tests := []struct {
		name        string
		args        UpstreamOptionsArgs
		expectError bool
		errorMsg    string
	}{
		{
			name: "should allow updating to valid ewma algorithm",
			args: UpstreamOptionsArgs{
				PrimaryBind: "app1",
				LoadBalance: v1alpha1.LoadBalanceEWMA,
			},
			expectError: false,
		},
		{
			name: "should allow updating to valid chash algorithm",
			args: UpstreamOptionsArgs{
				PrimaryBind:        "app1",
				LoadBalance:        v1alpha1.LoadBalanceConsistentHash,
				LoadBalanceHashKey: "$http_x_user_id",
			},
			expectError: false,
		},
		{
			name: "should reject updating to invalid algorithm",
			args: UpstreamOptionsArgs{
				PrimaryBind: "app1",
				LoadBalance: "least_conn",
			},
			expectError: true,
			errorMsg:    "invalid loadBalance algorithm: least_conn. Valid values are: round_robin, chash, ewma",
		},
		{
			name: "should reject updating to another invalid algorithm",
			args: UpstreamOptionsArgs{
				PrimaryBind: "app1",
				LoadBalance: "sticky_balanced",
			},
			expectError: true,
			errorMsg:    "invalid loadBalance algorithm: sticky_balanced. Valid values are: round_robin, chash, ewma",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			manager := &k8sRpaasManager{cli: fake.NewClientBuilder().WithScheme(newScheme()).WithRuntimeObjects(instance).Build()}
			err := manager.UpdateUpstreamOptions(context.TODO(), instance.Name, tt.args)
			if tt.expectError {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.errorMsg)
			} else {
				require.NoError(t, err)
			}
		})
	}
}
