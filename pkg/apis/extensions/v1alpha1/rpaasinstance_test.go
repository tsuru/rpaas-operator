// Copyright 2020 tsuru authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package v1alpha1

import (
	"testing"

	"github.com/stretchr/testify/assert"
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
