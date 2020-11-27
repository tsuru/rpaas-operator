// Copyright 2019 tsuru authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package controllers

import (
	"context"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	nginxv1alpha1 "github.com/tsuru/nginx-operator/api/v1alpha1"
	autoscalingv2beta2 "k8s.io/api/autoscaling/v2beta2"
	batchv1beta1 "k8s.io/api/batch/v1beta1"
	corev1 "k8s.io/api/core/v1"
	k8sErrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"github.com/tsuru/rpaas-operator/api/v1alpha1"
	"github.com/tsuru/rpaas-operator/internal/config"
	extensionsruntime "github.com/tsuru/rpaas-operator/pkg/runtime"
)

func Test_mergePlans(t *testing.T) {
	tests := []struct {
		base     v1alpha1.RpaasPlanSpec
		override v1alpha1.RpaasPlanSpec
		expected v1alpha1.RpaasPlanSpec
	}{
		{},
		{
			base: v1alpha1.RpaasPlanSpec{
				Image:       "img0",
				Description: "a",
				Config: v1alpha1.NginxConfig{
					User:         "root",
					CacheEnabled: v1alpha1.Bool(true),
				},
			},
			override: v1alpha1.RpaasPlanSpec{
				Image: "img1",
			},
			expected: v1alpha1.RpaasPlanSpec{
				Image:       "img1",
				Description: "a",
				Config: v1alpha1.NginxConfig{
					User:         "root",
					CacheEnabled: v1alpha1.Bool(true),
				},
			},
		},
		{
			base: v1alpha1.RpaasPlanSpec{
				Image:       "img0",
				Description: "a",
				Config: v1alpha1.NginxConfig{
					User:         "root",
					CacheSize:    resourceMustParsePtr("10M"),
					CacheEnabled: v1alpha1.Bool(true),
				},
			},
			override: v1alpha1.RpaasPlanSpec{
				Image: "img1",
				Config: v1alpha1.NginxConfig{
					User: "ubuntu",
				},
			},
			expected: v1alpha1.RpaasPlanSpec{
				Image:       "img1",
				Description: "a",
				Config: v1alpha1.NginxConfig{
					User:         "ubuntu",
					CacheSize:    resourceMustParsePtr("10M"),
					CacheEnabled: v1alpha1.Bool(true),
				},
			},
		},
		{
			base: v1alpha1.RpaasPlanSpec{
				Image:       "img0",
				Description: "a",
				Config: v1alpha1.NginxConfig{
					User:         "root",
					CacheSize:    resourceMustParsePtr("10M"),
					CacheEnabled: v1alpha1.Bool(true),
				},
			},
			override: v1alpha1.RpaasPlanSpec{
				Image: "img1",
				Config: v1alpha1.NginxConfig{
					User:         "ubuntu",
					CacheEnabled: v1alpha1.Bool(false),
				},
			},
			expected: v1alpha1.RpaasPlanSpec{
				Image:       "img1",
				Description: "a",
				Config: v1alpha1.NginxConfig{
					User:         "ubuntu",
					CacheSize:    resourceMustParsePtr("10M"),
					CacheEnabled: v1alpha1.Bool(false),
				},
			},
		},
		{
			base: v1alpha1.RpaasPlanSpec{
				Image: "img0",
				Resources: corev1.ResourceRequirements{
					Limits: corev1.ResourceList{
						corev1.ResourceCPU:    resource.MustParse("100m"),
						corev1.ResourceMemory: resource.MustParse("100Mi"),
					},
				},
			},
			override: v1alpha1.RpaasPlanSpec{
				Image: "img1",
				Resources: corev1.ResourceRequirements{
					Limits: corev1.ResourceList{
						corev1.ResourceMemory: resource.MustParse("200Mi"),
					},
				},
			},
			expected: v1alpha1.RpaasPlanSpec{
				Image: "img1",
				Resources: corev1.ResourceRequirements{
					Limits: corev1.ResourceList{
						corev1.ResourceCPU:    resource.MustParse("100m"),
						corev1.ResourceMemory: resource.MustParse("200Mi"),
					},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run("", func(t *testing.T) {
			result, err := mergePlans(tt.base, tt.override)
			require.NoError(t, err)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func Test_mergeInstance(t *testing.T) {
	tests := []struct {
		base     v1alpha1.RpaasInstanceSpec
		override v1alpha1.RpaasInstanceSpec
		expected v1alpha1.RpaasInstanceSpec
	}{
		{},
		{
			base:     v1alpha1.RpaasInstanceSpec{AllocateContainerPorts: v1alpha1.Bool(false)},
			override: v1alpha1.RpaasInstanceSpec{AllocateContainerPorts: v1alpha1.Bool(true)},
			expected: v1alpha1.RpaasInstanceSpec{AllocateContainerPorts: v1alpha1.Bool(true)},
		},
		{
			base:     v1alpha1.RpaasInstanceSpec{AllocateContainerPorts: v1alpha1.Bool(false)},
			override: v1alpha1.RpaasInstanceSpec{AllocateContainerPorts: v1alpha1.Bool(false)},
			expected: v1alpha1.RpaasInstanceSpec{AllocateContainerPorts: v1alpha1.Bool(false)},
		},
		{
			base:     v1alpha1.RpaasInstanceSpec{AllocateContainerPorts: v1alpha1.Bool(true)},
			override: v1alpha1.RpaasInstanceSpec{AllocateContainerPorts: v1alpha1.Bool(false)},
			expected: v1alpha1.RpaasInstanceSpec{AllocateContainerPorts: v1alpha1.Bool(false)},
		},
		{
			base:     v1alpha1.RpaasInstanceSpec{AllocateContainerPorts: v1alpha1.Bool(true)},
			expected: v1alpha1.RpaasInstanceSpec{AllocateContainerPorts: v1alpha1.Bool(true)},
		},
	}

	for _, tt := range tests {
		t.Run("", func(t *testing.T) {
			merged, err := mergeInstance(tt.base, tt.override)
			require.NoError(t, err)
			assert.Equal(t, tt.expected, merged)
		})
	}
}

func TestReconcileRpaasInstance_getRpaasInstance(t *testing.T) {
	instance1 := newEmptyRpaasInstance()
	instance1.Name = "instance1"

	instance2 := newEmptyRpaasInstance()
	instance2.Name = "instance2"
	instance2.Spec.Flavors = []string{"mint"}
	instance2.Spec.Lifecycle = &nginxv1alpha1.NginxLifecycle{
		PostStart: &nginxv1alpha1.NginxLifecycleHandler{
			Exec: &corev1.ExecAction{
				Command: []string{
					"echo",
					"hello world",
				},
			},
		},
	}
	instance2.Spec.Service = &nginxv1alpha1.NginxService{
		Annotations: map[string]string{
			"some-instance-annotation-key": "blah",
		},
		Labels: map[string]string{
			"some-instance-label-key": "label1",
			"conflict-label":          "instance value",
		},
	}

	instance3 := newEmptyRpaasInstance()
	instance3.Name = "instance3"
	instance3.Spec.Flavors = []string{"mint", "mango"}
	instance3.Spec.Service = &nginxv1alpha1.NginxService{
		Annotations: map[string]string{
			"some-instance-annotation-key": "blah",
		},
		Labels: map[string]string{
			"some-instance-label-key": "label1",
			"conflict-label":          "instance value",
		},
	}

	instance4 := newEmptyRpaasInstance()
	instance4.Name = "instance4"
	instance4.Labels = map[string]string{
		"rpaas_instance": "my-instance-name",
		"rpaas_service":  "my-service-name",
	}
	instance4.Spec.Service = &nginxv1alpha1.NginxService{
		Annotations: map[string]string{
			"some-instance-annotation-key": "my custom value: {{ .Labels.rpaas_service }}/{{ .Labels.rpaas_instance }}/{{ .Name }}",
		},
	}

	instance5 := newEmptyRpaasInstance()
	instance5.Name = "instance5"
	instance5.Spec.Flavors = []string{"banana"}

	mintFlavor := newRpaasFlavor()
	mintFlavor.Name = "mint"
	mintFlavor.Spec.InstanceTemplate = &v1alpha1.RpaasInstanceSpec{
		Service: &nginxv1alpha1.NginxService{
			Annotations: map[string]string{
				"flavored-service-annotation": "v1",
			},
			Labels: map[string]string{
				"flavored-service-label": "v1",
				"conflict-label":         "ignored",
			},
		},
		PodTemplate: nginxv1alpha1.NginxPodTemplateSpec{
			Annotations: map[string]string{
				"flavored-pod-annotation": "v1",
			},
			Labels: map[string]string{
				"flavored-pod-label": "v1",
			},
			HostNetwork: true,
		},
	}

	mangoFlavor := newRpaasFlavor()
	mangoFlavor.Name = "mango"
	mangoFlavor.Spec.InstanceTemplate = &v1alpha1.RpaasInstanceSpec{
		Service: &nginxv1alpha1.NginxService{
			Annotations: map[string]string{
				"mango-service-annotation": "mango",
			},
			Labels: map[string]string{
				"mango-service-label":    "mango",
				"flavored-service-label": "mango",
				"conflict-label":         "ignored",
			},
		},
		PodTemplate: nginxv1alpha1.NginxPodTemplateSpec{
			Annotations: map[string]string{
				"mango-pod-annotation": "mango",
			},
			Labels: map[string]string{
				"mango-pod-label": "mango",
			},
		},
	}

	bananaFlavor := newRpaasFlavor()
	bananaFlavor.Name = "banana"
	bananaFlavor.Spec.InstanceTemplate = &v1alpha1.RpaasInstanceSpec{
		AllocateContainerPorts: v1alpha1.Bool(false),
	}

	defaultFlavor := newRpaasFlavor()
	defaultFlavor.Name = "default"
	defaultFlavor.Spec.Default = true
	defaultFlavor.Spec.InstanceTemplate = &v1alpha1.RpaasInstanceSpec{
		AllocateContainerPorts: v1alpha1.Bool(true),
		Service: &nginxv1alpha1.NginxService{
			Annotations: map[string]string{
				"default-service-annotation": "default",
			},
			Labels: map[string]string{
				"default-service-label":  "default",
				"flavored-service-label": "default",
			},
		},
		PodTemplate: nginxv1alpha1.NginxPodTemplateSpec{
			Annotations: map[string]string{
				"default-pod-annotation": "default",
			},
			Labels: map[string]string{
				"mango-pod-label":   "not-a-mango",
				"default-pod-label": "default",
			},
		},
	}

	resources := []runtime.Object{instance1, instance2, instance3, instance4, instance5, mintFlavor, mangoFlavor, bananaFlavor, defaultFlavor}

	tests := []struct {
		name      string
		objectKey types.NamespacedName
		expected  v1alpha1.RpaasInstance
	}{
		{
			name:      "when the fetched RpaasInstance has no flavor provided it should merge with default flavors only",
			objectKey: types.NamespacedName{Name: instance1.Name, Namespace: instance1.Namespace},
			expected: v1alpha1.RpaasInstance{
				TypeMeta: metav1.TypeMeta{
					APIVersion: "extensions.tsuru.io/v1alpha1",
					Kind:       "RpaasInstance",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:      instance1.Name,
					Namespace: instance1.Namespace,
				},
				Spec: v1alpha1.RpaasInstanceSpec{
					PlanName:               "my-plan",
					AllocateContainerPorts: v1alpha1.Bool(true),
					Service: &nginxv1alpha1.NginxService{
						Annotations: map[string]string{
							"default-service-annotation": "default",
						},
						Labels: map[string]string{
							"default-service-label":  "default",
							"flavored-service-label": "default",
						},
					},
					PodTemplate: nginxv1alpha1.NginxPodTemplateSpec{
						Annotations: map[string]string{
							"default-pod-annotation": "default",
						},
						Labels: map[string]string{
							"mango-pod-label":   "not-a-mango",
							"default-pod-label": "default",
						},
					},
				},
			},
		},
		{
			name:      "when instance refers to one flavor, the returned instance should be merged with it",
			objectKey: types.NamespacedName{Name: instance2.Name, Namespace: instance2.Namespace},
			expected: v1alpha1.RpaasInstance{
				TypeMeta: metav1.TypeMeta{
					APIVersion: "extensions.tsuru.io/v1alpha1",
					Kind:       "RpaasInstance",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:      instance2.Name,
					Namespace: instance2.Namespace,
				},
				Spec: v1alpha1.RpaasInstanceSpec{
					Flavors:                []string{"mint"},
					PlanName:               "my-plan",
					AllocateContainerPorts: v1alpha1.Bool(true),
					Lifecycle: &nginxv1alpha1.NginxLifecycle{
						PostStart: &nginxv1alpha1.NginxLifecycleHandler{
							Exec: &corev1.ExecAction{
								Command: []string{
									"echo",
									"hello world",
								},
							},
						},
					},
					Service: &nginxv1alpha1.NginxService{
						Annotations: map[string]string{
							"default-service-annotation":   "default",
							"some-instance-annotation-key": "blah",
							"flavored-service-annotation":  "v1",
						},
						Labels: map[string]string{
							"flavored-service-label":  "v1",
							"default-service-label":   "default",
							"some-instance-label-key": "label1",
							"conflict-label":          "instance value",
						},
					},
					PodTemplate: nginxv1alpha1.NginxPodTemplateSpec{
						Annotations: map[string]string{
							"flavored-pod-annotation": "v1",
							"default-pod-annotation":  "default",
						},
						Labels: map[string]string{
							"mango-pod-label":    "not-a-mango",
							"default-pod-label":  "default",
							"flavored-pod-label": "v1",
						},
						HostNetwork: true,
					},
				},
			},
		},
		{
			name: "when the instance refers to more than one flavor, the returned instance spec should be merged with those flavors",
			objectKey: types.NamespacedName{
				Name:      instance3.Name,
				Namespace: instance3.Namespace,
			},
			expected: v1alpha1.RpaasInstance{
				TypeMeta: metav1.TypeMeta{
					APIVersion: "extensions.tsuru.io/v1alpha1",
					Kind:       "RpaasInstance",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:      instance3.Name,
					Namespace: instance3.Namespace,
				},
				Spec: v1alpha1.RpaasInstanceSpec{
					Flavors:                []string{"mint", "mango"},
					PlanName:               "my-plan",
					AllocateContainerPorts: v1alpha1.Bool(true),
					Service: &nginxv1alpha1.NginxService{
						Annotations: map[string]string{
							"default-service-annotation":   "default",
							"some-instance-annotation-key": "blah",
							"flavored-service-annotation":  "v1",
							"mango-service-annotation":     "mango",
						},
						Labels: map[string]string{
							"default-service-label":   "default",
							"some-instance-label-key": "label1",
							"conflict-label":          "instance value",
							"flavored-service-label":  "v1",
							"mango-service-label":     "mango",
						},
					},
					PodTemplate: nginxv1alpha1.NginxPodTemplateSpec{
						Annotations: map[string]string{
							"default-pod-annotation":  "default",
							"flavored-pod-annotation": "v1",
							"mango-pod-annotation":    "mango",
						},
						Labels: map[string]string{
							"flavored-pod-label": "v1",
							"mango-pod-label":    "mango",
							"default-pod-label":  "default",
						},
						HostNetwork: true,
					},
				},
			},
		},
		{
			name: "when service annotations have custom values, should render them",
			objectKey: types.NamespacedName{
				Name:      instance4.Name,
				Namespace: instance4.Namespace,
			},
			expected: v1alpha1.RpaasInstance{
				TypeMeta: metav1.TypeMeta{
					APIVersion: "extensions.tsuru.io/v1alpha1",
					Kind:       "RpaasInstance",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:      instance4.Name,
					Namespace: instance4.Namespace,
					Labels: map[string]string{
						"rpaas_instance": "my-instance-name",
						"rpaas_service":  "my-service-name",
					},
				},
				Spec: v1alpha1.RpaasInstanceSpec{
					PlanName:               "my-plan",
					AllocateContainerPorts: v1alpha1.Bool(true),
					Service: &nginxv1alpha1.NginxService{
						Annotations: map[string]string{
							"default-service-annotation":   "default",
							"some-instance-annotation-key": "my custom value: my-service-name/my-instance-name/instance4",
						},
						Labels: map[string]string{
							"default-service-label":  "default",
							"flavored-service-label": "default",
						},
					},
					PodTemplate: nginxv1alpha1.NginxPodTemplateSpec{
						Annotations: map[string]string{
							"default-pod-annotation": "default",
						},
						Labels: map[string]string{
							"mango-pod-label":   "not-a-mango",
							"default-pod-label": "default",
						},
					},
				},
			},
		},
		{
			name:      "when default flavor has container port allocations enabled but flavor turn off it",
			objectKey: types.NamespacedName{Name: instance5.Name, Namespace: instance5.Namespace},
			expected: v1alpha1.RpaasInstance{
				TypeMeta: metav1.TypeMeta{
					APIVersion: "extensions.tsuru.io/v1alpha1",
					Kind:       "RpaasInstance",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:      instance5.Name,
					Namespace: instance5.Namespace,
				},
				Spec: v1alpha1.RpaasInstanceSpec{
					PlanName:               "my-plan",
					Flavors:                []string{"banana"},
					AllocateContainerPorts: v1alpha1.Bool(false),
					Service: &nginxv1alpha1.NginxService{
						Annotations: map[string]string{
							"default-service-annotation": "default",
						},
						Labels: map[string]string{
							"default-service-label":  "default",
							"flavored-service-label": "default",
						},
					},
					PodTemplate: nginxv1alpha1.NginxPodTemplateSpec{
						Annotations: map[string]string{
							"default-pod-annotation": "default",
						},
						Labels: map[string]string{
							"mango-pod-label":   "not-a-mango",
							"default-pod-label": "default",
						},
					},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			reconciler := newRpaasInstanceReconciler(resources...)
			instance, err := reconciler.getRpaasInstance(context.TODO(), tt.objectKey)
			require.NoError(t, err)
			assert.Equal(t, tt.expected, *instance)
		})
	}
}

func Test_reconcileHPA(t *testing.T) {
	instance1 := newEmptyRpaasInstance()
	instance1.Name = "instance-1"
	instance1.Spec.Autoscale = &v1alpha1.RpaasInstanceAutoscaleSpec{
		MaxReplicas:                       int32(25),
		MinReplicas:                       int32Ptr(4),
		TargetCPUUtilizationPercentage:    int32Ptr(75),
		TargetMemoryUtilizationPercentage: int32Ptr(90),
	}

	instance2 := newEmptyRpaasInstance()
	instance2.Name = "instance-2"

	hpa2 := &autoscalingv2beta2.HorizontalPodAutoscaler{
		TypeMeta: metav1.TypeMeta{
			Kind:       "HorizontalPodAutoscaler",
			APIVersion: "autoscaling/v2beta2",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      instance2.Name,
			Namespace: instance2.Namespace,
		},
		Spec: autoscalingv2beta2.HorizontalPodAutoscalerSpec{
			ScaleTargetRef: autoscalingv2beta2.CrossVersionObjectReference{
				APIVersion: "extensions.tsuru.io/v1alpha1",
				Kind:       "RpaasInstance",
				Name:       instance2.Name,
			},
			MinReplicas: int32Ptr(5),
			MaxReplicas: int32(100),
			Metrics: []autoscalingv2beta2.MetricSpec{
				{
					Type: autoscalingv2beta2.ResourceMetricSourceType,
					Resource: &autoscalingv2beta2.ResourceMetricSource{
						Name: corev1.ResourceCPU,
						Target: autoscalingv2beta2.MetricTarget{
							Type:               autoscalingv2beta2.UtilizationMetricType,
							AverageUtilization: int32Ptr(75),
						},
					},
				},
			},
		},
	}

	resources := []runtime.Object{instance1, instance2, hpa2}

	tests := []struct {
		name      string
		instance  *v1alpha1.RpaasInstance
		assertion func(t *testing.T, err error, got *autoscalingv2beta2.HorizontalPodAutoscaler)
	}{
		{
			name:     "when there is HPA resource but autoscale spec is nil",
			instance: instance2,
			assertion: func(t *testing.T, err error, got *autoscalingv2beta2.HorizontalPodAutoscaler) {
				require.Error(t, err)
				assert.True(t, k8sErrors.IsNotFound(err))
			},
		},
		{
			name:     "when there is no HPA resource but autoscale spec is provided",
			instance: instance1,
			assertion: func(t *testing.T, err error, got *autoscalingv2beta2.HorizontalPodAutoscaler) {
				require.NoError(t, err)
				require.NotNil(t, got)
				assert.Equal(t, int32(25), got.Spec.MaxReplicas)
				assert.Equal(t, int32Ptr(4), got.Spec.MinReplicas)
				assert.Equal(t, autoscalingv2beta2.CrossVersionObjectReference{
					APIVersion: "extensions.tsuru.io/v1alpha1",
					Kind:       "RpaasInstance",
					Name:       "instance-1",
				}, got.Spec.ScaleTargetRef)
				require.Len(t, got.Spec.Metrics, 2)
				assert.Equal(t, autoscalingv2beta2.MetricSpec{
					Type: autoscalingv2beta2.ResourceMetricSourceType,
					Resource: &autoscalingv2beta2.ResourceMetricSource{
						Name: corev1.ResourceCPU,
						Target: autoscalingv2beta2.MetricTarget{
							Type:               autoscalingv2beta2.UtilizationMetricType,
							AverageUtilization: int32Ptr(75),
						},
					},
				}, got.Spec.Metrics[0])
				assert.Equal(t, autoscalingv2beta2.MetricSpec{
					Type: autoscalingv2beta2.ResourceMetricSourceType,
					Resource: &autoscalingv2beta2.ResourceMetricSource{
						Name: corev1.ResourceMemory,
						Target: autoscalingv2beta2.MetricTarget{
							Type:               autoscalingv2beta2.UtilizationMetricType,
							AverageUtilization: int32Ptr(90),
						},
					},
				}, got.Spec.Metrics[1])

				assert.Equal(t, map[string]string{
					"rpaas.extensions.tsuru.io/instance-name": "instance-1",
					"rpaas.extensions.tsuru.io/plan-name":     "my-plan",
				}, got.ObjectMeta.Labels)
			},
		},
		{
			name: "when there is HPA resource but differs from autoscale spec",
			instance: &v1alpha1.RpaasInstance{
				TypeMeta: metav1.TypeMeta{
					APIVersion: "extensions.tsuru.io/v1alpha1",
					Kind:       "RpaasInstance",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:      instance2.Name,
					Namespace: instance2.Namespace,
				},
				Spec: v1alpha1.RpaasInstanceSpec{
					Replicas: int32Ptr(2),
					Autoscale: &v1alpha1.RpaasInstanceAutoscaleSpec{
						MaxReplicas:                       int32(200),
						TargetCPUUtilizationPercentage:    int32Ptr(60),
						TargetMemoryUtilizationPercentage: int32Ptr(85),
					},
				},
			},
			assertion: func(t *testing.T, err error, got *autoscalingv2beta2.HorizontalPodAutoscaler) {
				require.NoError(t, err)
				require.NotNil(t, got)
				assert.Equal(t, int32(200), got.Spec.MaxReplicas)
				assert.Equal(t, int32Ptr(2), got.Spec.MinReplicas)
				assert.Equal(t, autoscalingv2beta2.CrossVersionObjectReference{
					APIVersion: "extensions.tsuru.io/v1alpha1",
					Kind:       "RpaasInstance",
					Name:       "instance-2",
				}, got.Spec.ScaleTargetRef)
				require.Len(t, got.Spec.Metrics, 2)
				assert.Equal(t, autoscalingv2beta2.MetricSpec{
					Type: autoscalingv2beta2.ResourceMetricSourceType,
					Resource: &autoscalingv2beta2.ResourceMetricSource{
						Name: corev1.ResourceCPU,
						Target: autoscalingv2beta2.MetricTarget{
							Type:               autoscalingv2beta2.UtilizationMetricType,
							AverageUtilization: int32Ptr(60),
						},
					},
				}, got.Spec.Metrics[0])
				assert.Equal(t, autoscalingv2beta2.MetricSpec{
					Type: autoscalingv2beta2.ResourceMetricSourceType,
					Resource: &autoscalingv2beta2.ResourceMetricSource{
						Name: corev1.ResourceMemory,
						Target: autoscalingv2beta2.MetricTarget{
							Type:               autoscalingv2beta2.UtilizationMetricType,
							AverageUtilization: int32Ptr(85),
						},
					},
				}, got.Spec.Metrics[1])
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			reconciler := newRpaasInstanceReconciler(resources...)

			err := reconciler.reconcileHPA(context.TODO(), tt.instance)
			require.NoError(t, err)

			hpa := new(autoscalingv2beta2.HorizontalPodAutoscaler)
			if err == nil {
				err = reconciler.Client.Get(context.TODO(), types.NamespacedName{Name: tt.instance.Name, Namespace: tt.instance.Namespace}, hpa)
			}

			tt.assertion(t, err, hpa)
		})
	}
}

func Test_reconcileSnapshotVolume(t *testing.T) {
	ctx := context.TODO()
	rpaasInstance := newEmptyRpaasInstance()
	rpaasInstance.Name = "my-instance"
	rpaasInstance.SetTeamOwner("team-one")

	tests := []struct {
		name     string
		planSpec v1alpha1.RpaasPlanSpec
		assert   func(*testing.T, *corev1.PersistentVolumeClaim)
	}{
		{
			name: "Should repass attributes to PVC",
			planSpec: v1alpha1.RpaasPlanSpec{
				Config: v1alpha1.NginxConfig{
					CacheSize: resourceMustParsePtr("10Gi"),
					CacheSnapshotStorage: v1alpha1.CacheSnapshotStorage{
						StorageClassName: strPtr("my-storage-class"),
					},
				},
			},
			assert: func(t *testing.T, pvc *corev1.PersistentVolumeClaim) {
				assert.Equal(t, pvc.ObjectMeta.OwnerReferences[0].Kind, "RpaasInstance")
				assert.Equal(t, pvc.ObjectMeta.OwnerReferences[0].Name, rpaasInstance.Name)
				assert.Equal(t, pvc.Spec.StorageClassName, strPtr("my-storage-class"))
				assert.Equal(t, pvc.Spec.AccessModes, []corev1.PersistentVolumeAccessMode{corev1.ReadWriteMany})

				parsedSize, _ := resource.ParseQuantity("10Gi")
				assert.Equal(t, parsedSize, pvc.Spec.Resources.Requests["storage"])
			},
		},
		{
			name: "Should repass volume labels to PVC",
			planSpec: v1alpha1.RpaasPlanSpec{
				Config: v1alpha1.NginxConfig{
					CacheSnapshotStorage: v1alpha1.CacheSnapshotStorage{
						StorageClassName: strPtr("my-storage-class"),
						VolumeLabels: map[string]string{
							"some-label":  "foo",
							"other-label": "bar",
						},
					},
				},
			},
			assert: func(t *testing.T, pvc *corev1.PersistentVolumeClaim) {
				assert.Equal(t, 5, len(pvc.ObjectMeta.Labels))
				assert.Equal(t, map[string]string{
					"some-label":           "foo",
					"other-label":          "bar",
					"tsuru.io/volume-team": "team-one",
					"rpaas.extensions.tsuru.io/instance-name": "my-instance",
					"rpaas.extensions.tsuru.io/plan-name":     "my-plan",
				}, pvc.ObjectMeta.Labels)
			},
		},

		{
			name: "Should priorize the team inside plan",
			planSpec: v1alpha1.RpaasPlanSpec{
				Config: v1alpha1.NginxConfig{
					CacheSnapshotStorage: v1alpha1.CacheSnapshotStorage{
						VolumeLabels: map[string]string{
							"tsuru.io/volume-team": "another-team",
						},
					},
				},
			},
			assert: func(t *testing.T, pvc *corev1.PersistentVolumeClaim) {
				assert.Equal(t, "another-team", pvc.ObjectMeta.Labels["tsuru.io/volume-team"])
			},
		},
		{
			name: "Should allow to customize size of PVC separately of cache settings",
			planSpec: v1alpha1.RpaasPlanSpec{
				Config: v1alpha1.NginxConfig{
					CacheSize: resourceMustParsePtr("10Gi"),
					CacheSnapshotStorage: v1alpha1.CacheSnapshotStorage{
						StorageSize: resourceMustParsePtr("100Gi"),
					},
				},
			},
			assert: func(t *testing.T, pvc *corev1.PersistentVolumeClaim) {
				parsedSize, _ := resource.ParseQuantity("100Gi")
				assert.Equal(t, parsedSize, pvc.Spec.Resources.Requests["storage"])
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			reconciler := newRpaasInstanceReconciler()
			err := reconciler.reconcileCacheSnapshotVolume(ctx, rpaasInstance, &v1alpha1.RpaasPlan{Spec: tt.planSpec})
			require.NoError(t, err)

			pvc := &corev1.PersistentVolumeClaim{}
			err = reconciler.Client.Get(ctx, types.NamespacedName{
				Name:      rpaasInstance.Name + "-snapshot-volume",
				Namespace: rpaasInstance.Namespace,
			}, pvc)
			require.NoError(t, err)

			tt.assert(t, pvc)
		})
	}

}

func Test_destroySnapshotVolume(t *testing.T) {
	ctx := context.TODO()
	instance1 := newEmptyRpaasInstance()
	instance1.Name = "instance-1"

	pvc := &corev1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "instance-1-snapshot-volume",
			Namespace: "default",
		},
	}
	reconciler := newRpaasInstanceReconciler(pvc)

	err := reconciler.destroyCacheSnapshotVolume(ctx, instance1)
	require.NoError(t, err)

	pvc = &corev1.PersistentVolumeClaim{}
	err = reconciler.Client.Get(context.TODO(), types.NamespacedName{Name: instance1.Name + "-snapshot-volume", Namespace: instance1.Namespace}, pvc)
	require.True(t, k8sErrors.IsNotFound(err))
}

func Test_reconcileCacheSnapshotCronJobCreation(t *testing.T) {
	ctx := context.TODO()
	instance1 := newEmptyRpaasInstance()
	instance1.Name = "instance-1"

	reconciler := newRpaasInstanceReconciler()

	plan := &v1alpha1.RpaasPlan{
		Spec: v1alpha1.RpaasPlanSpec{},
	}

	err := reconciler.reconcileCacheSnapshotCronJob(ctx, instance1, plan)
	require.NoError(t, err)

	cronJob := &batchv1beta1.CronJob{}
	err = reconciler.Client.Get(context.TODO(), types.NamespacedName{Name: instance1.Name + "-snapshot-cron-job", Namespace: instance1.Namespace}, cronJob)
	require.NoError(t, err)

	assert.Equal(t, "RpaasInstance", cronJob.ObjectMeta.OwnerReferences[0].Kind)
	assert.Equal(t, instance1.Name, cronJob.ObjectMeta.OwnerReferences[0].Name)

	assert.Equal(t, map[string]string{
		"rpaas.extensions.tsuru.io/instance-name": "instance-1",
		"rpaas.extensions.tsuru.io/plan-name":     "my-plan",
	}, cronJob.ObjectMeta.Labels)

	assert.Equal(t, map[string]string{
		"log-app-name":     "instance-1",
		"log-process-name": "cache-synchronize",
		"rpaas.extensions.tsuru.io/instance-name": "instance-1",
		"rpaas.extensions.tsuru.io/plan-name":     "my-plan",
	}, cronJob.Spec.JobTemplate.Spec.Template.ObjectMeta.Labels)
}

func Test_reconcileCacheSnapshotCronJobUpdate(t *testing.T) {
	ctx := context.TODO()
	instance1 := newEmptyRpaasInstance()
	instance1.Name = "instance-1"

	previousCronJob := &batchv1beta1.CronJob{
		ObjectMeta: metav1.ObjectMeta{
			Name: instance1.Name + "-snapshot-cronjob",
		},
		Spec: batchv1beta1.CronJobSpec{
			Schedule: "old-schedule",
		},
	}

	reconciler := newRpaasInstanceReconciler(previousCronJob)

	plan := &v1alpha1.RpaasPlan{
		Spec: v1alpha1.RpaasPlanSpec{
			Config: v1alpha1.NginxConfig{
				CacheSnapshotSync: v1alpha1.CacheSnapshotSyncSpec{
					Schedule: "new-schedule",
				},
			},
		},
	}

	err := reconciler.reconcileCacheSnapshotCronJob(ctx, instance1, plan)
	require.NoError(t, err)

	cronJob := &batchv1beta1.CronJob{}
	err = reconciler.Client.Get(context.TODO(), types.NamespacedName{Name: instance1.Name + "-snapshot-cron-job", Namespace: instance1.Namespace}, cronJob)
	require.NoError(t, err)

	assert.Equal(t, "RpaasInstance", cronJob.ObjectMeta.OwnerReferences[0].Kind)
	assert.Equal(t, instance1.Name, cronJob.ObjectMeta.OwnerReferences[0].Name)
	assert.Equal(t, "new-schedule", cronJob.Spec.Schedule)
}

func Test_destroySnapshotCronJob(t *testing.T) {
	ctx := context.TODO()
	instance1 := newEmptyRpaasInstance()
	instance1.Name = "instance-1"

	cronJob := &batchv1beta1.CronJob{
		ObjectMeta: metav1.ObjectMeta{
			Name:      instance1.Name + "-snapshot-cron-job",
			Namespace: instance1.Namespace,
		},
	}

	reconciler := newRpaasInstanceReconciler(cronJob)

	err := reconciler.destroyCacheSnapshotCronJob(ctx, instance1)
	require.NoError(t, err)

	cronJob = &batchv1beta1.CronJob{}

	err = reconciler.Client.Get(context.TODO(), types.NamespacedName{Name: instance1.Name + "-snapshot-cron-job", Namespace: instance1.Namespace}, cronJob)
	require.True(t, k8sErrors.IsNotFound(err))
}

func int32Ptr(n int32) *int32 {
	return &n
}

func strPtr(s string) *string {
	return &s
}

func newEmptyRpaasInstance() *v1alpha1.RpaasInstance {
	return &v1alpha1.RpaasInstance{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "extensions.tsuru.io/v1alpha1",
			Kind:       "RpaasInstance",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "my-instance",
			Namespace: "default",
		},
		Spec: v1alpha1.RpaasInstanceSpec{
			PlanName: "my-plan",
		},
	}
}

func newRpaasFlavor() *v1alpha1.RpaasFlavor {
	return &v1alpha1.RpaasFlavor{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "extensions.tsuru.io/v1alpha1",
			Kind:       "RpaasFlavor",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "my-flavor",
			Namespace: "default",
		},
		Spec: v1alpha1.RpaasFlavorSpec{},
	}
}

func TestReconcileNginx_reconcileDedicatedPorts(t *testing.T) {
	tests := []struct {
		name      string
		rpaas     *v1alpha1.RpaasInstance
		objects   []runtime.Object
		config    *config.RpaasConfig
		assertion func(t *testing.T, err error, ports []int, portAlloc v1alpha1.RpaasPortAllocationSpec)
	}{
		{
			name: "creates empty port allocation",
			rpaas: &v1alpha1.RpaasInstance{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "my-rpaas",
					Namespace: "default",
				},
				Spec:   v1alpha1.RpaasInstanceSpec{},
				Status: v1alpha1.RpaasInstanceStatus{},
			},
			assertion: func(t *testing.T, err error, ports []int, portAlloc v1alpha1.RpaasPortAllocationSpec) {
				assert.NoError(t, err)
				assert.Nil(t, ports)
				assert.Equal(t, v1alpha1.RpaasPortAllocationSpec{}, portAlloc)
			},
		},
		{
			name: "allocate ports if requested",
			rpaas: &v1alpha1.RpaasInstance{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "my-rpaas",
					Namespace: "default",
					UID:       "1337",
				},
				Spec: v1alpha1.RpaasInstanceSpec{
					AllocateContainerPorts: v1alpha1.Bool(true),
				},
				Status: v1alpha1.RpaasInstanceStatus{},
			},
			assertion: func(t *testing.T, err error, ports []int, portAlloc v1alpha1.RpaasPortAllocationSpec) {
				assert.NoError(t, err)
				assert.Equal(t, []int{20000, 20001}, ports)
				assert.Equal(t, v1alpha1.RpaasPortAllocationSpec{
					Ports: []v1alpha1.AllocatedPort{
						{
							Port: 20000,
							Owner: v1alpha1.NamespacedOwner{
								Namespace: "default",
								RpaasName: "my-rpaas",
								UID:       "1337",
							},
						},
						{
							Port: 20001,
							Owner: v1alpha1.NamespacedOwner{
								Namespace: "default",
								RpaasName: "my-rpaas",
								UID:       "1337",
							},
						},
					},
				}, portAlloc)
			},
		},
		{
			name: "skip already allocated when allocating new ports",
			rpaas: &v1alpha1.RpaasInstance{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "my-rpaas",
					Namespace: "default",
					UID:       "1337",
				},
				Spec: v1alpha1.RpaasInstanceSpec{
					AllocateContainerPorts: v1alpha1.Bool(true),
				},
				Status: v1alpha1.RpaasInstanceStatus{},
			},
			objects: []runtime.Object{
				&v1alpha1.RpaasInstance{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "other-nginx",
						Namespace: "default",
						UID:       "1337-af",
					},
				},
				&v1alpha1.RpaasPortAllocation{
					ObjectMeta: metav1.ObjectMeta{
						Name: "default",
					},
					Spec: v1alpha1.RpaasPortAllocationSpec{
						Ports: []v1alpha1.AllocatedPort{
							{
								Port: 20000,
								Owner: v1alpha1.NamespacedOwner{
									Namespace: "default",
									RpaasName: "other-nginx",
									UID:       "1337-af",
								},
							},
							{
								Port: 20002,
								Owner: v1alpha1.NamespacedOwner{
									Namespace: "default",
									RpaasName: "other-nginx",
									UID:       "1337-af",
								},
							},
						},
					},
				},
			},
			assertion: func(t *testing.T, err error, ports []int, portAlloc v1alpha1.RpaasPortAllocationSpec) {
				assert.NoError(t, err)
				assert.Equal(t, []int{20003, 20004}, ports)
				assert.Equal(t, v1alpha1.RpaasPortAllocationSpec{
					Ports: []v1alpha1.AllocatedPort{
						{
							Port: 20000,
							Owner: v1alpha1.NamespacedOwner{
								Namespace: "default",
								RpaasName: "other-nginx",
								UID:       "1337-af",
							},
						},
						{
							Port: 20002,
							Owner: v1alpha1.NamespacedOwner{
								Namespace: "default",
								RpaasName: "other-nginx",
								UID:       "1337-af",
							},
						},
						{
							Port: 20003,
							Owner: v1alpha1.NamespacedOwner{
								Namespace: "default",
								RpaasName: "my-rpaas",
								UID:       "1337",
							},
						},
						{
							Port: 20004,
							Owner: v1alpha1.NamespacedOwner{
								Namespace: "default",
								RpaasName: "my-rpaas",
								UID:       "1337",
							},
						},
					},
				}, portAlloc)
			},
		},
		{
			name: "reuse previous allocations",
			rpaas: &v1alpha1.RpaasInstance{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "my-rpaas",
					Namespace: "default",
					UID:       "1337",
				},
				Spec: v1alpha1.RpaasInstanceSpec{
					AllocateContainerPorts: v1alpha1.Bool(true),
				},
				Status: v1alpha1.RpaasInstanceStatus{},
			},
			objects: []runtime.Object{
				&v1alpha1.RpaasPortAllocation{
					ObjectMeta: metav1.ObjectMeta{
						Name: "default",
					},
					Spec: v1alpha1.RpaasPortAllocationSpec{
						Ports: []v1alpha1.AllocatedPort{
							{
								Port: 20000,
								Owner: v1alpha1.NamespacedOwner{
									Namespace: "default",
									RpaasName: "my-rpaas",
									UID:       "1337",
								},
							},
							{
								Port: 20002,
								Owner: v1alpha1.NamespacedOwner{
									Namespace: "default",
									RpaasName: "my-rpaas",
									UID:       "1337",
								},
							},
						},
					},
				},
			},
			assertion: func(t *testing.T, err error, ports []int, portAlloc v1alpha1.RpaasPortAllocationSpec) {
				assert.NoError(t, err)
				assert.Equal(t, []int{20000, 20002}, ports)
				assert.Equal(t, v1alpha1.RpaasPortAllocationSpec{
					Ports: []v1alpha1.AllocatedPort{
						{
							Port: 20000,
							Owner: v1alpha1.NamespacedOwner{
								Namespace: "default",
								RpaasName: "my-rpaas",
								UID:       "1337",
							},
						},
						{
							Port: 20002,
							Owner: v1alpha1.NamespacedOwner{
								Namespace: "default",
								RpaasName: "my-rpaas",
								UID:       "1337",
							},
						},
					},
				}, portAlloc)
			},
		},
		{
			name: "remove allocations for removed objects",
			rpaas: &v1alpha1.RpaasInstance{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "my-rpaas",
					Namespace: "default",
					UID:       "1337",
				},
				Spec:   v1alpha1.RpaasInstanceSpec{},
				Status: v1alpha1.RpaasInstanceStatus{},
			},
			objects: []runtime.Object{
				&v1alpha1.RpaasPortAllocation{
					ObjectMeta: metav1.ObjectMeta{
						Name: "default",
					},
					Spec: v1alpha1.RpaasPortAllocationSpec{
						Ports: []v1alpha1.AllocatedPort{
							{
								Port: 20000,
								Owner: v1alpha1.NamespacedOwner{
									Namespace: "default",
									RpaasName: "other-nginx",
									UID:       "1337-af",
								},
							},
							{
								Port: 20002,
								Owner: v1alpha1.NamespacedOwner{
									Namespace: "default",
									RpaasName: "other-nginx",
									UID:       "1337-af",
								},
							},
						},
					},
				},
			},
			assertion: func(t *testing.T, err error, ports []int, portAlloc v1alpha1.RpaasPortAllocationSpec) {
				assert.NoError(t, err)
				assert.Nil(t, ports)
				assert.Equal(t, v1alpha1.RpaasPortAllocationSpec{}, portAlloc)
			},
		},
		{
			name: "remove allocations for objects not matching UID",
			rpaas: &v1alpha1.RpaasInstance{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "my-rpaas",
					Namespace: "default",
					UID:       "1337",
				},
				Spec:   v1alpha1.RpaasInstanceSpec{},
				Status: v1alpha1.RpaasInstanceStatus{},
			},
			objects: []runtime.Object{
				&v1alpha1.RpaasPortAllocation{
					ObjectMeta: metav1.ObjectMeta{
						Name: "default",
					},
					Spec: v1alpha1.RpaasPortAllocationSpec{
						Ports: []v1alpha1.AllocatedPort{
							{
								Port: 20000,
								Owner: v1alpha1.NamespacedOwner{
									Namespace: "default",
									RpaasName: "my-rpaas",
									UID:       "1337-af",
								},
							},
							{
								Port: 20002,
								Owner: v1alpha1.NamespacedOwner{
									Namespace: "default",
									RpaasName: "my-rpaas",
									UID:       "1337-af",
								},
							},
						},
					},
				},
			},
			assertion: func(t *testing.T, err error, ports []int, portAlloc v1alpha1.RpaasPortAllocationSpec) {
				assert.NoError(t, err)
				assert.Nil(t, ports)
				assert.Equal(t, v1alpha1.RpaasPortAllocationSpec{}, portAlloc)
			},
		},
		{
			name: "loops around looking for ports",
			config: &config.RpaasConfig{
				PortRangeMin: 10,
				PortRangeMax: 13,
			},
			rpaas: &v1alpha1.RpaasInstance{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "my-rpaas",
					Namespace: "default",
					UID:       "1234",
				},
				Spec: v1alpha1.RpaasInstanceSpec{
					AllocateContainerPorts: v1alpha1.Bool(true),
				},
				Status: v1alpha1.RpaasInstanceStatus{},
			},
			objects: []runtime.Object{
				&v1alpha1.RpaasInstance{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "other-rpaas",
						Namespace: "default",
						UID:       "1337",
					},
					Spec: v1alpha1.RpaasInstanceSpec{
						AllocateContainerPorts: v1alpha1.Bool(true),
					},
				},
				&v1alpha1.RpaasPortAllocation{
					ObjectMeta: metav1.ObjectMeta{
						Name: "default",
					},
					Spec: v1alpha1.RpaasPortAllocationSpec{
						Ports: []v1alpha1.AllocatedPort{
							{
								Port: 10,
								Owner: v1alpha1.NamespacedOwner{
									Namespace: "default",
									RpaasName: "other-rpaas",
									UID:       "1337",
								},
							},
							{
								Port: 12,
								Owner: v1alpha1.NamespacedOwner{
									Namespace: "default",
									RpaasName: "other-rpaas",
									UID:       "1337",
								},
							},
						},
					},
				},
			},
			assertion: func(t *testing.T, err error, ports []int, portAlloc v1alpha1.RpaasPortAllocationSpec) {
				assert.NoError(t, err)
				assert.Equal(t, []int{13, 11}, ports)
				assert.Equal(t, v1alpha1.RpaasPortAllocationSpec{
					Ports: []v1alpha1.AllocatedPort{
						{
							Port: 10,
							Owner: v1alpha1.NamespacedOwner{
								Namespace: "default",
								RpaasName: "other-rpaas",
								UID:       "1337",
							},
						},
						{
							Port: 12,
							Owner: v1alpha1.NamespacedOwner{
								Namespace: "default",
								RpaasName: "other-rpaas",
								UID:       "1337",
							},
						},
						{
							Port: 13,
							Owner: v1alpha1.NamespacedOwner{
								Namespace: "default",
								RpaasName: "my-rpaas",
								UID:       "1234",
							},
						},
						{
							Port: 11,
							Owner: v1alpha1.NamespacedOwner{
								Namespace: "default",
								RpaasName: "my-rpaas",
								UID:       "1234",
							},
						},
					},
				}, portAlloc)
			},
		},
		{
			name: "errors if no ports are available",
			config: &config.RpaasConfig{
				PortRangeMin: 10,
				PortRangeMax: 11,
			},
			rpaas: &v1alpha1.RpaasInstance{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "my-rpaas",
					Namespace: "default",
					UID:       "1234",
				},
				Spec: v1alpha1.RpaasInstanceSpec{
					AllocateContainerPorts: v1alpha1.Bool(true),
				},
				Status: v1alpha1.RpaasInstanceStatus{},
			},
			objects: []runtime.Object{
				&v1alpha1.RpaasInstance{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "other-rpaas",
						Namespace: "default",
						UID:       "1337",
					},
					Spec: v1alpha1.RpaasInstanceSpec{
						AllocateContainerPorts: v1alpha1.Bool(true),
					},
				},
				&v1alpha1.RpaasPortAllocation{
					ObjectMeta: metav1.ObjectMeta{
						Name: "default",
					},
					Spec: v1alpha1.RpaasPortAllocationSpec{
						Ports: []v1alpha1.AllocatedPort{
							{
								Port: 10,
								Owner: v1alpha1.NamespacedOwner{
									Namespace: "default",
									RpaasName: "other-rpaas",
									UID:       "1337",
								},
							},
							{
								Port: 11,
								Owner: v1alpha1.NamespacedOwner{
									Namespace: "default",
									RpaasName: "other-rpaas",
									UID:       "1337",
								},
							},
						},
					},
				},
			},
			assertion: func(t *testing.T, err error, ports []int, portAlloc v1alpha1.RpaasPortAllocationSpec) {
				assert.Nil(t, ports)
				assert.EqualError(t, err, `unable to allocate container ports, wanted 2, allocated 0`)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := config.Init()
			require.NoError(t, err)
			if tt.config != nil {
				config.Set(*tt.config)
			}
			resources := []runtime.Object{tt.rpaas}
			if tt.objects != nil {
				resources = append(resources, tt.objects...)
			}
			reconciler := newRpaasInstanceReconciler(resources...)
			ports, err := reconciler.reconcileDedicatedPorts(context.Background(), tt.rpaas, 2)
			var allocation v1alpha1.RpaasPortAllocation
			allocErr := reconciler.Client.Get(context.Background(), types.NamespacedName{
				Name: defaultPortAllocationResource,
			}, &allocation)
			require.NoError(t, allocErr)
			tt.assertion(t, err, ports, allocation.Spec)
		})
	}
}

