// Copyright 2019 tsuru authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package rpaasinstance

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	nginxv1alpha1 "github.com/tsuru/nginx-operator/pkg/apis/nginx/v1alpha1"
	"github.com/tsuru/rpaas-operator/pkg/apis/extensions/v1alpha1"
	autoscalingv2beta2 "k8s.io/api/autoscaling/v2beta2"
	corev1 "k8s.io/api/core/v1"
	k8sErrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func Test_mergePlans(t *testing.T) {
	tests := []struct {
		base     v1alpha1.RpaasPlanSpec
		override v1alpha1.RpaasPlanSpec
		expected v1alpha1.RpaasPlanSpec
	}{
		{
			base:     v1alpha1.RpaasPlanSpec{},
			override: v1alpha1.RpaasPlanSpec{},
			expected: v1alpha1.RpaasPlanSpec{},
		},
		{
			base:     v1alpha1.RpaasPlanSpec{Image: "img0", Description: "a", Config: v1alpha1.NginxConfig{User: "root", CacheEnabled: v1alpha1.Bool(true)}},
			override: v1alpha1.RpaasPlanSpec{Image: "img1"},
			expected: v1alpha1.RpaasPlanSpec{Image: "img1", Description: "a", Config: v1alpha1.NginxConfig{User: "root", CacheEnabled: v1alpha1.Bool(true)}},
		},
		{
			base:     v1alpha1.RpaasPlanSpec{Image: "img0", Description: "a", Config: v1alpha1.NginxConfig{User: "root", CacheSize: "10", CacheEnabled: v1alpha1.Bool(true)}},
			override: v1alpha1.RpaasPlanSpec{Image: "img1", Config: v1alpha1.NginxConfig{User: "ubuntu"}},
			expected: v1alpha1.RpaasPlanSpec{Image: "img1", Description: "a", Config: v1alpha1.NginxConfig{User: "ubuntu", CacheSize: "10", CacheEnabled: v1alpha1.Bool(true)}},
		},
		{
			base:     v1alpha1.RpaasPlanSpec{Image: "img0", Description: "a", Config: v1alpha1.NginxConfig{User: "root", CacheSize: "10", CacheEnabled: v1alpha1.Bool(true)}},
			override: v1alpha1.RpaasPlanSpec{Image: "img1", Config: v1alpha1.NginxConfig{User: "ubuntu", CacheEnabled: v1alpha1.Bool(false)}},
			expected: v1alpha1.RpaasPlanSpec{Image: "img1", Description: "a", Config: v1alpha1.NginxConfig{User: "ubuntu", CacheSize: "10", CacheEnabled: v1alpha1.Bool(false)}},
		},

		{
			base:     v1alpha1.RpaasPlanSpec{Image: "img0", Resources: corev1.ResourceRequirements{Limits: corev1.ResourceList{corev1.ResourceCPU: resource.MustParse("100m"), corev1.ResourceMemory: resource.MustParse("100Mi")}}},
			override: v1alpha1.RpaasPlanSpec{Image: "img1", Resources: corev1.ResourceRequirements{Limits: corev1.ResourceList{corev1.ResourceMemory: resource.MustParse("200Mi")}}},
			expected: v1alpha1.RpaasPlanSpec{Image: "img1", Resources: corev1.ResourceRequirements{Limits: corev1.ResourceList{corev1.ResourceCPU: resource.MustParse("100m"), corev1.ResourceMemory: resource.MustParse("200Mi")}}},
		},
	}

	for _, tt := range tests {
		t.Run("", func(t *testing.T) {
			result, err := mergePlans(tt.base, tt.override)
			require.NoError(t, err)
			assert.Equal(t, result, tt.expected)
		})
	}
}

