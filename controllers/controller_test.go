// Copyright 2019 tsuru authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package controllers

import (
	"context"
	"fmt"
	"testing"

	kedav1alpha1 "github.com/kedacore/keda/v2/apis/keda/v1alpha1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	nginxv1alpha1 "github.com/tsuru/nginx-operator/api/v1alpha1"
	autoscalingv2 "k8s.io/api/autoscaling/v2"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	policyv1 "k8s.io/api/policy/v1"
	k8sErrors "k8s.io/apimachinery/pkg/api/errors"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"github.com/tsuru/rpaas-operator/api/v1alpha1"
	extensionsruntime "github.com/tsuru/rpaas-operator/pkg/runtime"
)

func Test_newNginx(t *testing.T) {
	tests := map[string]struct {
		instance func(i *v1alpha1.RpaasInstance) *v1alpha1.RpaasInstance
		plan     func(p *v1alpha1.RpaasPlan) *v1alpha1.RpaasPlan
		cm       func(c *corev1.ConfigMap) *corev1.ConfigMap
		expected func(n *nginxv1alpha1.Nginx) *nginxv1alpha1.Nginx
	}{
		"w/ extra files": {
			instance: func(i *v1alpha1.RpaasInstance) *v1alpha1.RpaasInstance {
				i.Spec.Files = []v1alpha1.File{
					{
						Name: "waf.cfg",
						ConfigMap: &corev1.ConfigMapKeySelector{
							LocalObjectReference: corev1.LocalObjectReference{Name: "my-instance-extra-files-1"},
						},
					},
					{
						Name: "binary.exe",
						ConfigMap: &corev1.ConfigMapKeySelector{
							LocalObjectReference: corev1.LocalObjectReference{Name: "my-instance-extra-files-2"},
						},
					},
				}
				return i
			},
			expected: func(n *nginxv1alpha1.Nginx) *nginxv1alpha1.Nginx {
				n.Spec.PodTemplate.Volumes = []corev1.Volume{
					{
						Name: "extra-files-0",
						VolumeSource: corev1.VolumeSource{
							ConfigMap: &corev1.ConfigMapVolumeSource{
								LocalObjectReference: corev1.LocalObjectReference{Name: "my-instance-extra-files-1"},
							},
						},
					},
					{
						Name: "extra-files-1",
						VolumeSource: corev1.VolumeSource{
							ConfigMap: &corev1.ConfigMapVolumeSource{
								LocalObjectReference: corev1.LocalObjectReference{Name: "my-instance-extra-files-2"},
							},
						},
					},
				}
				n.Spec.PodTemplate.VolumeMounts = []corev1.VolumeMount{
					{
						Name:      "extra-files-0",
						MountPath: "/etc/nginx/extra_files/waf.cfg",
						SubPath:   "waf.cfg",
						ReadOnly:  true,
					},
					{
						Name:      "extra-files-1",
						MountPath: "/etc/nginx/extra_files/binary.exe",
						SubPath:   "binary.exe",
						ReadOnly:  true,
					},
				}
				return n
			},
		},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			instance := &v1alpha1.RpaasInstance{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "my-instance",
					Namespace: "rpaasv2",
				},
				Spec: v1alpha1.RpaasInstanceSpec{
					PlanName: "my-plan",
				},
			}
			if tt.instance != nil {
				instance = tt.instance(instance)
			}

			plan := &v1alpha1.RpaasPlan{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "my-plan",
					Namespace: "rpaasv2",
				},
			}
			if tt.plan != nil {
				plan = tt.plan(plan)
			}

			cm := &corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "my-instance-nginx-conf",
					Namespace: "rpaasv2",
				},
			}

			nginx := &nginxv1alpha1.Nginx{
				TypeMeta: metav1.TypeMeta{
					APIVersion: "nginx.tsuru.io/v1alpha1",
					Kind:       "Nginx",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:      "my-instance",
					Namespace: "rpaasv2",
					Labels: map[string]string{
						"rpaas.extensions.tsuru.io/instance-name": "my-instance",
						"rpaas.extensions.tsuru.io/plan-name":     "my-plan",
					},
					OwnerReferences: []metav1.OwnerReference{{
						APIVersion:         "extensions.tsuru.io/v1alpha1",
						Kind:               "RpaasInstance",
						Name:               "my-instance",
						Controller:         func(b bool) *bool { return &b }(true),
						BlockOwnerDeletion: func(b bool) *bool { return &b }(true),
					}},
				},
				Spec: nginxv1alpha1.NginxSpec{
					Config: &nginxv1alpha1.ConfigRef{
						Kind: nginxv1alpha1.ConfigKindConfigMap,
						Name: "my-instance-nginx-conf",
					},
					HealthcheckPath: "/_nginx_healthcheck",
				},
			}
			if tt.expected != nil {
				nginx = tt.expected(nginx)
			}

			got := newNginx(instance, plan, cm)
			assert.Equal(t, nginx, got)
		})
	}
}

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
		{
			base: v1alpha1.RpaasPlanSpec{
				Config: v1alpha1.NginxConfig{
					CacheEnabled:     v1alpha1.Bool(true),
					CachePath:        "/var/cache/nginx/rpaas",
					CacheSize:        func(r resource.Quantity) *resource.Quantity { return &r }(resource.MustParse("8Gi")),
					CacheZoneSize:    func(r resource.Quantity) *resource.Quantity { return &r }(resource.MustParse("100Mi")),
					CacheInactive:    "12h",
					CacheLoaderFiles: 100,
				},
			},
			override: v1alpha1.RpaasPlanSpec{
				Config: v1alpha1.NginxConfig{
					CacheSize:        func(r resource.Quantity) *resource.Quantity { return &r }(resource.MustParse("14Gi")),
					CacheZoneSize:    func(r resource.Quantity) *resource.Quantity { return &r }(resource.MustParse("500Mi")),
					CacheInactive:    "7d",
					CacheLoaderFiles: 100000,
				},
			},
			expected: v1alpha1.RpaasPlanSpec{
				Config: v1alpha1.NginxConfig{
					CacheEnabled:     v1alpha1.Bool(true),
					CachePath:        "/var/cache/nginx/rpaas",
					CacheSize:        func(r resource.Quantity) *resource.Quantity { return &r }(resource.MustParse("14Gi")),
					CacheZoneSize:    func(r resource.Quantity) *resource.Quantity { return &r }(resource.MustParse("500Mi")),
					CacheInactive:    "7d",
					CacheLoaderFiles: 100000,
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

func TestReconcileRpaasInstance_getRpaasInstance(t *testing.T) {
	t.Parallel()

	tests := map[string]struct {
		resources []runtime.Object
		instance  func(i *v1alpha1.RpaasInstance) *v1alpha1.RpaasInstance
		expected  func(i *v1alpha1.RpaasInstance) *v1alpha1.RpaasInstance
	}{
		"instance with neither custom flavor nor default ones": {
			instance: func(i *v1alpha1.RpaasInstance) *v1alpha1.RpaasInstance {
				return i
			},
			expected: func(i *v1alpha1.RpaasInstance) *v1alpha1.RpaasInstance {
				return i
			},
		},

		"instance without custom flavor, but with default one": {
			resources: []runtime.Object{
				&v1alpha1.RpaasFlavor{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "default",
						Namespace: "default",
					},
					Spec: v1alpha1.RpaasFlavorSpec{
						Default: true,
						InstanceTemplate: &v1alpha1.RpaasInstanceSpec{
							Service: &nginxv1alpha1.NginxService{
								Annotations: map[string]string{
									"rpaas.extensions.tsuru.io/is-default-service-annotation": "true",
								},
								Labels: map[string]string{
									"rpaas.extensions.tsuru.io/is-default-service-label": "true",
								},
							},
							PodTemplate: nginxv1alpha1.NginxPodTemplateSpec{
								Annotations: map[string]string{
									"rpaas.extensions.tsuru.io/is-default-pod-annotation": "true",
								},
								Labels: map[string]string{
									"rpaas.extensions.tsuru.io/is-default-pod-label": "true",
								},
							},
						},
					},
				},
			},
			instance: func(i *v1alpha1.RpaasInstance) *v1alpha1.RpaasInstance {
				i.Spec.Service = &nginxv1alpha1.NginxService{
					Annotations: map[string]string{
						"rpaas.extensions.tsuru.io/is-instance-service-annotation": "true",
					},
					Labels: map[string]string{
						"rpaas.extensions.tsuru.io/is-instance-service-label": "true",
					},
				}
				i.Spec.PodTemplate = nginxv1alpha1.NginxPodTemplateSpec{
					Annotations: map[string]string{
						"rpaas.extensions.tsuru.io/is-instance-pod-annotation": "true",
					},
					Labels: map[string]string{
						"rpaas.extensions.tsuru.io/is-instance-pod-label": "true",
					},
				}
				return i
			},
			expected: func(i *v1alpha1.RpaasInstance) *v1alpha1.RpaasInstance {
				i.Spec.Service = &nginxv1alpha1.NginxService{
					Annotations: map[string]string{
						"rpaas.extensions.tsuru.io/is-default-service-annotation":  "true",
						"rpaas.extensions.tsuru.io/is-instance-service-annotation": "true",
					},
					Labels: map[string]string{
						"rpaas.extensions.tsuru.io/is-default-service-label":  "true",
						"rpaas.extensions.tsuru.io/is-instance-service-label": "true",
					},
				}
				i.Spec.PodTemplate = nginxv1alpha1.NginxPodTemplateSpec{
					Annotations: map[string]string{
						"rpaas.extensions.tsuru.io/is-default-pod-annotation":  "true",
						"rpaas.extensions.tsuru.io/is-instance-pod-annotation": "true",
					},
					Labels: map[string]string{
						"rpaas.extensions.tsuru.io/is-default-pod-label":  "true",
						"rpaas.extensions.tsuru.io/is-instance-pod-label": "true",
					},
				}
				return i
			},
		},

		"when DNS zone is defined on default flavor but custom flavor overrides it": {
			resources: []runtime.Object{
				&v1alpha1.RpaasFlavor{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "default",
						Namespace: "default",
					},
					Spec: v1alpha1.RpaasFlavorSpec{
						InstanceTemplate: &v1alpha1.RpaasInstanceSpec{
							DNS: &v1alpha1.DNSConfig{
								Zone: "apps.example.com",
								TTL:  func(n int32) *int32 { return &n }(300),
							},
						},
					},
				},
				&v1alpha1.RpaasFlavor{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "flavor-a",
						Namespace: "default",
					},
					Spec: v1alpha1.RpaasFlavorSpec{
						InstanceTemplate: &v1alpha1.RpaasInstanceSpec{
							DNS: &v1alpha1.DNSConfig{
								Zone: "apps.test",
								TTL:  func(n int32) *int32 { return &n }(30),
							},
						},
					},
				},
			},
			instance: func(i *v1alpha1.RpaasInstance) *v1alpha1.RpaasInstance {
				i.Spec.Flavors = []string{"flavor-a"}
				return i
			},
			expected: func(i *v1alpha1.RpaasInstance) *v1alpha1.RpaasInstance {
				i.Spec.DNS = &v1alpha1.DNSConfig{
					Zone: "apps.test",
					TTL:  func(n int32) *int32 { return &n }(30),
				}
				return i
			},
		},

		"using a custom flavor from another namespace": {
			resources: []runtime.Object{
				&v1alpha1.RpaasFlavor{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "flavor-a",
						Namespace: "rpaasv2-system",
					},
					Spec: v1alpha1.RpaasFlavorSpec{
						InstanceTemplate: &v1alpha1.RpaasInstanceSpec{
							EnablePodDisruptionBudget: func(b bool) *bool { return &b }(true),
						},
					},
				},
			},
			instance: func(i *v1alpha1.RpaasInstance) *v1alpha1.RpaasInstance {
				i.Spec.Flavors = []string{"flavor-a"}
				i.Spec.PlanNamespace = "rpaasv2-system"
				return i
			},
			expected: func(i *v1alpha1.RpaasInstance) *v1alpha1.RpaasInstance {
				i.Spec.EnablePodDisruptionBudget = func(b bool) *bool { return &b }(true)
				return i
			},
		},

		"when there's a flavor with custom values on service annotations": {
			resources: []runtime.Object{
				&v1alpha1.RpaasFlavor{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "flavor-a",
						Namespace: "default",
					},
					Spec: v1alpha1.RpaasFlavorSpec{
						InstanceTemplate: &v1alpha1.RpaasInstanceSpec{
							Service: &nginxv1alpha1.NginxService{
								Annotations: map[string]string{
									"rpaas.extensions.tsuru.io/custom-annotation": "Custom annotation value: {{ .Labels.rpaas_service }}/{{ .Labels.rpaas_instance }}/{{ .Name }}",
								},
							},
						},
					},
				},
			},
			instance: func(i *v1alpha1.RpaasInstance) *v1alpha1.RpaasInstance {
				i.Spec.Flavors = []string{"flavor-a"}
				return i
			},
			expected: func(i *v1alpha1.RpaasInstance) *v1alpha1.RpaasInstance {
				i.Spec.Service = &nginxv1alpha1.NginxService{
					Annotations: map[string]string{
						"rpaas.extensions.tsuru.io/custom-annotation": "Custom annotation value: rpaasv2/my-instance/my-instance",
					},
				}
				return i
			},
		},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			instance := tt.instance(&v1alpha1.RpaasInstance{
				TypeMeta: metav1.TypeMeta{
					APIVersion: "extensions.tsuru.io/v1alpha1",
					Kind:       "RpaasInstance",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:      "my-instance",
					Namespace: metav1.NamespaceDefault,
					Labels: map[string]string{
						"rpaas_service":  "rpaasv2",
						"rpaas_instance": "my-instance",
					},
				},
			})

			resources := append(tt.resources, instance.DeepCopy())

			reconciler := newRpaasInstanceReconciler(resources...)
			i, err := reconciler.getRpaasInstance(context.TODO(), types.NamespacedName{Name: instance.Name, Namespace: instance.Namespace})
			require.NoError(t, err)

			got, err := reconciler.mergeWithFlavors(context.TODO(), i)
			require.NoError(t, err)

			assert.Equal(t, tt.expected(i.DeepCopy()), got)
		})
	}
}

