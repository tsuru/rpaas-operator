package v1alpha1

const(
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

func mergeMap(a, b map[string]string) map[string]string {
	if a == nil {
		return b
	}
	for k, v := range b {
		a[k] = v
	}
	return a
}
