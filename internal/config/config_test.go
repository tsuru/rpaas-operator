// Copyright 2019 tsuru authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package config

import (
	"os"
	"path/filepath"
	"regexp"
	"testing"
	"time"

	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
)

func Test_Init(t *testing.T) {
	tests := []struct {
		config   string
		envs     map[string]string
		expected func(c RpaasConfig) RpaasConfig
	}{
		{},
		{
			config: `
new-instance-replicas: 5
`,
			expected: func(c RpaasConfig) RpaasConfig {
				c.NewInstanceReplicas = 5
				return c
			},
		},
		{
			config: `
sync-interval: 2m
`,
			expected: func(c RpaasConfig) RpaasConfig {
				c.SyncInterval = 2 * time.Minute
				return c
			},
		},
		{
			config: `
tls-certificate: /var/share/tls/mycert.pem
tls-key: /var/share/tls/key.pem
`,
			expected: func(c RpaasConfig) RpaasConfig {
				c.TLSCertificate = "/var/share/tls/mycert.pem"
				c.TLSKey = "/var/share/tls/key.pem"
				return c
			},
		},
		{
			config: `
api-username: u1
`,
			expected: func(c RpaasConfig) RpaasConfig {
				c.APIUsername = "u1"
				return c
			},
		},
		{
			config: `
api-username: ignored1
service-name: rpaasv2be
`,
			envs: map[string]string{
				"RPAASV2_API_USERNAME": "u1",
				"RPAASV2_API_PASSWORD": "p1",
			},
			expected: func(c RpaasConfig) RpaasConfig {
				c.APIUsername = "u1"
				c.APIPassword = "p1"
				c.ServiceName = "rpaasv2be"
				return c
			},
		},
		{
			config: `
service-name: ignored-service-name
`,
			envs: map[string]string{
				"RPAASV2_SERVICE_NAME": "my-custom-service-name",
			},
			expected: func(c RpaasConfig) RpaasConfig {
				c.ServiceName = "my-custom-service-name"
				return c
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
			expected: func(c RpaasConfig) RpaasConfig {
				c.DefaultAffinity = &corev1.Affinity{
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
				}
				c.TeamAffinity = map[string]corev1.Affinity{
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
				}
				return c
			},
		},
		{
			config: `
loadbalancer-name-label-key: my.cloudprovider.example.com/lb-name
`,
			expected: func(c RpaasConfig) RpaasConfig {
				c.LoadBalancerNameLabelKey = "my.cloudprovider.example.com/lb-name"
				return c
			},
		},
		{
			config: `
websocket-handshake-timeout: 1m
websocket-read-buffer-size: 8192
websocket-write-buffer-size: 8192
websocket-ping-interval: 500ms
websocket-max-idle-time: 5s
websocket-write-wait: 5s
websocket-allowed-origins:
- rpaasv2.example.com
- rpaasv2.test
config-deny-patterns:
- pattern1.*
- pattern2.*
forbidden-annotations-prefixes:
- foo.bar/test
- foo.bar/another
`,
			expected: func(c RpaasConfig) RpaasConfig {
				c.WebSocketHandshakeTimeout = time.Minute
				c.WebSocketReadBufferSize = 8192
				c.WebSocketWriteBufferSize = 8192
				c.WebSocketPingInterval = 500 * time.Millisecond
				c.WebSocketMaxIdleTime = 5 * time.Second
				c.WebSocketWriteWait = 5 * time.Second
				c.WebSocketAllowedOrigins = []string{"rpaasv2.example.com", "rpaasv2.test"}
				c.ConfigDenyPatterns = []regexp.Regexp{
					*regexp.MustCompile(`pattern1.*`),
					*regexp.MustCompile(`pattern2.*`),
				}
				c.ForbiddenAnnotationsPrefixes = []string{"foo.bar/test", "foo.bar/another"}
				return c
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
			dir, err := os.MkdirTemp("", "")
			require.NoError(t, err)
			name := filepath.Join(dir, "config.yaml")
			err = os.WriteFile(name, []byte(tt.config), 0644)
			require.NoError(t, err)
			defer os.RemoveAll(dir)
			os.Args = []string{"test", "--config", name}
			err = Init()
			require.NoError(t, err)
			config := Get()
			expected := RpaasConfig{
				ServiceName:                  "rpaasv2",
				SyncInterval:                 5 * time.Minute,
				WebSocketHandshakeTimeout:    5 * time.Second,
				WebSocketReadBufferSize:      1024,
				WebSocketWriteBufferSize:     4096,
				WebSocketPingInterval:        2 * time.Second,
				WebSocketMaxIdleTime:         1 * time.Minute,
				WebSocketWriteWait:           time.Second,
				NewInstanceReplicas:          1,
				ForbiddenAnnotationsPrefixes: []string{"rpaas.extensions.tsuru.io", "afh.tsuru.io"},
			}
			if tt.expected != nil {
				expected = tt.expected(expected)
			}
			assert.Equal(t, expected, config)
		})
	}
}
