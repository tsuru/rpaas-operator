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
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			err := util.RenderCustomValues(tt.instance)
			require.NoError(t, err)
			assert.Equal(t, tt.instance, tt.expected)
		})
	}
}
