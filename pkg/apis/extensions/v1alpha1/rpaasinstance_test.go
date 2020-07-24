// Copyright 2020 tsuru authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package v1alpha1

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/tsuru/rpaas-operator/config"
	corev1 "k8s.io/api/core/v1"
)

func Test_SetTeamOwner(t *testing.T) {
	instance := &RpaasInstance{}
	instance.SetTeamOwner("team-one")
	expected := map[string]string{teamOwnerLabel: "team-one"}
	assert.Equal(t, expected, instance.Labels)
	assert.Equal(t, expected, instance.Annotations)
	assert.Equal(t, expected, instance.Spec.PodTemplate.Labels)

	instance.SetTeamOwner("team-two")
	expected = map[string]string{teamOwnerLabel: "team-two"}
	assert.Equal(t, expected, instance.Labels)
	assert.Equal(t, expected, instance.Annotations)
	assert.Equal(t, expected, instance.Spec.PodTemplate.Labels)
}

func Test_GetTeamOwner(t *testing.T) {
	instance := &RpaasInstance{}
	owner := instance.TeamOwner()
	assert.Equal(t, "", owner)
	instance.SetTeamOwner("team-one")
	owner = instance.TeamOwner()
	assert.Equal(t, "team-one", owner)
}

func Test_SetTeamAffinity(t *testing.T) {
	defaultAffinity := corev1.Affinity{
		NodeAffinity: &corev1.NodeAffinity{
			RequiredDuringSchedulingIgnoredDuringExecution: &corev1.NodeSelector{
				NodeSelectorTerms: []corev1.NodeSelectorTerm{
					{
						MatchExpressions: []corev1.NodeSelectorRequirement{
							{
								Key:      "machine-type",
								Operator: corev1.NodeSelectorOpIn,
								Values:   []string{"default"},
							},
						},
					},
				},
			},
		},
	}

	teamOneAffinity := corev1.Affinity{
		NodeAffinity: &corev1.NodeAffinity{
			RequiredDuringSchedulingIgnoredDuringExecution: &corev1.NodeSelector{
				NodeSelectorTerms: []corev1.NodeSelectorTerm{
					{
						MatchExpressions: []corev1.NodeSelectorRequirement{
							{
								Key:      "machine-type",
								Operator: corev1.NodeSelectorOpIn,
								Values:   []string{"ultra-fast-io"},
							},
						},
					},
				},
			},
		},
	}

	customAffinity := corev1.Affinity{
		NodeAffinity: &corev1.NodeAffinity{
			RequiredDuringSchedulingIgnoredDuringExecution: &corev1.NodeSelector{
				NodeSelectorTerms: []corev1.NodeSelectorTerm{
					{
						MatchExpressions: []corev1.NodeSelectorRequirement{
							{
								Key:      "machine-type",
								Operator: corev1.NodeSelectorOpIn,
								Values:   []string{"another-criteria"},
							},
						},
					},
				},
			},
		},
	}

	baseConfig := config.RpaasConfig{
		DefaultAffinity: &defaultAffinity,
		TeamAffinity: map[string]corev1.Affinity{
			"team-one": teamOneAffinity,
		},
	}
	config.Set(baseConfig)
	defer config.Set(config.RpaasConfig{})
	instance := &RpaasInstance{}

	instance.SetTeamAffinity()
	assert.Equal(t, &defaultAffinity, instance.Spec.PodTemplate.Affinity)

	instance.Spec.PodTemplate.Affinity = nil
	instance.SetTeamOwner("team-one")
	instance.SetTeamAffinity()
	assert.Equal(t, &teamOneAffinity, instance.Spec.PodTemplate.Affinity)

	instance.Spec.PodTemplate.Affinity = &customAffinity
	instance.SetTeamAffinity()
	assert.Equal(t, &customAffinity, instance.Spec.PodTemplate.Affinity)

	instance.SetTeamOwner("team-two")
	instance.Spec.PodTemplate.Affinity = &customAffinity
	instance.SetTeamAffinity()
	assert.Equal(t, &customAffinity, instance.Spec.PodTemplate.Affinity)
}
