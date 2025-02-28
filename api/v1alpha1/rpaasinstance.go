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

func (s *RpaasInstanceSpec) CertManagerRequests(name string) (reqs []CertManager) {
	if s == nil || s.DynamicCertificates == nil {
		return
	}

	uniqueCertsByIssuer := make(map[string]*CertManager)
	uniqueCertsByName := make(map[string]*CertManager)

	if req := s.DynamicCertificates.CertManager; req != nil {
		r := req.DeepCopy()
		r.DNSNames = r.dnsNames(name, s)

		if req.Name != "" {
			uniqueCertsByName[req.Name] = r
		} else {
			uniqueCertsByIssuer[r.Issuer] = r
		}
	}

	for _, req := range s.DynamicCertificates.CertManagerRequests {

		if req.Name != "" {
			r, found := uniqueCertsByName[req.Name]
			if found {
				r.DNSNames = append(r.DNSNames, req.dnsNames(name, s)...)
				r.IPAddresses = append(r.IPAddresses, req.IPAddresses...)
			} else {
				uniqueCertsByName[req.Name] = req.DeepCopy()
			}

			continue
		}

		r, found := uniqueCertsByIssuer[req.Issuer]
		if !found {
			uniqueCertsByIssuer[req.Issuer] = req.DeepCopy()
			continue
		}

		r.DNSNames = append(r.DNSNames, req.dnsNames(name, s)...)
		r.IPAddresses = append(r.IPAddresses, req.IPAddresses...)
	}

	for _, v := range uniqueCertsByName {
		reqs = append(reqs, *v)
	}
	for _, v := range uniqueCertsByIssuer {
		reqs = append(reqs, *v)
	}

	sort.Slice(reqs, func(i, j int) bool {
		if reqs[i].Name != reqs[j].Name {
			return reqs[i].Name < reqs[j].Name
		}

		return reqs[i].Issuer < reqs[j].Issuer
	})

	return
}

func (c *CertManager) dnsNames(name string, spec *RpaasInstanceSpec) (names []string) {
	if c == nil {
		return
	}

	names = append(names, c.DNSNames...)
	if c.DNSNamesDefault && spec.DNS != nil && spec.DNS.Zone != "" {
		names = append(names, fmt.Sprintf("%s.%s", name, spec.DNS.Zone))
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

func (i *RpaasValidation) BelongsToCluster(clusterName string) bool {
	instanceCluster := i.Labels[RpaasOperatorClusterNameLabelKey]

	if instanceCluster == "" {
		return false
	}

	return clusterName == instanceCluster
}
