// Copyright 2019 tsuru authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package config

import (
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"

	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tsuru/rpaas-operator/pkg/apis/extensions/v1alpha1"
	corev1 "k8s.io/api/core/v1"
)

func Test_Init(t *testing.T) {
	tests := []struct {
		config   string
		envs     map[string]string
		expected RpaasConfig
	}{
		{
			expected: RpaasConfig{
				ServiceName: "rpaasv2",
			},
		},
		{
			config: `
tls-certificate: /var/share/tls/mycert.pem
tls-key: /var/share/tls/key.pem
`,
			expected: RpaasConfig{
				ServiceName:    "rpaasv2",
				TLSCertificate: "/var/share/tls/mycert.pem",
				TLSKey:         "/var/share/tls/key.pem",
			},
		},
		{
			config: `
api-username: u1
service-annotations:
  a: b
  c: d
flavors:
- name: tangerine
  spec:
    image: img1
- name: mango
  spec:
    config:
      cacheEnabled: false
`,
			expected: RpaasConfig{
				APIUsername: "u1",
				ServiceName: "rpaasv2",
				ServiceAnnotations: map[string]string{
					"a": "b",
					"c": "d",
				},
				Flavors: []FlavorConfig{
					{
						Name: "tangerine",
						Spec: v1alpha1.RpaasPlanSpec{
							Image: "img1",
						},
					},
					{
						Name: "mango",
						Spec: v1alpha1.RpaasPlanSpec{
							Config: v1alpha1.NginxConfig{
								CacheEnabled: v1alpha1.Bool(false),
							},
						},
					},
				},
			},
		},
		{
			config: `
api-username: ignored1
service-annotations:
  ig: nored
service-name: rpaasv2be
flavors:
- name: strawberry
`,
			envs: map[string]string{
				"RPAASV2_API_USERNAME":        "u1",
				"RPAASV2_API_PASSWORD":        "p1",
				"RPAASV2_SERVICE_ANNOTATIONS": `{"x": "y"}`,
			},
			expected: RpaasConfig{
				APIUsername: "u1",
				APIPassword: "p1",
				ServiceName: "rpaasv2be",
				ServiceAnnotations: map[string]string{
					"x": "y",
				},
				Flavors: []FlavorConfig{
					{
						Name: "strawberry",
					},
				},
			},
		},
		{
			config: `
default-affinity:
  nodeAffinity:
    requiredDuringSchedulingIgnoredDuringExecution:
      nodeSelectorTerms:
      - matchExpressions:
        - key: pool
          operator: In
          values:
          - dev
team-affinity:
  team1:
    nodeAffinity:
      requiredDuringSchedulingIgnoredDuringExecution:
        nodeSelectorTerms:
        - matchExpressions:
          - key: pool
            operator: NotIn
            values:
            - dev
`,
			expected: RpaasConfig{
				ServiceName: "rpaasv2",
				DefaultAffinity: &corev1.Affinity{
					NodeAffinity: &corev1.NodeAffinity{
						RequiredDuringSchedulingIgnoredDuringExecution: &corev1.NodeSelector{
							NodeSelectorTerms: []corev1.NodeSelectorTerm{
								{
									MatchExpressions: []corev1.NodeSelectorRequirement{
										{
											Key:      "pool",
											Operator: corev1.NodeSelectorOpIn,
											Values:   []string{"dev"},
										},
									},
								},
							},
						},
					},
				},
				TeamAffinity: map[string]corev1.Affinity{
					"team1": {
						NodeAffinity: &corev1.NodeAffinity{
							RequiredDuringSchedulingIgnoredDuringExecution: &corev1.NodeSelector{
								NodeSelectorTerms: []corev1.NodeSelectorTerm{
									{
										MatchExpressions: []corev1.NodeSelectorRequirement{
											{
												Key:      "pool",
												Operator: corev1.NodeSelectorOpNotIn,
												Values:   []string{"dev"},
											},
										},
									},
								},
							},
						},
					},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run("", func(t *testing.T) {
			defer viper.Reset()
			for k, v := range tt.envs {
				os.Setenv(k, v)
				defer os.Unsetenv(k)
			}
			dir, err := ioutil.TempDir("", "")
			require.NoError(t, err)
			name := filepath.Join(dir, "config.yaml")
			err = ioutil.WriteFile(name, []byte(tt.config), 0644)
			require.NoError(t, err)
			defer os.RemoveAll(dir)
			os.Args = []string{"test", "--config", name}
			err = Init()
			require.NoError(t, err)
			config := Get()
			assert.Equal(t, tt.expected, config)
		})
	}
}