func Test_reconcileHPA(t *testing.T) {
	t.Parallel()

	baseExpectedHPA := &autoscalingv2.HorizontalPodAutoscaler{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "autoscaling/v2",
			Kind:       "HorizontalPodAutoscaler",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "my-instance",
			Namespace: metav1.NamespaceDefault,
			OwnerReferences: []metav1.OwnerReference{
				{
					APIVersion:         "extensions.tsuru.io/v1alpha1",
					Kind:               "RpaasInstance",
					Name:               "my-instance",
					Controller:         func(b bool) *bool { return &b }(true),
					BlockOwnerDeletion: func(b bool) *bool { return &b }(true),
				},
			},
			ResourceVersion: "1",
			Labels: map[string]string{
				"rpaas.extensions.tsuru.io/instance-name": "my-instance",
				"rpaas.extensions.tsuru.io/plan-name":     "my-plan",
			},
		},
	}

	baseExpectedScaledObject := &kedav1alpha1.ScaledObject{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "keda.sh/v1alpha1",
			Kind:       "ScaledObject",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "my-instance",
			Namespace: "default",
			OwnerReferences: []metav1.OwnerReference{
				{
					APIVersion:         "extensions.tsuru.io/v1alpha1",
					Kind:               "RpaasInstance",
					Name:               "my-instance",
					Controller:         func(b bool) *bool { return &b }(true),
					BlockOwnerDeletion: func(b bool) *bool { return &b }(true),
				},
			},
			ResourceVersion: "1",
			Labels: map[string]string{
				"rpaas.extensions.tsuru.io/instance-name": "my-instance",
				"rpaas.extensions.tsuru.io/plan-name":     "my-plan",
			},
		},
	}

	tests := map[string]struct {
		resources            []runtime.Object
		instance             func(*v1alpha1.RpaasInstance) *v1alpha1.RpaasInstance
		nginx                func(*nginxv1alpha1.Nginx) *nginxv1alpha1.Nginx
		expectedHPA          func(*autoscalingv2.HorizontalPodAutoscaler) *autoscalingv2.HorizontalPodAutoscaler
		expectedScaledObject func(*kedav1alpha1.ScaledObject) *kedav1alpha1.ScaledObject
		customAssert         func(t *testing.T, r *RpaasInstanceReconciler) bool

		expectedError func(t *testing.T)
	}{
		"(native HPA controller) setting autoscaling params first time": {
			instance: func(ri *v1alpha1.RpaasInstance) *v1alpha1.RpaasInstance {
				ri.Spec.Autoscale = &v1alpha1.RpaasInstanceAutoscaleSpec{
					MinReplicas:                    func(n int32) *int32 { return &n }(5),
					MaxReplicas:                    100,
					TargetCPUUtilizationPercentage: func(n int32) *int32 { return &n }(90),
				}
				return ri
			},
			expectedHPA: func(hpa *autoscalingv2.HorizontalPodAutoscaler) *autoscalingv2.HorizontalPodAutoscaler {
				hpa.Spec = autoscalingv2.HorizontalPodAutoscalerSpec{
					ScaleTargetRef: autoscalingv2.CrossVersionObjectReference{
						APIVersion: "apps/v1",
						Kind:       "Deployment",
						Name:       "my-instance",
					},
					MinReplicas: func(n int32) *int32 { return &n }(5),
					MaxReplicas: 100,
					Metrics: []autoscalingv2.MetricSpec{
						{
							Type: autoscalingv2.ResourceMetricSourceType,
							Resource: &autoscalingv2.ResourceMetricSource{
								Name: "cpu",
								Target: autoscalingv2.MetricTarget{
									Type:               autoscalingv2.UtilizationMetricType,
									AverageUtilization: func(n int32) *int32 { return &n }(90),
								},
							},
						},
					},
				}
				return hpa
			},
		},

		"(native HPA controller) updating autoscaling params": {
			resources: []runtime.Object{
				func(hpa *autoscalingv2.HorizontalPodAutoscaler) runtime.Object {
					hpa.Spec = autoscalingv2.HorizontalPodAutoscalerSpec{
						ScaleTargetRef: autoscalingv2.CrossVersionObjectReference{
							APIVersion: "apps/v1",
							Kind:       "Deployment",
							Name:       "my-instance",
						},
						MinReplicas: func(n int32) *int32 { return &n }(1),
						MaxReplicas: 10,
						Metrics: []autoscalingv2.MetricSpec{
							{
								Type: autoscalingv2.ResourceMetricSourceType,
								Resource: &autoscalingv2.ResourceMetricSource{
									Name: "cpu",
									Target: autoscalingv2.MetricTarget{
										Type:               autoscalingv2.UtilizationMetricType,
										AverageUtilization: func(n int32) *int32 { return &n }(200),
									},
								},
							},
						},
					}
					return hpa
				}(baseExpectedHPA.DeepCopy()),
			},
			instance: func(ri *v1alpha1.RpaasInstance) *v1alpha1.RpaasInstance {
				ri.Spec.Autoscale = &v1alpha1.RpaasInstanceAutoscaleSpec{
					MinReplicas:                       func(n int32) *int32 { return &n }(2),
					MaxReplicas:                       100,
					TargetCPUUtilizationPercentage:    func(n int32) *int32 { return &n }(90),
					TargetMemoryUtilizationPercentage: func(n int32) *int32 { return &n }(70),
				}
				return ri
			},
			expectedHPA: func(hpa *autoscalingv2.HorizontalPodAutoscaler) *autoscalingv2.HorizontalPodAutoscaler {
				hpa.ResourceVersion = "2" // second change
				hpa.Spec = autoscalingv2.HorizontalPodAutoscalerSpec{
					ScaleTargetRef: autoscalingv2.CrossVersionObjectReference{
						APIVersion: "apps/v1",
						Kind:       "Deployment",
						Name:       "my-instance",
					},
					MinReplicas: func(n int32) *int32 { return &n }(2),
					MaxReplicas: 100,
					Metrics: []autoscalingv2.MetricSpec{
						{
							Type: autoscalingv2.ResourceMetricSourceType,
							Resource: &autoscalingv2.ResourceMetricSource{
								Name: "cpu",
								Target: autoscalingv2.MetricTarget{
									Type:               autoscalingv2.UtilizationMetricType,
									AverageUtilization: func(n int32) *int32 { return &n }(90),
								},
							},
						},
						{
							Type: autoscalingv2.ResourceMetricSourceType,
							Resource: &autoscalingv2.ResourceMetricSource{
								Name: "memory",
								Target: autoscalingv2.MetricTarget{
									Type:               autoscalingv2.UtilizationMetricType,
									AverageUtilization: func(n int32) *int32 { return &n }(70),
								},
							},
						},
					},
				}
				return hpa
			},
		},

		"(native HPA controller) removing autoscale params": {
			resources: []runtime.Object{
				baseExpectedHPA.DeepCopy(),
			},
			customAssert: func(t *testing.T, r *RpaasInstanceReconciler) bool {
				var hpa autoscalingv2.HorizontalPodAutoscaler
				err := r.Client.Get(context.TODO(), types.NamespacedName{Name: "my-instance", Namespace: "default"}, &hpa)
				return assert.True(t, k8sErrors.IsNotFound(err))
			},
		},

		"(native HPA controller) with RPS enabled": {
			instance: func(ri *v1alpha1.RpaasInstance) *v1alpha1.RpaasInstance {
				ri.Spec.Autoscale = &v1alpha1.RpaasInstanceAutoscaleSpec{
					MinReplicas:             func(n int32) *int32 { return &n }(2),
					MaxReplicas:             500,
					TargetRequestsPerSecond: func(n int32) *int32 { return &n }(50),
				}
				return ri
			},
			customAssert: func(t *testing.T, r *RpaasInstanceReconciler) bool {
				rec, ok := r.EventRecorder.(*record.FakeRecorder)
				require.True(t, ok, "event recorder must be FakeRecorder")
				return assert.Equal(t, "Warning RpaasInstanceAutoscaleFailed native HPA controller doesn't support RPS metric target yet", <-rec.Events)
			},
		},

		"(KEDA controller) with RPS enabled": {
			instance: func(ri *v1alpha1.RpaasInstance) *v1alpha1.RpaasInstance {
				ri.Spec.Autoscale = &v1alpha1.RpaasInstanceAutoscaleSpec{
					MinReplicas:             func(n int32) *int32 { return &n }(2),
					MaxReplicas:             500,
					TargetRequestsPerSecond: func(n int32) *int32 { return &n }(50),
					KEDAOptions: &v1alpha1.AutoscaleKEDAOptions{
						Enabled:                 true,
						PrometheusServerAddress: "https://prometheus.example.com",
						RPSQueryTemplate:        `sum(rate(nginx_vts_requests_total{instance="{{ .Name }}", namespace="{{ .Namespace }}"}[5m]))`,
					},
				}
				return ri
			},
			expectedScaledObject: func(so *kedav1alpha1.ScaledObject) *kedav1alpha1.ScaledObject {
				so.Spec = kedav1alpha1.ScaledObjectSpec{
					ScaleTargetRef: &kedav1alpha1.ScaleTarget{
						APIVersion: "apps/v1",
						Kind:       "Deployment",
						Name:       "my-instance",
					},
					MinReplicaCount: func(n int32) *int32 { return &n }(2),
					MaxReplicaCount: func(n int32) *int32 { return &n }(500),
					Triggers: []kedav1alpha1.ScaleTriggers{
						{
							Type: "prometheus",
							Metadata: map[string]string{
								"serverAddress": "https://prometheus.example.com",
								"query":         `sum(rate(nginx_vts_requests_total{instance="my-instance", namespace="default"}[5m]))`,
								"threshold":     "50",
							},
						},
					},
				}
				return so
			},
		},

		"(KEDA controller) updating autoscaling params": {
			resources: []runtime.Object{
				func(so *kedav1alpha1.ScaledObject) runtime.Object {
					so.Spec = kedav1alpha1.ScaledObjectSpec{
						ScaleTargetRef: &kedav1alpha1.ScaleTarget{
							APIVersion: "apps/v1",
							Kind:       "Deployment",
							Name:       "my-instance",
						},
						MinReplicaCount: func(n int32) *int32 { return &n }(2),
						MaxReplicaCount: func(n int32) *int32 { return &n }(500),
						Triggers: []kedav1alpha1.ScaleTriggers{
							{
								Type: "prometheus",
								Metadata: map[string]string{
									"serverAddress": "https://prometheus.example.com",
									"query":         `sum(rate(nginx_vts_requests_total{instance="my-instance", namespace="default"}[5m]))`,
									"threshold":     "50",
								},
							},
						},
					}
					return so
				}(baseExpectedScaledObject.DeepCopy()),
			},
			instance: func(ri *v1alpha1.RpaasInstance) *v1alpha1.RpaasInstance {
				ri.Spec.Autoscale = &v1alpha1.RpaasInstanceAutoscaleSpec{
					MinReplicas:                    func(n int32) *int32 { return &n }(5),
					MaxReplicas:                    42,
					TargetCPUUtilizationPercentage: func(n int32) *int32 { return &n }(90),
					TargetRequestsPerSecond:        func(n int32) *int32 { return &n }(100),
					KEDAOptions: &v1alpha1.AutoscaleKEDAOptions{
						Enabled:                 true,
						PrometheusServerAddress: "https://prometheus.example.com",
						RPSQueryTemplate:        `sum(rate(nginx_vts_requests_total{instance="{{ .Name }}", namespace="{{ .Namespace }}"}[5m]))`,
						RPSAuthenticationRef: &kedav1alpha1.ScaledObjectAuthRef{
							Kind: "ClusterTriggerAuthentication",
							Name: "prometheus-auth",
						},
						PollingInterval: func(n int32) *int32 { return &n }(5),
					},
				}
				return ri
			},
			expectedScaledObject: func(so *kedav1alpha1.ScaledObject) *kedav1alpha1.ScaledObject {
				so.ResourceVersion = "2" // second update
				so.Spec = kedav1alpha1.ScaledObjectSpec{
					ScaleTargetRef: &kedav1alpha1.ScaleTarget{
						APIVersion: "apps/v1",
						Kind:       "Deployment",
						Name:       "my-instance",
					},
					MinReplicaCount: func(n int32) *int32 { return &n }(5),
					MaxReplicaCount: func(n int32) *int32 { return &n }(42),
					PollingInterval: func(n int32) *int32 { return &n }(5),
					Triggers: []kedav1alpha1.ScaleTriggers{
						{
							Type:       "cpu",
							MetricType: autoscalingv2.UtilizationMetricType,
							Metadata: map[string]string{
								"value": "90",
							},
						},
						{
							Type: "prometheus",
							Metadata: map[string]string{
								"serverAddress": "https://prometheus.example.com",
								"query":         `sum(rate(nginx_vts_requests_total{instance="my-instance", namespace="default"}[5m]))`,
								"threshold":     "100",
							},
							AuthenticationRef: &kedav1alpha1.ScaledObjectAuthRef{
								Kind: "ClusterTriggerAuthentication",
								Name: "prometheus-auth",
							},
						},
					},
				}
				return so
			},
		},

		"(KEDA controller) removing autoscaling params": {
			resources: []runtime.Object{
				baseExpectedScaledObject.DeepCopy(),
			},
			instance: func(ri *v1alpha1.RpaasInstance) *v1alpha1.RpaasInstance {
				ri.Spec.Autoscale = &v1alpha1.RpaasInstanceAutoscaleSpec{
					KEDAOptions: &v1alpha1.AutoscaleKEDAOptions{
						Enabled:                 true,
						PrometheusServerAddress: "https://prometheus.example.com",
						RPSQueryTemplate:        `sum(rate(nginx_vts_requests_total{instance="{{ .Name }}", namespace="{{ .Namespace }}"}[5m]))`,
					},
				}
				return ri
			},
			customAssert: func(t *testing.T, r *RpaasInstanceReconciler) bool {
				var so kedav1alpha1.ScaledObject
				err := r.Client.Get(context.TODO(), types.NamespacedName{Name: baseExpectedScaledObject.Name, Namespace: baseExpectedScaledObject.Namespace}, &so)
				return assert.True(t, k8sErrors.IsNotFound(err), "ScaledObject resource should not exist")
			},
		},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			instance := &v1alpha1.RpaasInstance{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "my-instance",
					Namespace: metav1.NamespaceDefault,
				},
				Spec: v1alpha1.RpaasInstanceSpec{
					PlanName: "my-plan",
				},
			}
			if tt.instance != nil {
				instance = tt.instance(instance)
			}

			nginx := &nginxv1alpha1.Nginx{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "my-instance",
					Namespace: metav1.NamespaceDefault,
				},
				Status: nginxv1alpha1.NginxStatus{
					Deployments: []nginxv1alpha1.DeploymentStatus{
						{Name: "my-instance"},
					},
				},
			}
			if tt.nginx != nil {
				nginx = tt.nginx(nginx)
			}

			resources := append(tt.resources, instance, nginx)

			r := newRpaasInstanceReconciler(resources...)

			err := r.reconcileHPA(context.TODO(), instance, nginx)
			require.NoError(t, err)

			if tt.expectedHPA == nil && tt.expectedScaledObject == nil && tt.customAssert == nil {
				require.Fail(t, "you must provide either expected HPA and/or ScaledObject or custom assert function")
			}

			if tt.customAssert != nil {
				require.True(t, tt.customAssert(t, r), "custom assert function should return true")
			}

			if tt.expectedHPA != nil {
				var got autoscalingv2.HorizontalPodAutoscaler
				err = r.Client.Get(context.TODO(), types.NamespacedName{Name: instance.Name, Namespace: instance.Namespace}, &got)
				require.NoError(t, err)
				assert.Equal(t, tt.expectedHPA(baseExpectedHPA.DeepCopy()), got.DeepCopy())
			}

			if tt.expectedScaledObject != nil {
				var got kedav1alpha1.ScaledObject
				err = r.Client.Get(context.TODO(), types.NamespacedName{Name: instance.Name, Namespace: instance.Namespace}, &got)
				require.NoError(t, err)
				assert.Equal(t, tt.expectedScaledObject(baseExpectedScaledObject.DeepCopy()), got.DeepCopy())
			}
		})
	}
}

