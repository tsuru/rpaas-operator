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
	"github.com/tsuru/rpaas-operator/config"
	"github.com/tsuru/rpaas-operator/pkg/apis"
	"github.com/tsuru/rpaas-operator/pkg/apis/extensions/v1alpha1"
	autoscalingv2beta2 "k8s.io/api/autoscaling/v2beta2"
	corev1 "k8s.io/api/core/v1"
	k8sErrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
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
					CacheSize:    "10",
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
					CacheSize:    "10",
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
					CacheSize:    "10",
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
					CacheSize:    "10",
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

	defaultFlavor := newRpaasFlavor()
	defaultFlavor.Name = "default"
	defaultFlavor.Spec.Default = true
	defaultFlavor.Spec.InstanceTemplate = &v1alpha1.RpaasInstanceSpec{
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

	resources := []runtime.Object{instance1, instance2, instance3, instance4, mintFlavor, mangoFlavor, defaultFlavor}

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
					Flavors: []string{"mint"},
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
							"default-service-label":   "default",
							"some-instance-label-key": "label1",
							"conflict-label":          "instance value",
							"flavored-service-label":  "default",
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
					Flavors: []string{"mint", "mango"},
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
							"flavored-service-label":  "default",
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
							"mango-pod-label":    "not-a-mango",
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

func Test_reconcileHeaterVolume(t *testing.T) {
	instance1 := newEmptyRpaasInstance()
	instance1.Name = "instance-1"

	resources := []runtime.Object{}
	scheme := newScheme()
	corev1.AddToScheme(scheme)

	k8sClient := fake.NewFakeClientWithScheme(scheme, resources...)
	reconciler := &ReconcileRpaasInstance{
		client: k8sClient,
		scheme: newScheme(),
	}

	plan := &v1alpha1.RpaasPlan{
		Spec: v1alpha1.RpaasPlanSpec{
			Config: v1alpha1.NginxConfig{
				CacheHeaterStorage: &v1alpha1.CacheHeaterStorage{
					StorageClassName: strPtr("my-storage-class"),
				},
			},
		},
	}

	err := reconciler.reconcileCacheHeaterVolume(instance1, plan)
	require.NoError(t, err)

	pvc := &corev1.PersistentVolumeClaim{}
	err = k8sClient.Get(context.TODO(), types.NamespacedName{Name: instance1.Name + "-heater-volume", Namespace: instance1.Namespace}, pvc)
	require.NoError(t, err)

	assert.Equal(t, pvc.ObjectMeta.OwnerReferences[0].Kind, "RpaasInstance")
	assert.Equal(t, pvc.ObjectMeta.OwnerReferences[0].Name, instance1.Name)
	assert.Equal(t, pvc.Spec.StorageClassName, strPtr("my-storage-class"))
	assert.Equal(t, pvc.Spec.AccessModes, []corev1.PersistentVolumeAccessMode{corev1.ReadWriteMany})
}

func Test_reconcileHeaterVolumeWithLabels(t *testing.T) {
	instance1 := newEmptyRpaasInstance()
	instance1.Name = "instance-1"

	resources := []runtime.Object{}
	scheme := newScheme()
	corev1.AddToScheme(scheme)

	k8sClient := fake.NewFakeClientWithScheme(scheme, resources...)
	reconciler := &ReconcileRpaasInstance{
		client: k8sClient,
		scheme: newScheme(),
	}

	plan := &v1alpha1.RpaasPlan{
		Spec: v1alpha1.RpaasPlanSpec{
			Config: v1alpha1.NginxConfig{
				CacheHeaterStorage: &v1alpha1.CacheHeaterStorage{
					StorageClassName: strPtr("my-storage-class"),
					VolumeLabels: map[string]string{
						"some-label":  "foo",
						"other-label": "bar",
					},
				},
			},
		},
	}

	err := reconciler.reconcileCacheHeaterVolume(instance1, plan)
	require.NoError(t, err)

	pvc := &corev1.PersistentVolumeClaim{}
	err = k8sClient.Get(context.TODO(), types.NamespacedName{Name: instance1.Name + "-heater-volume", Namespace: instance1.Namespace}, pvc)
	require.NoError(t, err)

	assert.Equal(t, 2, len(pvc.ObjectMeta.Labels))
	assert.Equal(t, "foo", pvc.ObjectMeta.Labels["some-label"])
	assert.Equal(t, "bar", pvc.ObjectMeta.Labels["other-label"])
}

func Test_reconcileHeaterVolumeWithInstanceTeamOwner(t *testing.T) {
	instance1 := newEmptyRpaasInstance()
	instance1.Name = "instance-1"
  instance1.SetTeamOwner("team-one")

	resources := []runtime.Object{}
	scheme := newScheme()
	corev1.AddToScheme(scheme)

	k8sClient := fake.NewFakeClientWithScheme(scheme, resources...)
	reconciler := &ReconcileRpaasInstance{
		client: k8sClient,
		scheme: newScheme(),
	}

	plan := &v1alpha1.RpaasPlan{
		Spec: v1alpha1.RpaasPlanSpec{
			Config: v1alpha1.NginxConfig{
				CacheHeaterStorage: &v1alpha1.CacheHeaterStorage{
					StorageClassName: strPtr("my-storage-class"),
					VolumeLabels: map[string]string{
						"some-label":  "foo",
						"other-label": "bar",
						volumeTeamLabel: "another-team",
					},
				},
			},
		},
	}

	err := reconciler.reconcileCacheHeaterVolume(instance1, plan)
	require.NoError(t, err)

	pvc := &corev1.PersistentVolumeClaim{}
	err = k8sClient.Get(context.TODO(), types.NamespacedName{Name: instance1.Name + "-heater-volume", Namespace: instance1.Namespace}, pvc)
	require.NoError(t, err)

	assert.Equal(t, 3, len(pvc.ObjectMeta.Labels))
	assert.Equal(t, "foo", pvc.ObjectMeta.Labels["some-label"])
	assert.Equal(t, "bar", pvc.ObjectMeta.Labels["other-label"])
	assert.Equal(t, "team-one", pvc.ObjectMeta.Labels[volumeTeamLabel])
}

func Test_reconcileHeaterVolumeUsingCacheSize(t *testing.T) {
	instance1 := newEmptyRpaasInstance()
	instance1.Name = "instance-1"

	resources := []runtime.Object{}
	scheme := newScheme()
	corev1.AddToScheme(scheme)

	k8sClient := fake.NewFakeClientWithScheme(scheme, resources...)
	reconciler := &ReconcileRpaasInstance{
		client: k8sClient,
		scheme: newScheme(),
	}

	plan := &v1alpha1.RpaasPlan{
		Spec: v1alpha1.RpaasPlanSpec{
			Config: v1alpha1.NginxConfig{
				CacheSize: "10Gi",
				CacheHeaterStorage: &v1alpha1.CacheHeaterStorage{
					StorageClassName: strPtr("my-storage-class"),
				},
			},
		},
	}

	err := reconciler.reconcileCacheHeaterVolume(instance1, plan)
	require.NoError(t, err)

	pvc := &corev1.PersistentVolumeClaim{}
	err = k8sClient.Get(context.TODO(), types.NamespacedName{Name: instance1.Name + "-heater-volume", Namespace: instance1.Namespace}, pvc)
	require.NoError(t, err)

	parsedSize, err := resource.ParseQuantity("10Gi")
	require.NoError(t, err)
	assert.Equal(t, parsedSize, pvc.Spec.Resources.Requests["storage"])
}

func Test_reconcileHeaterVolumeUsingStorageSize(t *testing.T) {
	instance1 := newEmptyRpaasInstance()
	instance1.Name = "instance-1"

	resources := []runtime.Object{}
	scheme := newScheme()
	corev1.AddToScheme(scheme)

	k8sClient := fake.NewFakeClientWithScheme(scheme, resources...)
	reconciler := &ReconcileRpaasInstance{
		client: k8sClient,
		scheme: newScheme(),
	}

	plan := &v1alpha1.RpaasPlan{
		Spec: v1alpha1.RpaasPlanSpec{
			Config: v1alpha1.NginxConfig{
				CacheSize: "10Gi",
				CacheHeaterStorage: &v1alpha1.CacheHeaterStorage{
					StorageClassName: strPtr("my-storage-class"),
					StorageSize:      "100Gi",
				},
			},
		},
	}

	err := reconciler.reconcileCacheHeaterVolume(instance1, plan)
	require.NoError(t, err)

	pvc := &corev1.PersistentVolumeClaim{}
	err = k8sClient.Get(context.TODO(), types.NamespacedName{Name: instance1.Name + "-heater-volume", Namespace: instance1.Namespace}, pvc)
	require.NoError(t, err)

	parsedSize, err := resource.ParseQuantity("100Gi")
	require.NoError(t, err)
	assert.Equal(t, parsedSize, pvc.Spec.Resources.Requests["storage"])
}

func Test_destroyHeaterVolume(t *testing.T) {
	instance1 := newEmptyRpaasInstance()
	instance1.Name = "instance-1"

	pvc := &corev1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "instance-1-heater-volume",
			Namespace: "default",
		},
	}
	storageConfig := &v1alpha1.CacheHeaterStorage{
		StorageClassName: strPtr("my-storage-class"),
	}
	resources := []runtime.Object{pvc}
	scheme := newScheme()
	corev1.AddToScheme(scheme)

	k8sClient := fake.NewFakeClientWithScheme(scheme, resources...)
	reconciler := &ReconcileRpaasInstance{
		client: k8sClient,
		scheme: newScheme(),
	}

	err := reconciler.destroyCacheHeaterVolume(instance1, storageConfig)
	require.NoError(t, err)

	pvc = &corev1.PersistentVolumeClaim{}
	err = k8sClient.Get(context.TODO(), types.NamespacedName{Name: instance1.Name + "-heater-volume", Namespace: instance1.Namespace}, pvc)
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

