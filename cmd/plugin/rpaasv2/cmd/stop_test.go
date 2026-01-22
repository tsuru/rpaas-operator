// Copyright 2024 tsuru authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package cmd

import (
	"bytes"
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/tsuru/rpaas-operator/pkg/rpaas/client/fake"
)

func TestStop(t *testing.T) {
	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}

	args := []string{"./rpaasv2", "stop", "-s", "some-service", "-i", "my-instance"}

	client := &fake.FakeClient{
		FakeStop: func(instance string) error {
			require.Equal(t, instance, "my-instance")
			return nil
		},
	}

	app := NewApp(stdout, stderr, client)
	err := app.Run(context.Background(), args)
	require.NoError(t, err)
	assert.Equal(t, stdout.String(), "Shutting down instance some-service/my-instance\n")
}