func Test_reconcilePDB(t *testing.T) {
	resources := []runtime.Object{
		&v1alpha1.RpaasInstance{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "another-instance",
				Namespace: "rpaasv2",
			},
			Spec: v1alpha1.RpaasInstanceSpec{
				EnablePodDisruptionBudget: func(b bool) *bool { return &b }(true),
				Replicas:                  func(n int32) *int32 { return &n }(1),
			},
		},

		&policyv1.PodDisruptionBudget{
			TypeMeta: metav1.TypeMeta{
				APIVersion: "policy/v1",
				Kind:       "PodDisruptionBudget",
			},
			ObjectMeta: metav1.ObjectMeta{
				Name:      "another-instance",
				Namespace: "rpaasv2",
				Labels: map[string]string{
					"rpaas.extensions.tsuru.io/instance-name": "another-instance",
					"rpaas.extensions.tsuru.io/plan-name":     "",
				},
				OwnerReferences: []metav1.OwnerReference{
					{
						APIVersion:         "extensions.tsuru.io/v1alpha1",
						Kind:               "RpaasInstance",
						Name:               "another-instance",
						Controller:         func(b bool) *bool { return &b }(true),
						BlockOwnerDeletion: func(b bool) *bool { return &b }(true),
					},
				},
			},
			Spec: policyv1.PodDisruptionBudgetSpec{
				MinAvailable: func(n intstr.IntOrString) *intstr.IntOrString { return &n }(intstr.FromInt(9)),
				Selector: &metav1.LabelSelector{
					MatchLabels: map[string]string{"nginx.tsuru.io/resource-name": "another-instance"},
				},
			},
		},
	}

	tests := map[string]struct {
		instance *v1alpha1.RpaasInstance
		nginx    *nginxv1alpha1.Nginx
		assert   func(t *testing.T, c client.Client)
	}{
		"creating PDB, instance with 1 replicas": {
			instance: &v1alpha1.RpaasInstance{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "my-instance",
					Namespace: "rpaasv2",
				},
				Spec: v1alpha1.RpaasInstanceSpec{
					EnablePodDisruptionBudget: func(b bool) *bool { return &b }(true),
					Replicas:                  func(n int32) *int32 { return &n }(1),
				},
			},
			nginx: &nginxv1alpha1.Nginx{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "my-instance",
					Namespace: "rpaasv2",
				},
				Status: nginxv1alpha1.NginxStatus{
					PodSelector: "nginx.tsuru.io/resource-name=my-instance",
				},
			},
			assert: func(t *testing.T, c client.Client) {
				var pdb policyv1.PodDisruptionBudget
				err := c.Get(context.TODO(), client.ObjectKey{Name: "my-instance", Namespace: "rpaasv2"}, &pdb)
				require.NoError(t, err)
				assert.Equal(t, policyv1.PodDisruptionBudget{
					TypeMeta: metav1.TypeMeta{
						APIVersion: "policy/v1",
						Kind:       "PodDisruptionBudget",
					},
					ObjectMeta: metav1.ObjectMeta{
						Name:      "my-instance",
						Namespace: "rpaasv2",
						Labels: map[string]string{
							"rpaas.extensions.tsuru.io/instance-name": "my-instance",
							"rpaas.extensions.tsuru.io/plan-name":     "",
						},
						ResourceVersion: "1",
						OwnerReferences: []metav1.OwnerReference{
							{
								APIVersion:         "extensions.tsuru.io/v1alpha1",
								Kind:               "RpaasInstance",
								Name:               "my-instance",
								Controller:         func(b bool) *bool { return &b }(true),
								BlockOwnerDeletion: func(b bool) *bool { return &b }(true),
							},
						},
					},
					Spec: policyv1.PodDisruptionBudgetSpec{
						MaxUnavailable: func(n intstr.IntOrString) *intstr.IntOrString { return &n }(intstr.FromString("10%")),
						Selector: &metav1.LabelSelector{
							MatchLabels: map[string]string{"nginx.tsuru.io/resource-name": "my-instance"},
						},
					},
				}, pdb)
			},
		},

		"creating PDB, instance with 10 replicas": {
			instance: &v1alpha1.RpaasInstance{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "my-instance",
					Namespace: "rpaasv2",
				},
				Spec: v1alpha1.RpaasInstanceSpec{
					EnablePodDisruptionBudget: func(b bool) *bool { return &b }(true),
					Replicas:                  func(n int32) *int32 { return &n }(10),
				},
			},
			nginx: &nginxv1alpha1.Nginx{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "my-instance",
					Namespace: "rpaasv2",
				},
				Status: nginxv1alpha1.NginxStatus{
					PodSelector: "nginx.tsuru.io/resource-name=my-instance",
				},
			},
			assert: func(t *testing.T, c client.Client) {
				var pdb policyv1.PodDisruptionBudget
				err := c.Get(context.TODO(), client.ObjectKey{Name: "my-instance", Namespace: "rpaasv2"}, &pdb)
				require.NoError(t, err)
				assert.Equal(t, policyv1.PodDisruptionBudget{
					TypeMeta: metav1.TypeMeta{
						APIVersion: "policy/v1",
						Kind:       "PodDisruptionBudget",
					},
					ObjectMeta: metav1.ObjectMeta{
						Name:      "my-instance",
						Namespace: "rpaasv2",
						Labels: map[string]string{
							"rpaas.extensions.tsuru.io/instance-name": "my-instance",
							"rpaas.extensions.tsuru.io/plan-name":     "",
						},
						ResourceVersion: "1",
						OwnerReferences: []metav1.OwnerReference{
							{
								APIVersion:         "extensions.tsuru.io/v1alpha1",
								Kind:               "RpaasInstance",
								Name:               "my-instance",
								Controller:         func(b bool) *bool { return &b }(true),
								BlockOwnerDeletion: func(b bool) *bool { return &b }(true),
							},
						},
					},
					Spec: policyv1.PodDisruptionBudgetSpec{
						MaxUnavailable: func(n intstr.IntOrString) *intstr.IntOrString { return &n }(intstr.FromString("10%")),
						Selector: &metav1.LabelSelector{
							MatchLabels: map[string]string{"nginx.tsuru.io/resource-name": "my-instance"},
						},
					},
				}, pdb)
			},
		},

		"updating PDB min available": {
			instance: &v1alpha1.RpaasInstance{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "another-instance",
					Namespace: "rpaasv2",
				},
				Spec: v1alpha1.RpaasInstanceSpec{
					EnablePodDisruptionBudget: func(b bool) *bool { return &b }(true),
					Replicas:                  func(n int32) *int32 { return &n }(10),
					Autoscale: &v1alpha1.RpaasInstanceAutoscaleSpec{
						MaxReplicas: int32(100),
						MinReplicas: func(n int32) *int32 { return &n }(int32(50)),
					},
				},
			},
			nginx: &nginxv1alpha1.Nginx{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "another-instance",
					Namespace: "rpaasv2",
				},
				Status: nginxv1alpha1.NginxStatus{
					PodSelector: "nginx.tsuru.io/resource-name=another-instance",
				},
			},
			assert: func(t *testing.T, c client.Client) {
				var pdb policyv1.PodDisruptionBudget
				err := c.Get(context.TODO(), client.ObjectKey{Name: "another-instance", Namespace: "rpaasv2"}, &pdb)
				require.NoError(t, err)
				assert.Equal(t, policyv1.PodDisruptionBudget{
					TypeMeta: metav1.TypeMeta{
						APIVersion: "policy/v1",
						Kind:       "PodDisruptionBudget",
					},
					ObjectMeta: metav1.ObjectMeta{
						Name:      "another-instance",
						Namespace: "rpaasv2",
						Labels: map[string]string{
							"rpaas.extensions.tsuru.io/instance-name": "another-instance",
							"rpaas.extensions.tsuru.io/plan-name":     "",
						},
						ResourceVersion: "1000",
						OwnerReferences: []metav1.OwnerReference{
							{
								APIVersion:         "extensions.tsuru.io/v1alpha1",
								Kind:               "RpaasInstance",
								Name:               "another-instance",
								Controller:         func(b bool) *bool { return &b }(true),
								BlockOwnerDeletion: func(b bool) *bool { return &b }(true),
							},
						},
					},
					Spec: policyv1.PodDisruptionBudgetSpec{
						MaxUnavailable: func(n intstr.IntOrString) *intstr.IntOrString { return &n }(intstr.FromString("10%")),
						Selector: &metav1.LabelSelector{
							MatchLabels: map[string]string{"nginx.tsuru.io/resource-name": "another-instance"},
						},
					},
				}, pdb)
			},
		},

		"removing PDB": {
			instance: &v1alpha1.RpaasInstance{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "another-instance",
					Namespace: "rpaasv2",
				},
				Spec: v1alpha1.RpaasInstanceSpec{
					Replicas: func(n int32) *int32 { return &n }(10),
					Autoscale: &v1alpha1.RpaasInstanceAutoscaleSpec{
						MaxReplicas: int32(100),
						MinReplicas: func(n int32) *int32 { return &n }(int32(50)),
					},
				},
			},
			nginx: &nginxv1alpha1.Nginx{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "another-instance",
					Namespace: "rpaasv2",
				},
				Status: nginxv1alpha1.NginxStatus{
					PodSelector: "nginx.tsuru.io/resource-name=another-instance",
				},
			},
			assert: func(t *testing.T, c client.Client) {
				var pdb policyv1.PodDisruptionBudget
				err := c.Get(context.TODO(), client.ObjectKey{Name: "another-instance", Namespace: "rpaasv2"}, &pdb)
				require.Error(t, err)
				assert.True(t, k8serrors.IsNotFound(err))
			},
		},

		"creating PDB when instance has 0 replicas": {
			instance: &v1alpha1.RpaasInstance{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "my-instance",
					Namespace: "rpaasv2",
				},
				Spec: v1alpha1.RpaasInstanceSpec{
					EnablePodDisruptionBudget: func(b bool) *bool { return &b }(true),
					Replicas:                  func(n int32) *int32 { return &n }(0),
				},
			},
			nginx: &nginxv1alpha1.Nginx{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "my-instance",
					Namespace: "rpaasv2",
				},
				Status: nginxv1alpha1.NginxStatus{
					PodSelector: "nginx.tsuru.io/resource-name=my-instance",
				},
			},
			assert: func(t *testing.T, c client.Client) {
				var pdb policyv1.PodDisruptionBudget
				err := c.Get(context.TODO(), client.ObjectKey{Name: "my-instance", Namespace: "rpaasv2"}, &pdb)
				require.NoError(t, err)
				assert.Equal(t, policyv1.PodDisruptionBudget{
					TypeMeta: metav1.TypeMeta{
						APIVersion: "policy/v1",
						Kind:       "PodDisruptionBudget",
					},
					ObjectMeta: metav1.ObjectMeta{
						Name:      "my-instance",
						Namespace: "rpaasv2",
						Labels: map[string]string{
							"rpaas.extensions.tsuru.io/instance-name": "my-instance",
							"rpaas.extensions.tsuru.io/plan-name":     "",
						},
						ResourceVersion: "1",
						OwnerReferences: []metav1.OwnerReference{
							{
								APIVersion:         "extensions.tsuru.io/v1alpha1",
								Kind:               "RpaasInstance",
								Name:               "my-instance",
								Controller:         func(b bool) *bool { return &b }(true),
								BlockOwnerDeletion: func(b bool) *bool { return &b }(true),
							},
						},
					},
					Spec: policyv1.PodDisruptionBudgetSpec{
						MaxUnavailable: func(n intstr.IntOrString) *intstr.IntOrString { return &n }(intstr.FromString("10%")),
						Selector: &metav1.LabelSelector{
							MatchLabels: map[string]string{"nginx.tsuru.io/resource-name": "my-instance"},
						},
					},
				}, pdb)
			},
		},

		"skip PDB creation when instance disables PDB feature": {
			instance: &v1alpha1.RpaasInstance{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "my-instance",
					Namespace: "rpaasv2",
				},
				Spec: v1alpha1.RpaasInstanceSpec{
					EnablePodDisruptionBudget: func(b bool) *bool { return &b }(false),
					Replicas:                  func(n int32) *int32 { return &n }(10),
				},
			},
			nginx: &nginxv1alpha1.Nginx{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "my-instance",
					Namespace: "rpaasv2",
				},
				Status: nginxv1alpha1.NginxStatus{
					PodSelector: "nginx.tsuru.io/resource-name=my-instance",
				},
			},
			assert: func(t *testing.T, c client.Client) {
				var pdb policyv1.PodDisruptionBudget
				err := c.Get(context.TODO(), client.ObjectKey{Name: "my-instance", Namespace: "rpaasv2"}, &pdb)
				require.Error(t, err)
				assert.True(t, k8serrors.IsNotFound(err))
			},
		},

		"skip PDB creation when nginx status is empty": {
			instance: &v1alpha1.RpaasInstance{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "my-instance",
					Namespace: "rpaasv2",
				},
				Spec: v1alpha1.RpaasInstanceSpec{
					EnablePodDisruptionBudget: func(b bool) *bool { return &b }(true),
					Replicas:                  func(n int32) *int32 { return &n }(10),
				},
			},
			nginx: &nginxv1alpha1.Nginx{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "my-instance",
					Namespace: "rpaasv2",
				},
			},
			assert: func(t *testing.T, c client.Client) {
				var pdb policyv1.PodDisruptionBudget
				err := c.Get(context.TODO(), client.ObjectKey{Name: "my-instance", Namespace: "rpaasv2"}, &pdb)
				require.Error(t, err)
				assert.True(t, k8serrors.IsNotFound(err))
			},
		},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			require.NotNil(t, tt.assert)

			r := newRpaasInstanceReconciler(resources...)
			err := r.reconcilePDB(context.TODO(), tt.instance, tt.nginx)
			require.NoError(t, err)
			tt.assert(t, r.Client)
		})
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

