package util

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
)

func Test_SHA256(t *testing.T) {
	tests := []struct {
		input    interface{}
		expected string
	}{
		{
			input:    nil,
			expected: "f390331726b48aeb576a4ca62efcca87b097105d22b4e5d92655c5608d334d75",
		},
		{
			input:    struct{ k, v string }{k: "key", v: "value"},
			expected: "f58a6fdd82f53f250083ff35a3b487f9e39582183711266d85ca00e736e6eda9",
		},
		{
			input: map[string]interface{}{
				"key": []byte("value"),
			},
			expected: "adabf12890f7ec4c07469295bb08c5b6633cf7edb522c867cbb440a6eef0c506",
		},
		{
			input: map[string]interface{}{
				"key": "value",
			},
			expected: "2f7cb7d9d9295be5e34f5ac6dd7eee3e5a5c468f6feb97c3824f2231dc85b611",
		},
	}

	for _, tt := range tests {
		t.Run(fmt.Sprintf("when the input object is %v", tt.input), func(t *testing.T) {
			assert.Equal(t, tt.expected, SHA256(tt.input))
		})
	}
}
