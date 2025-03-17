// Copyright 2021 tsuru authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package util_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	nginxv1alpha1 "github.com/tsuru/nginx-operator/api/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/tsuru/rpaas-operator/api/v1alpha1"
	"github.com/tsuru/rpaas-operator/internal/controllers/util"
)

func TestRenderCustomValues(t *testing.T) {
	tests := map[string]struct {
		instance *v1alpha1.RpaasInstance
		expected *v1alpha1.RpaasInstance
	}{
		"when instance is nil": {},

		"with custom service annotations": {
			instance: &v1alpha1.RpaasInstance{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "my-instance",
					Namespace: "rpaasv2",
				},
				Spec: v1alpha1.RpaasInstanceSpec{
					Service: &nginxv1alpha1.NginxService{
						Annotations: map[string]string{
							"key1": `{{ printf "%s-1234" .Name }}`,
							"key2": `{{ .Namespace }}/{{ .Name }}`,
						},
					},
				},
			},
			expected: &v1alpha1.RpaasInstance{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "my-instance",
					Namespace: "rpaasv2",
				},
				Spec: v1alpha1.RpaasInstanceSpec{
					Service: &nginxv1alpha1.NginxService{
						Annotations: map[string]string{
							"key1": "my-instance-1234",
							"key2": "rpaasv2/my-instance",
						},
					},
				},
			},
		},

		"with custom ingress annotations": {
			instance: &v1alpha1.RpaasInstance{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "my-instance",
					Namespace: "rpaasv2",
				},
				Spec: v1alpha1.RpaasInstanceSpec{
					Ingress: &nginxv1alpha1.NginxIngress{
						Annotations: map[string]string{
							"key1": `{{ printf "%s-1234" .Name }}`,
							"key2": `{{ .Namespace }}/{{ .Name }}`,
						},
					},
				},
			},
			expected: &v1alpha1.RpaasInstance{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "my-instance",
					Namespace: "rpaasv2",
				},
				Spec: v1alpha1.RpaasInstanceSpec{
					Ingress: &nginxv1alpha1.NginxIngress{
						Annotations: map[string]string{
							"key1": "my-instance-1234",
							"key2": "rpaasv2/my-instance",
						},
					},
				},
			},
		},

		"with custom required pod affinity": {
			instance: &v1alpha1.RpaasInstance{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "my-instance",
					Namespace: "rpaasv2",
				},
				Spec: v1alpha1.RpaasInstanceSpec{
					PodTemplate: nginxv1alpha1.NginxPodTemplateSpec{
						Affinity: &corev1.Affinity{
							PodAffinity: &corev1.PodAffinity{
								RequiredDuringSchedulingIgnoredDuringExecution: []corev1.PodAffinityTerm{
									{
										LabelSelector: &metav1.LabelSelector{
											MatchLabels: map[string]string{"example-key1": `{{ .Namespace }}/{{ .Name }}`},
											MatchExpressions: []metav1.LabelSelectorRequirement{
												{Key: "example-key2", Operator: metav1.LabelSelectorOpIn, Values: []string{`{{ printf "%s-1234" .Name }}`, "fallback"}},
											},
										},
									},
								},
							},
						},
					},
				},
			},
			expected: &v1alpha1.RpaasInstance{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "my-instance",
					Namespace: "rpaasv2",
				},
				Spec: v1alpha1.RpaasInstanceSpec{
					PodTemplate: nginxv1alpha1.NginxPodTemplateSpec{
						Affinity: &corev1.Affinity{
							PodAffinity: &corev1.PodAffinity{
								RequiredDuringSchedulingIgnoredDuringExecution: []corev1.PodAffinityTerm{
									{
										LabelSelector: &metav1.LabelSelector{
											MatchLabels: map[string]string{"example-key1": "rpaasv2/my-instance"},
											MatchExpressions: []metav1.LabelSelectorRequirement{
												{Key: "example-key2", Operator: metav1.LabelSelectorOpIn, Values: []string{"my-instance-1234", "fallback"}},
											},
										},
									},
								},
							},
						},
					},
				},
			},
		},

		"with custom required pod anti affinity": {
			instance: &v1alpha1.RpaasInstance{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "my-instance",
					Namespace: "rpaasv2",
				},
				Spec: v1alpha1.RpaasInstanceSpec{
					PodTemplate: nginxv1alpha1.NginxPodTemplateSpec{
						Affinity: &corev1.Affinity{
							PodAntiAffinity: &corev1.PodAntiAffinity{
								RequiredDuringSchedulingIgnoredDuringExecution: []corev1.PodAffinityTerm{
									{
										LabelSelector: &metav1.LabelSelector{
											MatchLabels: map[string]string{"example-key1": `{{ .Namespace }}/{{ .Name }}`},
											MatchExpressions: []metav1.LabelSelectorRequirement{
												{Key: "example-key2", Operator: metav1.LabelSelectorOpIn, Values: []string{`{{ printf "%s-1234" .Name }}`, "fallback"}},
											},
										},
									},
								},
							},
						},
					},
				},
			},
			expected: &v1alpha1.RpaasInstance{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "my-instance",
					Namespace: "rpaasv2",
				},
				Spec: v1alpha1.RpaasInstanceSpec{
					PodTemplate: nginxv1alpha1.NginxPodTemplateSpec{
						Affinity: &corev1.Affinity{
							PodAntiAffinity: &corev1.PodAntiAffinity{
								RequiredDuringSchedulingIgnoredDuringExecution: []corev1.PodAffinityTerm{
									{
										LabelSelector: &metav1.LabelSelector{
											MatchLabels: map[string]string{"example-key1": "rpaasv2/my-instance"},
											MatchExpressions: []metav1.LabelSelectorRequirement{
												{Key: "example-key2", Operator: metav1.LabelSelectorOpIn, Values: []string{"my-instance-1234", "fallback"}},
											},
										},
									},
								},
							},
						},
					},
				},
			},
		},

		"with custom preferred pod affinity": {
			instance: &v1alpha1.RpaasInstance{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "my-instance",
					Namespace: "rpaasv2",
				},
				Spec: v1alpha1.RpaasInstanceSpec{
					PodTemplate: nginxv1alpha1.NginxPodTemplateSpec{
						Affinity: &corev1.Affinity{
							PodAffinity: &corev1.PodAffinity{
								PreferredDuringSchedulingIgnoredDuringExecution: []corev1.WeightedPodAffinityTerm{
									{
										Weight: int32(100),
										PodAffinityTerm: corev1.PodAffinityTerm{
											LabelSelector: &metav1.LabelSelector{
												MatchLabels: map[string]string{"example-key1": `{{ .Namespace }}/{{ .Name }}`},
												MatchExpressions: []metav1.LabelSelectorRequirement{
													{Key: "example-key1", Operator: metav1.LabelSelectorOpIn, Values: []string{`{{ printf "%s-1234" .Name }}`, "fallback"}},
												},
											},
										},
									},
								},
							},
						},
					},
				},
			},
			expected: &v1alpha1.RpaasInstance{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "my-instance",
					Namespace: "rpaasv2",
				},
				Spec: v1alpha1.RpaasInstanceSpec{
					PodTemplate: nginxv1alpha1.NginxPodTemplateSpec{
						Affinity: &corev1.Affinity{
							PodAffinity: &corev1.PodAffinity{
								PreferredDuringSchedulingIgnoredDuringExecution: []corev1.WeightedPodAffinityTerm{
									{
										Weight: int32(100),
										PodAffinityTerm: corev1.PodAffinityTerm{
											LabelSelector: &metav1.LabelSelector{
												MatchLabels: map[string]string{"example-key1": "rpaasv2/my-instance"},
												MatchExpressions: []metav1.LabelSelectorRequirement{
													{Key: "example-key1", Operator: metav1.LabelSelectorOpIn, Values: []string{"my-instance-1234", "fallback"}},
												},
											},
										},
									},
								},
							},
						},
					},
				},
			},
		},

		"with custom preferred pod anti affinity": {
			instance: &v1alpha1.RpaasInstance{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "my-instance",
					Namespace: "rpaasv2",
				},
				Spec: v1alpha1.RpaasInstanceSpec{
					PodTemplate: nginxv1alpha1.NginxPodTemplateSpec{
						Affinity: &corev1.Affinity{
							PodAntiAffinity: &corev1.PodAntiAffinity{
								PreferredDuringSchedulingIgnoredDuringExecution: []corev1.WeightedPodAffinityTerm{
									{
										Weight: int32(100),
										PodAffinityTerm: corev1.PodAffinityTerm{
											LabelSelector: &metav1.LabelSelector{
												MatchLabels: map[string]string{"example-key1": `{{ .Namespace }}/{{ .Name }}`},
												MatchExpressions: []metav1.LabelSelectorRequirement{
													{Key: "example-key1", Operator: metav1.LabelSelectorOpIn, Values: []string{`{{ printf "%s-1234" .Name }}`, "fallback"}},
												},
											},
										},
									},
								},
							},
						},
					},
				},
			},
			expected: &v1alpha1.RpaasInstance{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "my-instance",
					Namespace: "rpaasv2",
				},
				Spec: v1alpha1.RpaasInstanceSpec{
					PodTemplate: nginxv1alpha1.NginxPodTemplateSpec{
						Affinity: &corev1.Affinity{
							PodAntiAffinity: &corev1.PodAntiAffinity{
								PreferredDuringSchedulingIgnoredDuringExecution: []corev1.WeightedPodAffinityTerm{
									{
										Weight: int32(100),
										PodAffinityTerm: corev1.PodAffinityTerm{
											LabelSelector: &metav1.LabelSelector{
												MatchLabels: map[string]string{"example-key1": "rpaasv2/my-instance"},
												MatchExpressions: []metav1.LabelSelectorRequirement{
													{Key: "example-key1", Operator: metav1.LabelSelectorOpIn, Values: []string{"my-instance-1234", "fallback"}},
												},
											},
										},
									},
								},
							},
						},
					},
				},
			},
		},

		"with custom topology spread constraints": {
			instance: &v1alpha1.RpaasInstance{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "my-instance",
					Namespace: "rpaasv2",
				},
				Spec: v1alpha1.RpaasInstanceSpec{
					PodTemplate: nginxv1alpha1.NginxPodTemplateSpec{
						TopologySpreadConstraints: []corev1.TopologySpreadConstraint{
							{MaxSkew: 1, TopologyKey: "zone", WhenUnsatisfiable: corev1.ScheduleAnyway, LabelSelector: &metav1.LabelSelector{
								MatchLabels: map[string]string{"example-key1": `{{ .Namespace }}`},
							}}, {MaxSkew: 1, TopologyKey: "host", WhenUnsatisfiable: corev1.ScheduleAnyway, LabelSelector: &metav1.LabelSelector{
								MatchLabels: map[string]string{"example-key2": `{{ .Name }}`},
							}},
						},
					},
				},
			},
			expected: &v1alpha1.RpaasInstance{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "my-instance",
					Namespace: "rpaasv2",
				},
				Spec: v1alpha1.RpaasInstanceSpec{
					PodTemplate: nginxv1alpha1.NginxPodTemplateSpec{
						TopologySpreadConstraints: []corev1.TopologySpreadConstraint{
							{MaxSkew: 1, TopologyKey: "zone", WhenUnsatisfiable: corev1.ScheduleAnyway, LabelSelector: &metav1.LabelSelector{
								MatchLabels: map[string]string{"example-key1": "rpaasv2"},
							}}, {MaxSkew: 1, TopologyKey: "host", WhenUnsatisfiable: corev1.ScheduleAnyway, LabelSelector: &metav1.LabelSelector{
								MatchLabels: map[string]string{"example-key2": "my-instance"},
							}},
						},
					},
				},
			},
		},
		"with custom pod spec": {
			instance: &v1alpha1.RpaasInstance{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "my-instance",
					Namespace: "rpaasv2",
				},
				Spec: v1alpha1.RpaasInstanceSpec{
					PodTemplate: nginxv1alpha1.NginxPodTemplateSpec{
						Containers: []corev1.Container{
							{
								Name: "sidecar1",
								Env: []corev1.EnvVar{
									{Name: "EXAMPLE_ENV1", Value: `{{ .Namespace }}`},
									{Name: "EXAMPLE_ENV2", Value: `{{ .Name }}`},
								},
							},
							{
								Name:    "sidecar2",
								Command: []string{"echo", `{{ .Namespace }}`, `{{ .Name }}`},
							},
							{
								Name: "sidecar3",
								Args: []string{`{{ .Namespace }}`, `{{ .Name }}`},
							},
						},
						InitContainers: []corev1.Container{
							{
								Name: "init-container",
								Env: []corev1.EnvVar{
									{Name: "EXAMPLE_ENV1", Value: `{{ .Namespace }}`},
									{Name: "EXAMPLE_ENV2", Value: `{{ .Name }}`},
								},
							},
						},
					},
				},
			},
			expected: &v1alpha1.RpaasInstance{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "my-instance",
					Namespace: "rpaasv2",
				},
				Spec: v1alpha1.RpaasInstanceSpec{
					PodTemplate: nginxv1alpha1.NginxPodTemplateSpec{
						Containers: []corev1.Container{
							{
								Name: "sidecar1",
								Env: []corev1.EnvVar{
									{Name: "EXAMPLE_ENV1", Value: "rpaasv2"},
									{Name: "EXAMPLE_ENV2", Value: "my-instance"},
								},
							},
							{
								Name:    "sidecar2",
								Command: []string{"echo", "rpaasv2", "my-instance"},
							},
							{
								Name: "sidecar3",
								Args: []string{"rpaasv2", "my-instance"},
							},
						},
						InitContainers: []corev1.Container{
							{
								Name: "init-container",
								Env: []corev1.EnvVar{
									{Name: "EXAMPLE_ENV1", Value: "rpaasv2"},
									{Name: "EXAMPLE_ENV2", Value: "my-instance"},
								},
							},
						},
					},
				},
			},
		},
	}
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			err := util.RenderCustomValues(tt.instance)
			require.NoError(t, err)
			assert.Equal(t, tt.instance, tt.expected)
		})
	}
}