func TestReconcileWithProxyProtocol(t *testing.T) {
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
		},
	}

	defaultFlavor := newRpaasFlavor()
	defaultFlavor.Name = "default"
	defaultFlavor.Spec.Default = true
	defaultFlavor.Spec.InstanceTemplate = &v1alpha1.RpaasInstanceSpec{
		ProxyProtocol: true,
	}
	reconciler := newRpaasInstanceReconciler(rpaas, plan, defaultFlavor)
	result, err := reconciler.Reconcile(context.Background(), reconcile.Request{NamespacedName: types.NamespacedName{Namespace: "default", Name: "my-instance"}})
	require.NoError(t, err)

	assert.Equal(t, result, reconcile.Result{})

	nginx := &nginxv1alpha1.Nginx{}
	err = reconciler.Client.Get(context.TODO(), types.NamespacedName{Name: rpaas.Name, Namespace: rpaas.Namespace}, nginx)
	require.NoError(t, err)
	assert.Equal(t, nginx.Spec.PodTemplate.Ports, []corev1.ContainerPort{
		{Name: "nginx-metrics", ContainerPort: 8800, Protocol: "TCP"},
		{Name: "proxy-http", ContainerPort: 9080, Protocol: "TCP"},
		{Name: "proxy-https", ContainerPort: 9443, Protocol: "TCP"},
	})
}

