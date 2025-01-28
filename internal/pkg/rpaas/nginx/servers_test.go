// Copyright 2025 tsuru authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package nginx

import (
	"testing"

	"github.com/stretchr/testify/assert"
	nginxv1alpha1 "github.com/tsuru/nginx-operator/api/v1alpha1"

	"github.com/tsuru/rpaas-operator/api/v1alpha1"
)

func TestProduceServers(t *testing.T) {
	tests := []struct {
		name     string
		spec     *v1alpha1.RpaasInstanceSpec
		expected []*Server
	}{

		{
			name: "spec with locations without server name",
			spec: &v1alpha1.RpaasInstanceSpec{
				Locations: []v1alpha1.Location{
					{
						Path:        "/",
						Destination: "http://example.com",
					},
					{
						Path:        "/test",
						Destination: "http://test.example.com",
					},
				},
			},
			expected: []*Server{
				{
					Default: true,
					Locations: []v1alpha1.Location{
						{
							Path:        "/",
							Destination: "http://example.com",
						},
						{
							Path:        "/test",
							Destination: "http://test.example.com",
						},
					},
				},
			},
		},
		{
			name: "spec with locations with many server names",
			spec: &v1alpha1.RpaasInstanceSpec{
				Locations: []v1alpha1.Location{
					{
						Path:        "/common",
						Destination: "http://common.com",
					},
					{
						Path:        "/",
						ServerName:  "example.com",
						Destination: "http://example.com",
					},
					{
						Path:        "/",
						ServerName:  "test.example.com",
						Destination: "http://test.example.com",
					},
				},
			},
			expected: []*Server{
				{
					Default: true,
					Locations: []v1alpha1.Location{
						{
							Path:        "/common",
							Destination: "http://common.com",
						},
					},
				},
				{
					Name: "example.com",
					Locations: []v1alpha1.Location{
						{
							Path:        "/common",
							Destination: "http://common.com",
						},
						{
							Path:        "/",
							ServerName:  "example.com",
							Destination: "http://example.com",
						},
					},
				},
				{
					Name: "test.example.com",
					Locations: []v1alpha1.Location{
						{
							Path:        "/common",
							Destination: "http://common.com",
						},
						{
							Path:        "/",
							ServerName:  "test.example.com",
							Destination: "http://test.example.com",
						},
					},
				},
			},
		},
		{
			name: "spec with locations with many server names with override",
			spec: &v1alpha1.RpaasInstanceSpec{
				Locations: []v1alpha1.Location{
					{
						Path:        "/",
						Destination: "http://common.com",
					},
					{
						Path:        "/",
						ServerName:  "example.com",
						Destination: "http://example.com",
					},
				},
			},
			expected: []*Server{
				{
					Default: true,
					Locations: []v1alpha1.Location{
						{
							Path:        "/",
							Destination: "http://common.com",
						},
					},
				},
				{
					Name: "example.com",
					Locations: []v1alpha1.Location{
						{
							Path:        "/",
							ServerName:  "example.com",
							Destination: "http://example.com",
						},
					},
				},
			},
		},

		{
			name: "spec with tls",
			spec: &v1alpha1.RpaasInstanceSpec{
				TLS: []nginxv1alpha1.NginxTLS{
					{
						SecretName: "example.org",
						Hosts:      []string{"example.org"},
					},
				},
			},
			expected: []*Server{
				{
					Default: true,
				},
				{
					TLS:           true,
					TLSSecretName: "example.org",
					Name:          "example.org",
				},
			},
		},

		{
			name: "spec with tls and locations",
			spec: &v1alpha1.RpaasInstanceSpec{
				TLS: []nginxv1alpha1.NginxTLS{
					{
						SecretName: "example.org",
						Hosts:      []string{"example.org"},
					},
				},
				Locations: []v1alpha1.Location{
					{
						Path: "/",
						Content: &v1alpha1.Value{
							Value: "# My custom NGINX config",
						},
					},
				},
			},
			expected: []*Server{
				{
					Default: true,
					Locations: []v1alpha1.Location{
						{
							Path: "/",
							Content: &v1alpha1.Value{
								Value: "# My custom NGINX config",
							},
						},
					},
				},
				{
					TLS:           true,
					TLSSecretName: "example.org",
					Name:          "example.org",
					Locations: []v1alpha1.Location{
						{
							Path: "/",
							Content: &v1alpha1.Value{
								Value: "# My custom NGINX config",
							},
						},
					},
				},
			},
		},

		{
			name: "spec with tls with wildcard",
			spec: &v1alpha1.RpaasInstanceSpec{
				TLS: []nginxv1alpha1.NginxTLS{
					{
						SecretName: "wild-card-example.org",
						Hosts:      []string{"*.example.org"},
					},
				},
				Locations: []v1alpha1.Location{
					{
						Path: "/",
						Content: &v1alpha1.Value{
							Value: "# My custom NGINX config",
						},
					},
				},
			},
			expected: []*Server{
				{
					Default: true,
					Locations: []v1alpha1.Location{
						{
							Path: "/",
							Content: &v1alpha1.Value{
								Value: "# My custom NGINX config",
							},
						},
					},
				},
				{
					TLS:           true,
					TLSSecretName: "wild-card-example.org",
					Name:          "*.example.org",
					Wildcard:      true,
					Locations: []v1alpha1.Location{
						{
							Path: "/",
							Content: &v1alpha1.Value{
								Value: "# My custom NGINX config",
							},
						},
					},
				},
			},
		},
		{
			name: "spec with tls with wildcard ad specific blocks",
			spec: &v1alpha1.RpaasInstanceSpec{
				TLS: []nginxv1alpha1.NginxTLS{
					{
						SecretName: "wild-card-example.org",
						Hosts:      []string{"*.example.org"},
					},
				},
				Locations: []v1alpha1.Location{
					{
						Path: "/",
						Content: &v1alpha1.Value{
							Value: "# My custom NGINX config",
						},
					},

					{
						Path:       "/info",
						ServerName: "info.example.org",
						Content: &v1alpha1.Value{
							Value: "# My custom NGINX config",
						},
					},
				},
			},
			expected: []*Server{
				{
					Default: true,
					Locations: []v1alpha1.Location{
						{
							Path: "/",
							Content: &v1alpha1.Value{
								Value: "# My custom NGINX config",
							},
						},
					},
				},
				{
					TLS:           true,
					TLSSecretName: "wild-card-example.org",
					Name:          "info.example.org",
					Locations: []v1alpha1.Location{
						{
							Path: "/",
							Content: &v1alpha1.Value{
								Value: "# My custom NGINX config",
							},
						},

						{
							Path:       "/info",
							ServerName: "info.example.org",
							Content: &v1alpha1.Value{
								Value: "# My custom NGINX config",
							},
						},
					},
				},
				{
					Name:          "*.example.org",
					TLS:           true,
					TLSSecretName: "wild-card-example.org",
					Wildcard:      true,
					Locations: []v1alpha1.Location{
						{
							Path: "/",
							Content: &v1alpha1.Value{
								Value: "# My custom NGINX config",
							},
						},
					},
				},
			},
		},

		{
			name: "spec with locations with many server names",
			spec: &v1alpha1.RpaasInstanceSpec{
				Locations: []v1alpha1.Location{
					{
						Path:        "/common",
						Destination: "http://common.com",
					},
					{
						Path:        "/",
						ServerName:  "example.com",
						Destination: "http://example.com",
					},
					{
						Path:        "/",
						ServerName:  "test.example.com",
						Destination: "http://test.example.com",
					},
				},
			},
			expected: []*Server{
				{
					Default: true,
					Locations: []v1alpha1.Location{
						{
							Path:        "/common",
							Destination: "http://common.com",
						},
					},
				},
				{
					Name: "example.com",
					Locations: []v1alpha1.Location{
						{
							Path:        "/common",
							Destination: "http://common.com",
						},
						{
							Path:        "/",
							ServerName:  "example.com",
							Destination: "http://example.com",
						},
					},
				},
				{
					Name: "test.example.com",
					Locations: []v1alpha1.Location{
						{
							Path:        "/common",
							Destination: "http://common.com",
						},
						{
							Path:        "/",
							ServerName:  "test.example.com",
							Destination: "http://test.example.com",
						},
					},
				},
			},
		},

		// Multi Block support
		{
			name: "spec with blocks",
			spec: &v1alpha1.RpaasInstanceSpec{
				Blocks: map[v1alpha1.BlockType]v1alpha1.Value{
					"server": {
						Value: "proxy_set_header MyHeader MyValue;",
					},
				},
			},
			expected: []*Server{
				{
					Default: true,
					Blocks: map[v1alpha1.BlockType]v1alpha1.Value{
						"server": {
							Value: "proxy_set_header MyHeader MyValue;",
						},
					},
				},
			},
		},

		{
			name: "spec with blocks with override and extend",
			spec: &v1alpha1.RpaasInstanceSpec{
				Blocks: map[v1alpha1.BlockType]v1alpha1.Value{
					"server": {
						Value: "proxy_set_header MyHeader MyValue;",
					},
				},
				ServerBlocks: []v1alpha1.ServerBlock{
					{
						ServerName: "example.com",
						Type:       "server",
						Content: &v1alpha1.Value{
							Value: "proxy_set_header MyHeader example.com;",
						},
					},
					{
						ServerName: "example.net",
						Type:       "server",
						Content: &v1alpha1.Value{
							Value: "proxy_set_header MyHeader example.net;",
						},
						Extend: true,
					},
				},
			},
			expected: []*Server{
				{
					Default: true,
					Blocks: map[v1alpha1.BlockType]v1alpha1.Value{
						"server": {
							Value: "proxy_set_header MyHeader MyValue;",
						},
					},
				},
				{
					Name: "example.com",
					Blocks: map[v1alpha1.BlockType]v1alpha1.Value{
						"server": {
							Value: "proxy_set_header MyHeader example.com;",
						},
					},
				},
				{
					Name: "example.net",

					Blocks: map[v1alpha1.BlockType]v1alpha1.Value{
						"server": {
							Value: "proxy_set_header MyHeader MyValue;\nproxy_set_header MyHeader example.net;",
						},
					},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ProduceServers(tt.spec)
			assert.Equal(t, tt.expected, result)
		})
	}
}
