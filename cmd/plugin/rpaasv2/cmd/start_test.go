// Copyright 2024 tsuru authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package cmd

import "context"

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/tsuru/rpaas-operator/pkg/rpaas/client/fake"
)

func TestStart(t *testing.T) {
	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}

	args := []string{"./rpaasv2", "start", "-s", "some-service", "-i", "my-instance"}

	client := &fake.FakeClient{
		FakeStart: func(instance string) error {
			require.Equal(t, instance, "my-instance")
			return nil
		},
	}

	app := NewApp(stdout, stderr, client)
	err := app.Run(context.Background(), args)
	require.NoError(t, err)
	assert.Equal(t, stdout.String(), "Started instance some-service/my-instance\n")
}
