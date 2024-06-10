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
			if instance == "my-instance" {
				return &types.Metadata{
					Labels: []types.MetadataItem{
						{Name: "label1", Value: "value1"},
					},
					Annotations: []types.MetadataItem{
						{Name: "annotation1", Value: "value1"},
						{Name: "annotation2", Value: "value2"},
					},
				}, nil
			} else if instance == "empty-instance" {
				return &types.Metadata{
					Labels:      []types.MetadataItem{},
					Annotations: []types.MetadataItem{},
				}, nil
			} else {
				return nil, errors.New("could not find instance")
			}
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
		{
			name:     "get metadata with no content",
			instance: "empty-instance",
			expected: "No metadata found\n",
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

func TestSetMetadata(t *testing.T) {
	baseArgs := []string{"./rpaasv2", "metadata", "set", "-s", "my-service"}
	testCases := []struct {
		name        string
		args        []string
		expectedErr string
	}{
		{
			name:        "instance not found",
			args:        []string{"-i", "invalid-instance", "-t", "label", "key1=value1"},
			expectedErr: "could not find instance",
		},
		{
			name:        "no key-values provided",
			args:        []string{"-i", "my-instance", "-t", "label"},
			expectedErr: "at least one NAME=value pair is required",
		},
		{
			name:        "invalid metadata type",
			args:        []string{"-i", "my-instance", "-t", "invalid", "key=value"},
			expectedErr: "invalid metadata type: \"invalid\"",
		},
		{
			name:        "invalid key value pair",
			args:        []string{"-i", "my-instance", "-t", "annotation", "key"},
			expectedErr: "invalid NAME=value pair: \"key\"",
		},
		{
			name: "valid metadata",
			args: []string{"-i", "my-instance", "-t", "label", "key1=value1", "key2=value2"},
		},
	}

	client := &fake.FakeClient{
		FakeSetMetadata: func(instance string, metadata *types.Metadata) error {
			if instance != "my-instance" {
				return errors.New("could not find instance")
			}
			return nil
		},
	}

	for _, tt := range testCases {
		t.Run(tt.name, func(t *testing.T) {
			stdout := &bytes.Buffer{}
			stderr := &bytes.Buffer{}

			args := append(baseArgs, tt.args...)

			app := NewApp(stdout, stderr, client)
			err := app.Run(args)

			if tt.expectedErr != "" {
				assert.EqualError(t, err, tt.expectedErr)
				return
			}

			assert.NoError(t, err)
		})
	}
}

func TestUnsetMetadata(t *testing.T) {
	baseArgs := []string{"./rpaasv2", "metadata", "unset", "-s", "my-service"}
	testCases := []struct {
		name        string
		args        []string
		expectedErr string
	}{
		{
			name:        "instance not found",
			args:        []string{"-i", "invalid-instance", "-t", "label", "key1"},
			expectedErr: "could not find instance",
		},
		{
			name:        "no key-values provided",
			args:        []string{"-i", "my-instance", "-t", "label"},
			expectedErr: "at least one NAME is required",
		},
		{
			name:        "invalid metadata type",
			args:        []string{"-i", "my-instance", "-t", "invalid", "key=value"},
			expectedErr: "invalid metadata type: \"invalid\"",
		},
		{
			name: "valid metadata",
			args: []string{"-i", "my-instance", "-t", "label", "key1", "key2"},
		},
	}

	client := &fake.FakeClient{
		FakeUnsetMetadata: func(instance string, metadata *types.Metadata) error {
			if instance != "my-instance" {
				return errors.New("could not find instance")
			}
			return nil
		},
	}

	for _, tt := range testCases {
		t.Run(tt.name, func(t *testing.T) {
			stdout := &bytes.Buffer{}
			stderr := &bytes.Buffer{}

			args := append(baseArgs, tt.args...)

			app := NewApp(stdout, stderr, client)
			err := app.Run(args)

			if tt.expectedErr != "" {
				assert.EqualError(t, err, tt.expectedErr)
				return
			}

			assert.NoError(t, err)
		})
	}
}
