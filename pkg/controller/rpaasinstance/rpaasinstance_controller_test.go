package rpaasinstance

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tsuru/rpaas-operator/pkg/apis/extensions/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
)

func Test_mergePlans(t *testing.T) {
	tests := []struct {
		base     v1alpha1.RpaasPlanSpec
		override v1alpha1.RpaasPlanSpec
		expected v1alpha1.RpaasPlanSpec
	}{
		{
			base:     v1alpha1.RpaasPlanSpec{},
			override: v1alpha1.RpaasPlanSpec{},
			expected: v1alpha1.RpaasPlanSpec{},
		},
		{
			base:     v1alpha1.RpaasPlanSpec{Image: "img0", Description: "a", Config: v1alpha1.NginxConfig{User: "root", CacheEnabled: v1alpha1.Bool(true)}},
			override: v1alpha1.RpaasPlanSpec{Image: "img1"},
			expected: v1alpha1.RpaasPlanSpec{Image: "img1", Description: "a", Config: v1alpha1.NginxConfig{User: "root", CacheEnabled: v1alpha1.Bool(true)}},
		},
		{
			base:     v1alpha1.RpaasPlanSpec{Image: "img0", Description: "a", Config: v1alpha1.NginxConfig{User: "root", CacheSize: "10", CacheEnabled: v1alpha1.Bool(true)}},
			override: v1alpha1.RpaasPlanSpec{Image: "img1", Config: v1alpha1.NginxConfig{User: "ubuntu"}},
			expected: v1alpha1.RpaasPlanSpec{Image: "img1", Description: "a", Config: v1alpha1.NginxConfig{User: "ubuntu", CacheSize: "10", CacheEnabled: v1alpha1.Bool(true)}},
		},
		{
			base:     v1alpha1.RpaasPlanSpec{Image: "img0", Description: "a", Config: v1alpha1.NginxConfig{User: "root", CacheSize: "10", CacheEnabled: v1alpha1.Bool(true)}},
			override: v1alpha1.RpaasPlanSpec{Image: "img1", Config: v1alpha1.NginxConfig{User: "ubuntu", CacheEnabled: v1alpha1.Bool(false)}},
			expected: v1alpha1.RpaasPlanSpec{Image: "img1", Description: "a", Config: v1alpha1.NginxConfig{User: "ubuntu", CacheSize: "10", CacheEnabled: v1alpha1.Bool(false)}},
		},

		{
			base:     v1alpha1.RpaasPlanSpec{Image: "img0", Resources: corev1.ResourceRequirements{Limits: corev1.ResourceList{corev1.ResourceCPU: resource.MustParse("100m"), corev1.ResourceMemory: resource.MustParse("100Mi")}}},
			override: v1alpha1.RpaasPlanSpec{Image: "img1", Resources: corev1.ResourceRequirements{Limits: corev1.ResourceList{corev1.ResourceMemory: resource.MustParse("200Mi")}}},
			expected: v1alpha1.RpaasPlanSpec{Image: "img1", Resources: corev1.ResourceRequirements{Limits: corev1.ResourceList{corev1.ResourceCPU: resource.MustParse("100m"), corev1.ResourceMemory: resource.MustParse("200Mi")}}},
		},
	}

	for _, tt := range tests {
		t.Run("", func(t *testing.T) {
			result, err := mergePlans(tt.base, tt.override)
			require.NoError(t, err)
			assert.Equal(t, result, tt.expected)
		})
	}
}