func TestReconcileNginx_reconcilePorts(t *testing.T) {
	scheme := runtime.NewScheme()
	corev1.AddToScheme(scheme)
	apis.AddToScheme(scheme)

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
					AllocateContainerPorts: true,
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
					AllocateContainerPorts: true,
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
					AllocateContainerPorts: true,
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
					AllocateContainerPorts: true,
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
						AllocateContainerPorts: true,
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
					AllocateContainerPorts: true,
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
						AllocateContainerPorts: true,
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
			reconciler := &ReconcileRpaasInstance{
				client: fake.NewFakeClientWithScheme(scheme, resources...),
				scheme: scheme,
			}
			ports, err := reconciler.reconcilePorts(context.Background(), tt.rpaas, 2)
			var allocation v1alpha1.RpaasPortAllocation
			allocErr := reconciler.client.Get(context.Background(), types.NamespacedName{
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
			Config: v1alpha1.NginxConfig{
				CacheHeaterEnabled: true,
				CacheHeaterStorage: &v1alpha1.CacheHeaterStorage{
					StorageClassName: strPtr("my-storage-class"),
				},
			},
		},
	}
	resources := []runtime.Object{rpaas, plan}
	scheme := newScheme()
	corev1.AddToScheme(scheme)
	client := fake.NewFakeClientWithScheme(scheme, resources...)
	reconciler := &ReconcileRpaasInstance{
		client: client,
		scheme: scheme,
	}
	result, err := reconciler.Reconcile(reconcile.Request{NamespacedName: types.NamespacedName{Namespace: "default", Name: "my-instance"}})
	require.NoError(t, err)

	assert.Equal(t, result, reconcile.Result{})

	nginx := &nginxv1alpha1.Nginx{}
	err = client.Get(context.TODO(), types.NamespacedName{Name: rpaas.Name, Namespace: rpaas.Namespace}, nginx)
	require.NoError(t, err)

	assert.Equal(t, nginx.Spec.PodTemplate.Volumes[0].Name, "cache-heater-volume")
	assert.Equal(t, nginx.Spec.PodTemplate.Volumes[0].PersistentVolumeClaim, &corev1.PersistentVolumeClaimVolumeSource{ClaimName: "my-instance-heater-volume"})
	assert.Equal(t, nginx.Spec.PodTemplate.VolumeMounts[0].Name, "cache-heater-volume")
	assert.Equal(t, nginx.Spec.PodTemplate.VolumeMounts[0].MountPath, "/var/cache/cache-heater")
}
