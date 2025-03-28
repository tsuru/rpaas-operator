// Copyright 2025 tsuru authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package macro_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/tsuru/rpaas-operator/pkg/macro"
)

func TestParse(t *testing.T) {
	macroExamples := map[string]*macro.MacroExpr{
		"MACRO": {
			Name: "MACRO",
		},
		"MACRO arg1 arg2": {
			Name: "MACRO",
			Arguments: []*macro.MacroArgument{
				{Arg: "arg1"},
				{Arg: "arg2"},
			},
		},
		"MACRO \"argument one\" \"argument two\"": {
			Name: "MACRO",
			Arguments: []*macro.MacroArgument{
				{Arg: "argument one"},
				{Arg: "argument two"},
			},
		},
		"MACRO_EXAMPLE http://arg1/argument": {
			Name: "MACRO_EXAMPLE",
			Arguments: []*macro.MacroArgument{
				{Arg: "http://arg1/argument"},
			},
		},
		"MACRO_EXAMPLE \"http://arg1/argument\"": {
			Name: "MACRO_EXAMPLE",
			Arguments: []*macro.MacroArgument{
				{Arg: "http://arg1/argument"},
			},
		},
		"micro arg1 arg3": nil,
		"MACRO key1=value1 key2=value2": {
			Name: "MACRO",
			Arguments: []*macro.MacroArgument{
				{KV: &macro.MacroKV{Key: "key1", Value: "value1"}},
				{KV: &macro.MacroKV{Key: "key2", Value: "value2"}},
			},
		},
		"MY_MACRO arg1 arg2 key1=value1 key2='string value' key3=\"https://globo.com/hello\"": {
			Name: "MY_MACRO",
			Arguments: []*macro.MacroArgument{
				{Arg: "arg1"},
				{Arg: "arg2"},
				{KV: &macro.MacroKV{Key: "key1", Value: "value1"}},
				{KV: &macro.MacroKV{Key: "key2", Value: "string value"}},
				{KV: &macro.MacroKV{Key: "key3", Value: "https://globo.com/hello"}},
			},
		},
	}

	for macroStr, expectedMacro := range macroExamples {

		t.Run(macroStr, func(t *testing.T) {
			parsed, err := macro.ParseExp(macroStr)

			if expectedMacro == nil {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, expectedMacro, parsed)
		})

	}

}