func TestReconcileRpaasInstance_getRpaasInstance(t *testing.T) {
	instance1 := newEmptyRpaasInstance()
	instance1.Name = "instance1"

	instance2 := newEmptyRpaasInstance()
	instance2.Name = "instance2"
	instance2.Spec.Flavors = []string{"mint"}
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

	resources := []runtime.Object{instance1, instance2, instance3, mintFlavor, mangoFlavor}

	tests := []struct {
		name      string
		objectKey types.NamespacedName
		expected  v1alpha1.RpaasInstance
	}{
		{
			name:      "when the fetched RpaasInstance has no flavor provided",
			objectKey: types.NamespacedName{Name: instance1.Name, Namespace: instance1.Namespace},
			expected:  *instance1,
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
					Flavors: []string{"mint"},
					Service: &nginxv1alpha1.NginxService{
						Annotations: map[string]string{
							"some-instance-annotation-key": "blah",
							"flavored-service-annotation":  "v1",
						},
						Labels: map[string]string{
							"some-instance-label-key": "label1",
							"conflict-label":          "instance value",
							"flavored-service-label":  "v1",
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
					Flavors: []string{"mint", "mango"},
					Service: &nginxv1alpha1.NginxService{
						Annotations: map[string]string{
							"some-instance-annotation-key": "blah",
							"flavored-service-annotation":  "v1",
							"mango-service-annotation":     "mango",
						},
						Labels: map[string]string{
							"some-instance-label-key": "label1",
							"conflict-label":          "instance value",
							"flavored-service-label":  "v1",
							"mango-service-label":     "mango",
						},
					},
					PodTemplate: nginxv1alpha1.NginxPodTemplateSpec{
						Annotations: map[string]string{
							"flavored-pod-annotation": "v1",
							"mango-pod-annotation":    "mango",
						},
						Labels: map[string]string{
							"flavored-pod-label": "v1",
							"mango-pod-label":    "mango",
						},
						HostNetwork: true,
					},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			k8sClient := fake.NewFakeClientWithScheme(newScheme(), resources...)
			reconciler := &ReconcileRpaasInstance{
				client: k8sClient,
				scheme: newScheme(),
			}
			instance, err := reconciler.getRpaasInstance(context.TODO(), tt.objectKey)
			require.NoError(t, err)
			assert.Equal(t, *instance, tt.expected)
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

	nginx1 := &nginxv1alpha1.Nginx{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "nginx.tsuru.io/v1alpha1",
			Kind:       "Nginx",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      instance1.Name,
			Namespace: instance1.Namespace,
		},
	}

	instance2 := newEmptyRpaasInstance()
	instance2.Name = "instance-2"

	nginx2 := nginx1.DeepCopy()
	nginx2.Name = "instance-2"

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
				APIVersion: "nginx.tsuru.io/v1alpha1",
				Kind:       "Nginx",
				Name:       nginx2.Name,
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

	resources := []runtime.Object{instance1, instance2, nginx1, nginx2, hpa2}

	tests := []struct {
		name      string
		instance  v1alpha1.RpaasInstance
		nginx     nginxv1alpha1.Nginx
		assertion func(t *testing.T, err error, got *autoscalingv2beta2.HorizontalPodAutoscaler)
	}{
		{
			name:     "when there is HPA resource but autoscale spec is nil",
			instance: *instance2,
			nginx:    *nginx2,
			assertion: func(t *testing.T, err error, got *autoscalingv2beta2.HorizontalPodAutoscaler) {
				require.Error(t, err)
				assert.True(t, k8sErrors.IsNotFound(err))
			},
		},
		{
			name:     "when there is no HPA resource but autoscale spec is provided",
			instance: *instance1,
			nginx:    *nginx1,
			assertion: func(t *testing.T, err error, got *autoscalingv2beta2.HorizontalPodAutoscaler) {
				require.NoError(t, err)
				require.NotNil(t, got)
				assert.Equal(t, int32(25), got.Spec.MaxReplicas)
				assert.Equal(t, int32Ptr(4), got.Spec.MinReplicas)
				assert.Equal(t, autoscalingv2beta2.CrossVersionObjectReference{
					APIVersion: "nginx.tsuru.io/v1alpha1",
					Kind:       "Nginx",
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
			},
		},
		{
			name: "when there is HPA resource but differs from autoscale spec",
			instance: v1alpha1.RpaasInstance{
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
			nginx: *nginx2,
			assertion: func(t *testing.T, err error, got *autoscalingv2beta2.HorizontalPodAutoscaler) {
				require.NoError(t, err)
				require.NotNil(t, got)
				assert.Equal(t, int32(200), got.Spec.MaxReplicas)
				assert.Equal(t, int32Ptr(2), got.Spec.MinReplicas)
				assert.Equal(t, autoscalingv2beta2.CrossVersionObjectReference{
					APIVersion: "nginx.tsuru.io/v1alpha1",
					Kind:       "Nginx",
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
			k8sClient := fake.NewFakeClientWithScheme(newScheme(), resources...)
			reconciler := &ReconcileRpaasInstance{
				client: k8sClient,
				scheme: newScheme(),
			}

			err := reconciler.reconcileHPA(context.TODO(), tt.instance, tt.nginx)
			require.NoError(t, err)

			hpa := new(autoscalingv2beta2.HorizontalPodAutoscaler)
			if err == nil {
				err = k8sClient.Get(context.TODO(), types.NamespacedName{Name: tt.instance.Name, Namespace: tt.instance.Namespace}, hpa)
			}

			tt.assertion(t, err, hpa)
		})
	}
}

func int32Ptr(n int32) *int32 {
	return &n
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
		Spec: v1alpha1.RpaasInstanceSpec{},
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

func newScheme() *runtime.Scheme {
	scheme := runtime.NewScheme()
	autoscalingv2beta2.SchemeBuilder.AddToScheme(scheme)
	v1alpha1.SchemeBuilder.AddToScheme(scheme)
	nginxv1alpha1.SchemeBuilder.AddToScheme(scheme)
	return scheme
}
