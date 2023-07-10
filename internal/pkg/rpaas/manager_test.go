// Copyright 2019 tsuru authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package rpaas

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tsuru/rpaas-operator/internal/config"
	"k8s.io/apimachinery/pkg/util/validation"
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
		{
			args: CreateArgs{
				Parameters: map[string]interface{}{"flavors": ""},
			},
		},
	}

	for _, tt := range tests {
		t.Run("", func(t *testing.T) {
			have := tt.args.Flavors()
			assert.Equal(t, tt.want, have)
		})
	}
}

func TestCreateArgs_IP(t *testing.T) {
	tests := []struct {
		args CreateArgs
		want string
	}{
		{},
		{
			args: CreateArgs{
				Parameters: map[string]interface{}{"ip": "7.7.7.7"},
			},
			want: "7.7.7.7",
		},
		{
			args: CreateArgs{
				Parameters: map[string]interface{}{"ip": []string{"not valid"}},
			},
		},
		{
			args: CreateArgs{Tags: []string{"ip:7.7.7.7"}},
			want: "7.7.7.7",
		},
		{
			args: CreateArgs{Tags: []string{"ip=7.7.7.7"}},
			want: "7.7.7.7",
		},
		{
			args: CreateArgs{Tags: []string{"ip:6.6.6.6", "ip=7.7.7.7"}},
			want: "6.6.6.6",
		},
	}

	for _, tt := range tests {
		t.Run("", func(t *testing.T) {
			have := tt.args.IP()
			assert.Equal(t, tt.want, have)
		})
	}
}

func TestCreateArgs_PlanOverride(t *testing.T) {
	tests := []struct {
		args CreateArgs
		want string
	}{
		{},
		{
			args: CreateArgs{
				Parameters: map[string]interface{}{"plan-override": `{"image": "nginx:alpine"}`},
			},
			want: `{"image": "nginx:alpine"}`,
		},
		{
			args: CreateArgs{
				Parameters: map[string]interface{}{"plan-override": []string{"not valid"}},
			},
		},
		{
			args: CreateArgs{Tags: []string{`plan-override:{"config": {"cacheEnabled": false}}`}},
			want: `{"config": {"cacheEnabled": false}}`,
		},
		{
			args: CreateArgs{Tags: []string{`plan-override={"config": {"cacheEnabled": false}}`}},
			want: `{"config": {"cacheEnabled": false}}`,
		},
		{
			args: CreateArgs{Tags: []string{`plan-override={"image": "nginx:alpine"}`, `plan-override:{"config": {"cacheEnabled": false}}`}},
			want: `{"image": "nginx:alpine"}`,
		},
	}

	for _, tt := range tests {
		t.Run("", func(t *testing.T) {
			have := tt.args.PlanOverride()
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
		{
			args: UpdateInstanceArgs{
				Parameters: map[string]interface{}{"flavors": ""},
			},
		},
	}

	for _, tt := range tests {
		t.Run("", func(t *testing.T) {
			have := tt.args.Flavors()
			assert.Equal(t, tt.want, have)
		})
	}
}

func TestUpdateInstanceArgs_IP(t *testing.T) {
	tests := []struct {
		args UpdateInstanceArgs
		want string
	}{
		{},
		{
			args: UpdateInstanceArgs{
				Parameters: map[string]interface{}{
					"ip": "7.7.7.7",
				},
			},
			want: "7.7.7.7",
		},
		{
			args: UpdateInstanceArgs{
				Parameters: map[string]interface{}{
					"ip": []string{"not valid"},
				},
			},
		},
		{
			args: UpdateInstanceArgs{Tags: []string{"ip:7.7.7.7"}},
			want: "7.7.7.7",
		},
		{
			args: UpdateInstanceArgs{Tags: []string{"ip=7.7.7.7"}},
			want: "7.7.7.7",
		},
		{
			args: UpdateInstanceArgs{Tags: []string{"ip:6.6.6.6", "ip=7.7.7.7"}},
			want: "6.6.6.6",
		},
	}

	for _, tt := range tests {
		t.Run("", func(t *testing.T) {
			have := tt.args.IP()
			assert.Equal(t, tt.want, have)
		})
	}
}

func TestUpdateInstanceArgs_PlanOverride(t *testing.T) {
	tests := []struct {
		args UpdateInstanceArgs
		want string
	}{
		{},
		{
			args: UpdateInstanceArgs{
				Parameters: map[string]interface{}{"plan-override": `{"image": "nginx:alpine"}`},
			},
			want: `{"image": "nginx:alpine"}`,
		},
		{
			args: UpdateInstanceArgs{
				Parameters: map[string]interface{}{"plan-override": []string{"not valid"}},
			},
		},
		{
			args: UpdateInstanceArgs{Tags: []string{`plan-override:{"config": {"cacheEnabled": false}}`}},
			want: `{"config": {"cacheEnabled": false}}`,
		},
		{
			args: UpdateInstanceArgs{Tags: []string{`plan-override={"config": {"cacheEnabled": false}}`}},
			want: `{"config": {"cacheEnabled": false}}`,
		},
		{
			args: UpdateInstanceArgs{Tags: []string{`plan-override={"image": "nginx:alpine"}`, `plan-override:{"config": {"cacheEnabled": false}}`}},
			want: `{"image": "nginx:alpine"}`,
		},
	}

	for _, tt := range tests {
		t.Run("", func(t *testing.T) {
			have := tt.args.PlanOverride()
			assert.Equal(t, tt.want, have)
		})
	}
}
func TestUpdateInstanceArgs_Annotations(t *testing.T) {
	tests := []struct {
		args          UpdateInstanceArgs
		want          map[string]string
		expectedError string
	}{
		{},
		{
			args: UpdateInstanceArgs{
				Parameters: map[string]interface{}{"annotations": `{"image": "nginx:alpine", "test.io/valid": "valid"}`},
			},
			want: map[string]string{"image": "nginx:alpine", "test.io/valid": "valid"},
		},
		{
			args: UpdateInstanceArgs{
				Parameters: map[string]interface{}{"annotations": `{"image": "nginx:alpine", "rpaas.extensions.tsuru.io/test": "not valid", "test.io/valid": "valid"}`},
			},
			expectedError: "annotation \"rpaas.extensions.tsuru.io/test\" is not allowed",
		},
		{
			args: UpdateInstanceArgs{
				Parameters: map[string]interface{}{"annotations": `{"invalid_domain.io/test": "test"}`},
			},
			expectedError: fmt.Sprintf("invalid annotation \"invalid_domain.io/test\": %s", validation.IsDNS1123Subdomain("invalid_domain.io/test")),
		},
	}

	for _, tt := range tests {
		t.Run("", func(t *testing.T) {
			err := config.Init()
			require.NoError(t, err)
			have, err := tt.args.Annotations()
			if tt.expectedError != "" {
				assert.EqualError(t, err, tt.expectedError)
				return
			}
			assert.Equal(t, tt.want, have)
		})
	}
}