func TestReconcilePoolNamespaced(t *testing.T) {
	rpaas := &v1alpha1.RpaasInstance{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "my-instance",
			Namespace: "rpaasv2-my-pool",
		},
		Spec: v1alpha1.RpaasInstanceSpec{
			PlanName:      "my-plan",
			PlanNamespace: "default",
		},
	}
	plan := &v1alpha1.RpaasPlan{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "my-plan",
			Namespace: "default",
		},
		Spec: v1alpha1.RpaasPlanSpec{
			Image: "tsuru:pool-namespaces-image:test",
		},
	}
	flavor := &v1alpha1.RpaasFlavor{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "my-flavor",
			Namespace: "default",
		},
		Spec: v1alpha1.RpaasFlavorSpec{
			InstanceTemplate: &v1alpha1.RpaasInstanceSpec{
				Service: &nginxv1alpha1.NginxService{
					Labels: map[string]string{
						"tsuru.io/custom-flavor-label": "foobar",
					},
				},
			},
			Default: true,
		},
	}

	reconciler := newRpaasInstanceReconciler(rpaas, plan, flavor)
	result, err := reconciler.Reconcile(context.TODO(), reconcile.Request{NamespacedName: types.NamespacedName{Namespace: "rpaasv2-my-pool", Name: "my-instance"}})
	require.NoError(t, err)

	assert.Equal(t, result, reconcile.Result{})

	nginx := &nginxv1alpha1.Nginx{}
	err = reconciler.Client.Get(context.TODO(), types.NamespacedName{Name: rpaas.Name, Namespace: rpaas.Namespace}, nginx)
	require.NoError(t, err)

	assert.Equal(t, "tsuru:pool-namespaces-image:test", nginx.Spec.Image)
	assert.Equal(t, "foobar", nginx.Spec.Service.Labels["tsuru.io/custom-flavor-label"])
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
		assert   func(t *testing.T, err error, gotSecret *corev1.Secret, gotCronJob *batchv1.CronJob)
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
			assert: func(t *testing.T, err error, gotSecret *corev1.Secret, gotCronJob *batchv1.CronJob) {
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
						Labels: map[string]string{
							"rpaas.extensions.tsuru.io/component": "session-tickets",
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
				&batchv1.CronJob{
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
			assert: func(t *testing.T, err error, gotSecret *corev1.Secret, gotCronJob *batchv1.CronJob) {
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
				&batchv1.CronJob{
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
			assert: func(t *testing.T, err error, gotSecret *corev1.Secret, gotCronJob *batchv1.CronJob) {
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
			assert: func(t *testing.T, err error, gotSecret *corev1.Secret, gotCronJob *batchv1.CronJob) {
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

			var cronJob batchv1.CronJob
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

func Test_mergeServiceWithDNS(t *testing.T) {
	tests := []struct {
		instance *v1alpha1.RpaasInstance
		expected *nginxv1alpha1.NginxService
	}{
		{},

		{
			instance: &v1alpha1.RpaasInstance{},
		},

		{
			instance: &v1alpha1.RpaasInstance{
				ObjectMeta: metav1.ObjectMeta{
					Name: "my-instance",
				},
				Spec: v1alpha1.RpaasInstanceSpec{
					DNS: &v1alpha1.DNSConfig{
						Zone: "apps.example.com",
					},
				},
			},
		},

		{
			instance: &v1alpha1.RpaasInstance{
				ObjectMeta: metav1.ObjectMeta{
					Name: "my-instance",
				},
				Spec: v1alpha1.RpaasInstanceSpec{
					Service: &nginxv1alpha1.NginxService{},
					DNS: &v1alpha1.DNSConfig{
						Zone: "apps.example.com",
						TTL:  func(n int32) *int32 { return &n }(int32(600)),
					},
				},
			},

			expected: &nginxv1alpha1.NginxService{
				Annotations: map[string]string{
					"external-dns.alpha.kubernetes.io/hostname": "my-instance.apps.example.com",
					"external-dns.alpha.kubernetes.io/ttl":      "600",
				},
			},
		},

		{
			instance: &v1alpha1.RpaasInstance{
				ObjectMeta: metav1.ObjectMeta{
					Name: "my-instance",
				},
				Spec: v1alpha1.RpaasInstanceSpec{
					Service: &nginxv1alpha1.NginxService{
						Annotations: map[string]string{
							"external-dns.alpha.kubernetes.io/hostname": "www.example.com,www.example.org",
						},
					},

					DNS: &v1alpha1.DNSConfig{
						Zone: "apps.example.com",
						TTL:  func(n int32) *int32 { return &n }(int32(600)),
					},
				},
			},

			expected: &nginxv1alpha1.NginxService{
				Annotations: map[string]string{
					"external-dns.alpha.kubernetes.io/hostname": "my-instance.apps.example.com,www.example.com,www.example.org",
					"external-dns.alpha.kubernetes.io/ttl":      "600",
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run("", func(t *testing.T) {
			assert.Equal(t, tt.expected, mergeServiceWithDNS(tt.instance))
		})
	}
}

func TestRpaasInstanceController_Reconcile_Suspended(t *testing.T) {
	t.Parallel()

	i := v1alpha1.RpaasInstance{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "my-instance",
			Namespace: "default",
		},
		Spec: v1alpha1.RpaasInstanceSpec{
			Suspend: func(b bool) *bool { return &b }(true),
		},
	}

	r := newRpaasInstanceReconciler(i.DeepCopy())
	result, err := r.Reconcile(context.TODO(), ctrl.Request{NamespacedName: types.NamespacedName{Name: i.Name, Namespace: i.Namespace}})
	require.NoError(t, err)
	assert.Equal(t, ctrl.Result{Requeue: true}, result)

	fr, ok := r.EventRecorder.(*record.FakeRecorder)
	require.True(t, ok)
	assert.Equal(t, "Warning RpaasInstanceSuspended no modifications will be done by RPaaS controller", <-fr.Events)
}

func newRpaasInstanceReconciler(objs ...runtime.Object) *RpaasInstanceReconciler {
	return &RpaasInstanceReconciler{
		Client:        fake.NewClientBuilder().WithScheme(extensionsruntime.NewScheme()).WithRuntimeObjects(objs...).Build(),
		EventRecorder: record.NewFakeRecorder(1),
		Log:           ctrl.Log,
	}
}
