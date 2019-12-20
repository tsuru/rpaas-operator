// Copyright 2019 tsuru authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package rpaas

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCreateArgs_Flavors(t *testing.T) {
	tests := []struct {
		args CreateArgs
		want []string
	}{
		{},
		{
			args: CreateArgs{
				Parameters: map[string]interface{}{
					"flavors": map[string]interface{}{
						"0": "strawberry",
						"1": "blueberry",
					},
				},
			},
			want: []string{"strawberry", "blueberry"},
		},
		{
			args: CreateArgs{
				Tags: []string{"flavor:banana"},
			},
			want: []string{"banana"},
		},
		{
			args: CreateArgs{
				Parameters: map[string]interface{}{
					"flavors": map[string]interface{}{
						"0": "strawberry",
						"1": "blueberry",
					},
				},
				Tags: []string{"flavors=banana"},
			},
			want: []string{"strawberry", "blueberry"},
		},
		{
			args: CreateArgs{
				Tags: []string{"flavor:strawberry,blueberry"},
			},
			want: []string{"strawberry", "blueberry"},
		},
		{
			args: CreateArgs{
				Tags: []string{"flavors:strawberry,blueberry"},
			},
			want: []string{"strawberry", "blueberry"},
		},
		{
			args: CreateArgs{
				Tags: []string{"flavor=strawberry,blueberry"},
			},
			want: []string{"strawberry", "blueberry"},
		},
		{
			args: CreateArgs{
				Tags: []string{"flavors=strawberry,blueberry"},
			},
			want: []string{"strawberry", "blueberry"},
		},
		{
			args: CreateArgs{
				Tags: []string{"flavor:banana", "flavors=strawberry,blueberry"},
			},
			want: []string{"banana"},
		},
	}

	for _, tt := range tests {
		t.Run("", func(t *testing.T) {
			have := tt.args.Flavors()
			assert.Equal(t, tt.want, have)
		})
	}
}

func TestUpdateInstanceArgs_Flavors(t *testing.T) {
	tests := []struct {
		args UpdateInstanceArgs
		want []string
	}{
		{},
		{
			args: UpdateInstanceArgs{
				Parameters: map[string]interface{}{
					"flavors": map[string]interface{}{
						"0": "strawberry",
						"1": "blueberry",
					},
				},
			},
			want: []string{"strawberry", "blueberry"},
		},
		{
			args: UpdateInstanceArgs{
				Tags: []string{"flavor:banana"},
			},
			want: []string{"banana"},
		},
		{
			args: UpdateInstanceArgs{
				Parameters: map[string]interface{}{
					"flavors": map[string]interface{}{
						"0": "strawberry",
						"1": "blueberry",
					},
				},
				Tags: []string{"flavors=banana"},
			},
			want: []string{"strawberry", "blueberry"},
		},
		{
			args: UpdateInstanceArgs{
				Tags: []string{"flavor:strawberry,blueberry"},
			},
			want: []string{"strawberry", "blueberry"},
		},
		{
			args: UpdateInstanceArgs{
				Tags: []string{"flavors:strawberry,blueberry"},
			},
			want: []string{"strawberry", "blueberry"},
		},
		{
			args: UpdateInstanceArgs{
				Tags: []string{"flavor=strawberry,blueberry"},
			},
			want: []string{"strawberry", "blueberry"},
		},
		{
			args: UpdateInstanceArgs{
				Tags: []string{"flavors=strawberry,blueberry"},
			},
			want: []string{"strawberry", "blueberry"},
		},
		{
			args: UpdateInstanceArgs{
				Tags: []string{"flavor:banana", "flavors=strawberry,blueberry"},
			},
			want: []string{"banana"},
		},
	}

	for _, tt := range tests {
		t.Run("", func(t *testing.T) {
			have := tt.args.Flavors()
			assert.Equal(t, tt.want, have)
		})
	}
}