func TestReconcile(t *testing.T) {
	rpaas := &v1alpha1.RpaasInstance{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "my-instance",
			Namespace: "default",
		},
		Spec: v1alpha1.RpaasInstanceSpec{
			PlanName: "my-plan",
		},
	}
	plan := &v1alpha1.RpaasPlan{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "my-plan",
			Namespace: "default",
		},
		Spec: v1alpha1.RpaasPlanSpec{
			Image: "tsuru:mynginx:test",
			Config: v1alpha1.NginxConfig{
				CacheEnabled:         v1alpha1.Bool(true),
				CacheSize:            resourceMustParsePtr("100M"),
				CacheSnapshotEnabled: true,
				CacheSnapshotStorage: v1alpha1.CacheSnapshotStorage{
					StorageClassName: strPtr("my-storage-class"),
				},
				CachePath: "/var/cache/nginx/rpaas",
				CacheSnapshotSync: v1alpha1.CacheSnapshotSyncSpec{
					Schedule: "1 * * * *",
					Image:    "test/test:latest",
					CmdPodToPVC: []string{
						"/bin/bash",
						"-c",
						"echo 'this is a test'",
					},
					CmdPVCToPod: []string{
						"/bin/bash",
						"-c",
						"echo 'this is a the first pod sync'",
					},
				},
			},
		},
	}
	reconciler := newRpaasInstanceReconciler(rpaas, plan)
	result, err := reconciler.Reconcile(reconcile.Request{NamespacedName: types.NamespacedName{Namespace: "default", Name: "my-instance"}})
	require.NoError(t, err)

	assert.Equal(t, result, reconcile.Result{})

	nginx := &nginxv1alpha1.Nginx{}
	err = reconciler.Client.Get(context.TODO(), types.NamespacedName{Name: rpaas.Name, Namespace: rpaas.Namespace}, nginx)
	require.NoError(t, err)

	assert.Equal(t, "cache-snapshot-volume", nginx.Spec.PodTemplate.Volumes[0].Name)
	assert.Equal(t, &corev1.PersistentVolumeClaimVolumeSource{ClaimName: "my-instance-snapshot-volume"}, nginx.Spec.PodTemplate.Volumes[0].PersistentVolumeClaim)
	assert.Equal(t, "cache-snapshot-volume", nginx.Spec.PodTemplate.VolumeMounts[0].Name)
	assert.Equal(t, "/var/cache/cache-snapshot", nginx.Spec.PodTemplate.VolumeMounts[0].MountPath)

	assert.Equal(t, nginx.Spec.PodTemplate.Ports, []corev1.ContainerPort{
		{Name: "nginx-metrics", ContainerPort: 8800, Protocol: "TCP"},
	})
	assert.Equal(t, resource.MustParse("100M"), *nginx.Spec.Cache.Size)

	initContainer := nginx.Spec.PodTemplate.InitContainers[0]
	assert.Equal(t, "restore-snapshot", initContainer.Name)
	assert.Equal(t, "tsuru:mynginx:test", initContainer.Image)
	assert.Equal(t, "/bin/bash", initContainer.Command[0])
	assert.Equal(t, "-c", initContainer.Args[0])
	assert.Equal(t, "echo 'this is a the first pod sync'", initContainer.Args[1])
	assert.Equal(t, []corev1.EnvVar{
		{Name: "SERVICE_NAME", Value: "default"},
		{Name: "INSTANCE_NAME", Value: "my-instance"},
		{Name: "CACHE_SNAPSHOT_MOUNTPOINT", Value: "/var/cache/cache-snapshot"},
		{Name: "CACHE_PATH", Value: "/var/cache/nginx/rpaas"},
		{Name: "POD_CMD", Value: "rsync -avz --recursive --delete --temp-dir=/var/cache/nginx/rpaas/nginx_tmp /var/cache/cache-snapshot/nginx /var/cache/nginx/rpaas"},
	}, initContainer.Env)

	assert.Equal(t, []corev1.VolumeMount{
		{Name: "cache-snapshot-volume", MountPath: "/var/cache/cache-snapshot"},
		{Name: "cache-vol", MountPath: "/var/cache/nginx/rpaas"},
	}, initContainer.VolumeMounts)

	cronJob := &batchv1beta1.CronJob{}
	err = reconciler.Client.Get(context.TODO(), types.NamespacedName{Name: "my-instance-snapshot-cron-job", Namespace: rpaas.Namespace}, cronJob)
	require.NoError(t, err)

	assert.Equal(t, "1 * * * *", cronJob.Spec.Schedule)
	podTemplateSpec := cronJob.Spec.JobTemplate.Spec.Template
	podSpec := podTemplateSpec.Spec
	assert.Equal(t, "test/test:latest", podSpec.Containers[0].Image)
	assert.Equal(t, "/bin/bash", podSpec.Containers[0].Command[0])
	assert.Equal(t, "-c", podSpec.Containers[0].Args[0])
	assert.Equal(t, "echo 'this is a test'", podSpec.Containers[0].Args[1])
	assert.Equal(t, "my-instance", podTemplateSpec.ObjectMeta.Labels["log-app-name"])
	assert.Equal(t, "cache-synchronize", podTemplateSpec.ObjectMeta.Labels["log-process-name"])
	assert.Equal(t, []corev1.EnvVar{
		{Name: "SERVICE_NAME", Value: "default"},
		{Name: "INSTANCE_NAME", Value: "my-instance"},
		{Name: "CACHE_SNAPSHOT_MOUNTPOINT", Value: "/var/cache/cache-snapshot"},
		{Name: "CACHE_PATH", Value: "/var/cache/nginx/rpaas"},
		{Name: "POD_CMD", Value: "rsync -avz --recursive --delete --temp-dir=/var/cache/cache-snapshot/temp /var/cache/nginx/rpaas/nginx /var/cache/cache-snapshot"},
	}, podSpec.Containers[0].Env)
}

