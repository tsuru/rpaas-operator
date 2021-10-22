// Copyright 2020 tsuru authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package v1alpha1

import (
	"testing"

	"github.com/stretchr/testify/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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

func Test_BelongsToCluster(t *testing.T) {
	instance := &RpaasInstance{}
	belongs := instance.BelongsToCluster("cluster01")
	assert.Equal(t, false, belongs)

	instance = &RpaasInstance{
		ObjectMeta: metav1.ObjectMeta{
			Labels: map[string]string{
				clusterNameLabel: "cluster01",
			},
		},
	}

	belongs = instance.BelongsToCluster("cluster02")
	assert.Equal(t, false, belongs)

	belongs = instance.BelongsToCluster("cluster01")
	assert.Equal(t, true, belongs)
}

func TestCertManagerRequests(t *testing.T) {
	instance := &RpaasInstance{
		Spec: RpaasInstanceSpec{
			// this is a default certificate
			DynamicCertificates: &DynamicCertificates{
				CertManager: &CertManager{
					Issuer: "my-issuer",
					DNSNames: []string{
						"default-domain.my-company.io",
					},
					IPAddresses: []string{
						"10.1.1.1",
					},
				},
				CertManagerRequests: []CertManager{
					{
						Issuer: "my-issuer",
						DNSNames: []string{
							"custom-domain.my-company.io",
						},
						IPAddresses: []string{
							"10.1.1.2",
						},
					},
				},
			},
		},
	}

	assert.Equal(t, []CertManager{
		{
			Issuer:          "my-issuer",
			DNSNames:        []string{"default-domain.my-company.io", "custom-domain.my-company.io"},
			IPAddresses:     []string{"10.1.1.1", "10.1.1.2"},
			DNSNamesDefault: false,
		},
	}, instance.CertManagerRequests())

}
