// Copyright 2020 tsuru authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package v1alpha1

import "github.com/tsuru/rpaas-operator/config"

const (
	teamOwnerLabel = "rpaas.extensions.tsuru.io/team-owner"
)

func (i *RpaasInstance) SetTeamOwner(team string) {
	newLabels := map[string]string{teamOwnerLabel: team}
	i.Labels = mergeMap(i.Labels, newLabels)
	i.Annotations = mergeMap(i.Annotations, newLabels)
	i.Spec.PodTemplate.Labels = mergeMap(i.Spec.PodTemplate.Labels, newLabels)
}

func (i *RpaasInstance) TeamOwner() string {
	return i.Labels[teamOwnerLabel]
}

func (i *RpaasInstance) SetTeamAffinity() {
	if i.Spec.PodTemplate.Affinity != nil {
		return
	}
	team := i.TeamOwner()
	conf := config.Get()
	if conf.TeamAffinity != nil {
		if teamAffinity, ok := conf.TeamAffinity[team]; ok {
			i.Spec.PodTemplate.Affinity = &teamAffinity
			return
		}
	}
	i.Spec.PodTemplate.Affinity = conf.DefaultAffinity
}

func mergeMap(a, b map[string]string) map[string]string {
	if a == nil {
		return b
	}
	for k, v := range b {
		a[k] = v
	}
	return a
}