func resourceMustParsePtr(fmt string) *resource.Quantity {
	qty := resource.MustParse(fmt)
	return &qty
}

func TestMinutesIntervalToSchedule(t *testing.T) {
	tests := []struct {
		minutes uint32
		want    string
	}{
		{
			want: "*/1 * * * *",
		},
		{
			minutes: 60, // an hour
			want:    "*/60 * * * *",
		},
		{
			minutes: 12 * 60, // a half day
			want:    "*/720 * * * *",
		},
	}

	for _, tt := range tests {
		t.Run(fmt.Sprintf("%d min == %q", tt.minutes, tt.want), func(t *testing.T) {
			have := minutesIntervalToSchedule(tt.minutes)
			assert.Equal(t, tt.want, have)
		})
	}
}

func TestReconcileRpaasInstance_reconcileTLSSessionResumption(t *testing.T) {
	tests := []struct {
		name     string
		instance *v1alpha1.RpaasInstance
		objects  []runtime.Object
		assert   func(t *testing.T, err error, gotSecret *corev1.Secret, gotCronJob *batchv1beta1.CronJob)
	}{
		{
			name: "when no TLS session resumption is enabled",
			instance: &v1alpha1.RpaasInstance{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "my-instance",
					Namespace: "default",
				},
			},
		},
		{
			name: "Session Tickets: default container image + default key length + default rotation interval",
			instance: &v1alpha1.RpaasInstance{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "my-instance",
					Namespace: "default",
				},
				Spec: v1alpha1.RpaasInstanceSpec{
					TLSSessionResumption: &v1alpha1.TLSSessionResumption{
						SessionTicket: &v1alpha1.TLSSessionTicket{},
					},
				},
			},
			assert: func(t *testing.T, err error, gotSecret *corev1.Secret, gotCronJob *batchv1beta1.CronJob) {
				require.NoError(t, err)
				require.NotNil(t, gotSecret)

				expectedKeyLength := 48

				currentTicket, ok := gotSecret.Data["ticket.0.key"]
				require.True(t, ok)
				require.NotEmpty(t, currentTicket)
				require.Len(t, currentTicket, expectedKeyLength)

				require.NotNil(t, gotCronJob)
				assert.Equal(t, "*/60 * * * *", gotCronJob.Spec.Schedule)
				assert.Equal(t, corev1.PodTemplateSpec{
					ObjectMeta: metav1.ObjectMeta{
						Annotations: map[string]string{
							rotateTLSSessionTicketsScriptFilename: rotateTLSSessionTicketsScript,
						},
					},
					Spec: corev1.PodSpec{
						ServiceAccountName: rotateTLSSessionTicketsServiceAccountName,
						RestartPolicy:      corev1.RestartPolicyNever,
						Containers: []corev1.Container{
							{
								Name:    "session-ticket-rotator",
								Image:   defaultRotateTLSSessionTicketsImage,
								Command: []string{"/bin/bash"},
								Args:    []string{rotateTLSSessionTicketsScriptPath},
								Env: []corev1.EnvVar{
									{
										Name:  "SECRET_NAME",
										Value: gotSecret.Name,
									},
									{
										Name:  "SECRET_NAMESPACE",
										Value: gotSecret.Namespace,
									},
									{
										Name:  "SESSION_TICKET_KEY_LENGTH",
										Value: fmt.Sprint(expectedKeyLength),
									},
									{
										Name:  "SESSION_TICKET_KEYS",
										Value: "1",
									},
									{
										Name:  "NGINX_LABEL_SELECTOR",
										Value: "nginx.tsuru.io/app=nginx,nginx.tsuru.io/resource-name=my-instance",
									},
								},
								VolumeMounts: []corev1.VolumeMount{
									{
										Name:      rotateTLSSessionTicketsVolumeName,
										MountPath: rotateTLSSessionTicketsScriptPath,
										SubPath:   rotateTLSSessionTicketsScriptFilename,
									},
								},
							},
						},
						Volumes: []corev1.Volume{
							{
								Name: rotateTLSSessionTicketsVolumeName,
								VolumeSource: corev1.VolumeSource{
									DownwardAPI: &corev1.DownwardAPIVolumeSource{
										Items: []corev1.DownwardAPIVolumeFile{
											{
												Path: rotateTLSSessionTicketsScriptFilename,
												FieldRef: &corev1.ObjectFieldSelector{
													FieldPath: fmt.Sprintf("metadata.annotations['%s']", rotateTLSSessionTicketsScriptFilename),
												},
											},
										},
									},
								},
							},
						},
					},
				}, gotCronJob.Spec.JobTemplate.Spec.Template)
			},
		},
		{
			name: "Session Ticket: update key length and rotatation interval",
			objects: []runtime.Object{
				&corev1.Secret{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "my-instance-session-tickets",
						Namespace: "default",
					},
				},
				&batchv1beta1.CronJob{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "my-instance-session-tickets",
						Namespace: "default",
					},
				},
			},
			instance: &v1alpha1.RpaasInstance{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "my-instance",
					Namespace: "default",
				},
				Spec: v1alpha1.RpaasInstanceSpec{
					TLSSessionResumption: &v1alpha1.TLSSessionResumption{
						SessionTicket: &v1alpha1.TLSSessionTicket{
							KeepLastKeys:        uint32(3),
							KeyRotationInterval: uint32(60 * 24), // a day
							KeyLength:           v1alpha1.SessionTicketKeyLength80,
							Image:               "my.custom.image:tag",
						},
					},
				},
			},
			assert: func(t *testing.T, err error, gotSecret *corev1.Secret, gotCronJob *batchv1beta1.CronJob) {
				require.NoError(t, err)
				require.NotNil(t, gotSecret)
				require.NotNil(t, gotCronJob)

				expectedKeyLength := 80
				assert.Len(t, gotSecret.Data, 4)
				for i := 0; i < 4; i++ {
					assert.Len(t, gotSecret.Data[fmt.Sprintf("ticket.%d.key", i)], expectedKeyLength)
				}

				assert.Equal(t, "*/1440 * * * *", gotCronJob.Spec.Schedule)
				assert.Equal(t, "my.custom.image:tag", gotCronJob.Spec.JobTemplate.Spec.Template.Spec.Containers[0].Image)
				assert.Contains(t, gotCronJob.Spec.JobTemplate.Spec.Template.Spec.Containers[0].Env, corev1.EnvVar{Name: "SESSION_TICKET_KEY_LENGTH", Value: "80"})
				assert.Contains(t, gotCronJob.Spec.JobTemplate.Spec.Template.Spec.Containers[0].Env, corev1.EnvVar{Name: "SESSION_TICKET_KEYS", Value: "4"})
				assert.Contains(t, gotCronJob.Spec.JobTemplate.Spec.Template.Spec.Containers[0].Env, corev1.EnvVar{Name: "NGINX_LABEL_SELECTOR", Value: "nginx.tsuru.io/app=nginx,nginx.tsuru.io/resource-name=my-instance"})
			},
		},
		{
			name: "when session ticket is disabled, should remove Secret and CronJob objects",
			objects: []runtime.Object{
				&corev1.Secret{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "my-instance-session-tickets",
						Namespace: "default",
					},
				},
				&batchv1beta1.CronJob{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "my-instance-session-tickets",
						Namespace: "default",
					},
				},
			},
			instance: &v1alpha1.RpaasInstance{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "my-instance",
					Namespace: "default",
				},
				Spec: v1alpha1.RpaasInstanceSpec{
					TLSSessionResumption: &v1alpha1.TLSSessionResumption{},
				},
			},
			assert: func(t *testing.T, err error, gotSecret *corev1.Secret, gotCronJob *batchv1beta1.CronJob) {
				require.NoError(t, err)
				assert.Empty(t, gotSecret.Name)
				assert.Empty(t, gotCronJob.Name)
			},
		},
		{
			name: "when decreasing the number of keys",
			instance: &v1alpha1.RpaasInstance{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "my-instance",
					Namespace: "default",
				},
				Spec: v1alpha1.RpaasInstanceSpec{
					TLSSessionResumption: &v1alpha1.TLSSessionResumption{
						SessionTicket: &v1alpha1.TLSSessionTicket{
							KeepLastKeys: uint32(1),
						},
					},
				},
			},
			objects: []runtime.Object{
				&corev1.Secret{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "my-instance-session-tickets",
						Namespace: "default",
					},
					Data: map[string][]byte{
						"ticket.0.key": {'h', 'e', 'l', 'l', 'o'},
						"ticket.1.key": {'w', 'o', 'r', 'd', '!'},
						"ticket.2.key": {'f', 'o', 'o'},
						"ticket.3.key": {'b', 'a', 'r'},
					},
				},
			},
			assert: func(t *testing.T, err error, gotSecret *corev1.Secret, gotCronJob *batchv1beta1.CronJob) {
				require.NoError(t, err)

				expectedKeys := 2
				assert.Len(t, gotSecret.Data, expectedKeys)
				assert.Equal(t, gotSecret.Data["ticket.0.key"], []byte{'h', 'e', 'l', 'l', 'o'})
				assert.Equal(t, gotSecret.Data["ticket.1.key"], []byte{'w', 'o', 'r', 'd', '!'})
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resources := []runtime.Object{tt.instance}
			if tt.objects != nil {
				resources = append(resources, tt.objects...)
			}

			r := newRpaasInstanceReconciler(resources...)

			err := r.reconcileTLSSessionResumption(context.TODO(), tt.instance)
			if tt.assert == nil {
				require.NoError(t, err)
				return
			}

			var secret corev1.Secret
			secretName := types.NamespacedName{
				Name:      tt.instance.Name + sessionTicketsSecretSuffix,
				Namespace: tt.instance.Namespace,
			}
			r.Client.Get(context.TODO(), secretName, &secret)

			var cronJob batchv1beta1.CronJob
			cronJobName := types.NamespacedName{
				Name:      tt.instance.Name + sessionTicketsCronJobSuffix,
				Namespace: tt.instance.Namespace,
			}
			r.Client.Get(context.TODO(), cronJobName, &cronJob)

			tt.assert(t, err, &secret, &cronJob)
		})
	}
}

func Test_nameForCronJob(t *testing.T) {
	tests := []struct {
		cronJobName string
		expected    string
	}{
		{
			cronJobName: "my-instance-some-suffix",
			expected:    "my-instance-some-suffix",
		},
		{
			cronJobName: "some-great-great-great-instance-name-just-another-longer-enough-suffix-too",
			expected:    "some-great-great-great-instance-name-just-a6df1c7316",
		},
	}

	for _, tt := range tests {
		t.Run("", func(t *testing.T) {
			got := nameForCronJob(tt.cronJobName)
			assert.Equal(t, tt.expected, got)
		})
	}
}

func newRpaasInstanceReconciler(objs ...runtime.Object) *RpaasInstanceReconciler {
	scheme := extensionsruntime.NewScheme()
	return &RpaasInstanceReconciler{
		Client:              fake.NewFakeClientWithScheme(scheme, objs...),
		Log:                 ctrl.Log,
		Scheme:              scheme,
		RolloutNginxEnabled: true,
	}
}
