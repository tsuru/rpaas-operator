// Copyright 2020 tsuru authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package v1alpha1

const (
	teamOwnerLabel   = "rpaas.extensions.tsuru.io/team-owner"
	clusterNameLabel = "rpaas.extensions.tsuru.io/cluster-name"
)

func (i *RpaasInstance) SetTeamOwner(team string) {
	newLabels := map[string]string{teamOwnerLabel: team}
	i.appendNewLabels(newLabels)
}

func (i *RpaasInstance) SetClusterName(clusterName string) {
	newLabels := map[string]string{clusterNameLabel: clusterName}
	i.appendNewLabels(newLabels)
}

func (i *RpaasInstance) appendNewLabels(newLabels map[string]string) {
	i.Labels = mergeMap(i.Labels, newLabels)
	i.Annotations = mergeMap(i.Annotations, newLabels)
	i.Spec.PodTemplate.Labels = mergeMap(i.Spec.PodTemplate.Labels, newLabels)
}

func (i *RpaasInstance) TeamOwner() string {
	return i.Labels[teamOwnerLabel]
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
