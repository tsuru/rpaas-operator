// Copyright 2024 tsuru authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package cmd

import (
	"bytes"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/tsuru/rpaas-operator/pkg/rpaas/client/fake"
	"github.com/tsuru/rpaas-operator/pkg/rpaas/client/types"
)

func TestGetMetadata(t *testing.T) {
	baseArgs := []string{"./rpaasv2", "metadata", "get", "-s", "my-service"}

	client := &fake.FakeClient{
		FakeGetMetadata: func(instance string) (*types.Metadata, error) {
			if instance != "my-instance" {
				return nil, errors.New("could not find instance")
			}
			return &types.Metadata{
				Labels: []types.MetadataItem{
					{Name: "label1", Value: "value1"},
				},
				Annotations: []types.MetadataItem{
					{Name: "annotation1", Value: "value1"},
					{Name: "annotation2", Value: "value2"},
				},
			}, nil
		},
	}

	testCases := []struct {
		name        string
		instance    string
		expected    string
		expectedErr string
	}{
		{
			name:     "get metadata",
			instance: "my-instance",
			expected: `Labels:
  label1: value1
Annotations:
  annotation1: value1
  annotation2: value2
`,
		},
		{
			name:        "get metadata with invalid instance",
			instance:    "invalid-instance",
			expectedErr: "could not find instance",
		},
	}

	for _, tt := range testCases {
		t.Run(tt.name, func(t *testing.T) {
			stdout := &bytes.Buffer{}
			stderr := &bytes.Buffer{}

			args := append(baseArgs, "-i", tt.instance)

			app := NewApp(stdout, stderr, client)
			err := app.Run(args)

			if tt.expectedErr != "" {
				assert.EqualError(t, err, tt.expectedErr)
				return
			}

			assert.NoError(t, err)
			assert.Equal(t, tt.expected, stdout.String())
		})
	}
}
