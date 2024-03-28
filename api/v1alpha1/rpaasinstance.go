// Copyright 2020 tsuru authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package v1alpha1

import (
	"fmt"
	"sort"
)

const (
	DefaultLabelKeyPrefix = "rpaas.extensions.tsuru.io"

	RpaasOperatorValidationNameLabelKey      = DefaultLabelKeyPrefix + "/validation-name"
	RpaasOperatorValidationHashAnnotationKey = DefaultLabelKeyPrefix + "/validation-hash"

	RpaasOperatorInstanceNameLabelKey = DefaultLabelKeyPrefix + "/instance-name"
	RpaasOperatorServiceNameLabelKey  = DefaultLabelKeyPrefix + "/service-name"
	RpaasOperatorPlanNameLabelKey     = DefaultLabelKeyPrefix + "/plan-name"
	RpaasOperatorTeamOwnerLabelKey    = DefaultLabelKeyPrefix + "/team-owner"
	RpaasOperatorClusterNameLabelKey  = DefaultLabelKeyPrefix + "/cluster-name"

	LegacyRpaasOperatorInstanceNameLabelKey = "rpaas_instance"
	LegacyRpaasOperatorServiceNameLabelKey  = "rpaas_service"
)

func (i *RpaasInstance) GetBaseLabels(labels map[string]string) map[string]string {
	return mergeMap(map[string]string{
		LegacyRpaasOperatorInstanceNameLabelKey: i.Name,
		LegacyRpaasOperatorServiceNameLabelKey:  i.Labels[RpaasOperatorServiceNameLabelKey],
		RpaasOperatorInstanceNameLabelKey:       i.Name,
		RpaasOperatorServiceNameLabelKey:        i.Labels[RpaasOperatorServiceNameLabelKey],
		RpaasOperatorPlanNameLabelKey:           i.Spec.PlanName,
		RpaasOperatorTeamOwnerLabelKey:          i.Labels[RpaasOperatorTeamOwnerLabelKey],
	}, labels)
}

func (i *RpaasInstance) TeamOwner() string {
	return i.Labels[RpaasOperatorTeamOwnerLabelKey]
}

func (i *RpaasInstance) ClusterName() string {
	return i.Labels[RpaasOperatorClusterNameLabelKey]
}

func (i *RpaasInstance) SetTeamOwner(team string) {
	newLabels := map[string]string{RpaasOperatorTeamOwnerLabelKey: team}
	i.appendNewLabels(newLabels)
}

func (i *RpaasInstance) SetClusterName(clusterName string) {
	newLabels := map[string]string{RpaasOperatorClusterNameLabelKey: clusterName}
	i.appendNewLabels(newLabels)
}

func (i *RpaasInstance) BelongsToCluster(clusterName string) bool {
	instanceCluster := i.Labels[RpaasOperatorClusterNameLabelKey]

	if instanceCluster == "" {
		return false
	}

	return clusterName == instanceCluster
}

func (i *RpaasInstance) CertManagerRequests() (reqs []CertManager) {
	if i == nil || i.Spec.DynamicCertificates == nil {
		return
	}

	uniqueCerts := make(map[string]*CertManager)
	if req := i.Spec.DynamicCertificates.CertManager; req != nil {
		r := req.DeepCopy()
		r.DNSNames = r.dnsNames(i)
		uniqueCerts[r.Issuer] = r
	}

	for _, req := range i.Spec.DynamicCertificates.CertManagerRequests {
		r, found := uniqueCerts[req.Issuer]
		if !found {
			uniqueCerts[req.Issuer] = req.DeepCopy()
			continue
		}

		r.DNSNames = append(r.DNSNames, req.dnsNames(i)...)
		r.IPAddresses = append(r.IPAddresses, req.IPAddresses...)
	}

	for _, v := range uniqueCerts {
		reqs = append(reqs, *v)
	}

	sort.Slice(reqs, func(i, j int) bool { return reqs[i].Issuer < reqs[j].Issuer })

	return
}

func (c *CertManager) dnsNames(i *RpaasInstance) (names []string) {
	if c == nil {
		return
	}

	names = append(names, c.DNSNames...)
	if c.DNSNamesDefault && i.Spec.DNS != nil && i.Spec.DNS.Zone != "" {
		names = append(names, fmt.Sprintf("%s.%s", i.Name, i.Spec.DNS.Zone))
	}

	return
}

func (i *RpaasInstance) appendNewLabels(newLabels map[string]string) {
	i.Labels = mergeMap(i.Labels, newLabels)
	i.Annotations = mergeMap(i.Annotations, newLabels)
	i.Spec.PodTemplate.Labels = mergeMap(i.Spec.PodTemplate.Labels, newLabels)
	if i.Spec.Service != nil {
		i.Spec.Service.Labels = mergeMap(i.Spec.Service.Labels, newLabels)
	}
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
