// Copyright 2020 tsuru authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package v1alpha1

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/tsuru/nginx-operator/api/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func Test_SetTeamOwner(t *testing.T) {
	instance := &RpaasInstance{}
	instance.Spec.Service = &v1alpha1.NginxService{}
	instance.SetTeamOwner("team-one")
	expected := map[string]string{RpaasOperatorTeamOwnerLabelKey: "team-one"}
	assert.Equal(t, expected, instance.Labels)
	assert.Equal(t, expected, instance.Annotations)
	assert.Equal(t, expected, instance.Spec.PodTemplate.Labels)
	assert.Equal(t, expected, instance.Spec.Service.Labels)

	instance.SetTeamOwner("team-two")
	expected = map[string]string{RpaasOperatorTeamOwnerLabelKey: "team-two"}
	assert.Equal(t, expected, instance.Labels)
	assert.Equal(t, expected, instance.Annotations)
	assert.Equal(t, expected, instance.Spec.PodTemplate.Labels)
	assert.Equal(t, expected, instance.Spec.Service.Labels)
}

func Test_GetTeamOwner(t *testing.T) {
	instance := &RpaasInstance{}
	assert.Equal(t, "", instance.TeamOwner())
	instance.SetTeamOwner("team-one")
	assert.Equal(t, "team-one", instance.TeamOwner())
}

func Test_BelongsToCluster(t *testing.T) {
	instance := &RpaasInstance{}
	belongs := instance.BelongsToCluster("cluster01")
	assert.Equal(t, false, belongs)

	instance = &RpaasInstance{
		ObjectMeta: metav1.ObjectMeta{
			Labels: map[string]string{
				RpaasOperatorClusterNameLabelKey: "cluster01",
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
					DNSNamesDefault: true,
				},
				CertManagerRequests: []CertManager{
					{
						Issuer:      "my-issuer",
						DNSNames:    []string{"custom-domain.my-company.io"},
						IPAddresses: []string{"10.1.1.2"},
					},
					{
						Issuer:      "another-issuer",
						DNSNames:    []string{"www.example.com"},
						IPAddresses: []string{"169.254.254.101"},
					},
					{
						Issuer:      "another-issuer",
						DNSNames:    []string{"web.example.com"},
						IPAddresses: []string{"169.254.254.102"},
					},
				},
			},
		},
	}

	assert.Equal(t, []CertManager{
		{
			Issuer:      "another-issuer",
			DNSNames:    []string{"www.example.com", "web.example.com"},
			IPAddresses: []string{"169.254.254.101", "169.254.254.102"},
		},
		{
			Issuer:          "my-issuer",
			DNSNames:        []string{"default-domain.my-company.io", "custom-domain.my-company.io"},
			IPAddresses:     []string{"10.1.1.1", "10.1.1.2"},
			DNSNamesDefault: true,
		},
	}, instance.CertManagerRequests())
}
