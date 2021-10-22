// Copyright 2020 tsuru authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package v1alpha1

import (
	"fmt"
	"sort"
)

const (
	teamOwnerLabel   = "rpaas.extensions.tsuru.io/team-owner"
	clusterNameLabel = "rpaas.extensions.tsuru.io/cluster-name"
)

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

func (i *RpaasInstance) SetTeamOwner(team string) {
	newLabels := map[string]string{teamOwnerLabel: team}
	i.appendNewLabels(newLabels)
}

func (i *RpaasInstance) SetClusterName(clusterName string) {
	newLabels := map[string]string{clusterNameLabel: clusterName}
	i.appendNewLabels(newLabels)
}

func (i *RpaasInstance) BelongsToCluster(clusterName string) bool {
	instanceCluster := i.Labels[clusterNameLabel]

	if instanceCluster == "" {
		return false
	}

	return clusterName == instanceCluster
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
