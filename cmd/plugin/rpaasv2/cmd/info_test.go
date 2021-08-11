// Copyright 2020 tsuru authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package cmd

import (
	"bytes"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/tsuru/rpaas-operator/api/v1alpha1"
	"github.com/tsuru/rpaas-operator/pkg/rpaas/client"
	"github.com/tsuru/rpaas-operator/pkg/rpaas/client/fake"
	"github.com/tsuru/rpaas-operator/pkg/rpaas/client/types"
	clientTypes "github.com/tsuru/rpaas-operator/pkg/rpaas/client/types"
)

func int32Ptr(n int32) *int32 {
	return &n
}

func TestInfo(t *testing.T) {
	tests := []struct {
		name          string
		args          []string
		expected      string
		expectedError string
		client        client.Client
	}{
		{
			name:          "when info route does not find the instance",
			args:          []string{"./rpaasv2", "info", "-s", "my-service", "-i", "my-instance"},
			expectedError: "not found error",
			client: &fake.FakeClient{
				FakeInfo: func(args client.InfoArgs) (*clientTypes.InstanceInfo, error) {
					require.Equal(t, args.Instance, "my-instance")
					return nil, fmt.Errorf("not found error")
				},
			},
		},
		{
			name: "when info route is successful",
			args: []string{"./rpaasv2", "info", "-s", "my-service", "-i", "my-instance"},
			client: &fake.FakeClient{
				FakeInfo: func(args client.InfoArgs) (*clientTypes.InstanceInfo, error) {
					require.Equal(t, args.Instance, "my-instance")
					return &clientTypes.InstanceInfo{
						Name:      "my-instance",
						Dashboard: "https://grafana.mycompany.io/my-instance",
						Addresses: []clientTypes.InstanceAddress{
							{
								Type:     clientTypes.InstanceAddressTypeClusterExternal,
								Hostname: "some-host",
								IP:       "0.0.0.0",
							},
							{
								Type:     clientTypes.InstanceAddressTypeClusterExternal,
								Hostname: "some-host2",
								IP:       "0.0.0.1",
							},
						},
						Plan: "basic",
						Binds: []v1alpha1.Bind{
							{
								Name: "some-name",
								Host: "some-host",
							},
							{
								Name: "some-name2",
								Host: "some-host2",
							},
						},
						Replicas: int32Ptr(3),
						Blocks: []clientTypes.Block{
							{
								Name:    "http",
								Content: "# some nginx config at http context",
							},
							{
								Name:    "server",
								Content: "# some nginx config at server context",
							},
						},
						Routes: []types.Route{
							{
								Path:        "/app1",
								Destination: "app1.tsuru.example.com",
							},
							{
								Path:        "/login/provider1",
								Destination: "app2.tsuru.example.com",
								HTTPSOnly:   true,
							},
							{
								Path:    "/app3/",
								Content: "# some raw nginx config",
							},
						},
						Team:        "some-team",
						Cluster:     "my-dedicated-cluster",
						Description: "some description",
						Tags:        []string{"tag1", "tag2", "tag3"},
						Flavors:     []string{"flavor1", "flavor2", "flavor-N"},
						Autoscale: &clientTypes.Autoscale{
							MaxReplicas: int32Ptr(5),
							MinReplicas: int32Ptr(2),
							CPU:         int32Ptr(55),
							Memory:      int32Ptr(77),
						},
						Pods: []clientTypes.Pod{
							{
								Name:      "my-instance-75c8bdc6b9-abcde",
								IP:        "169.254.1.100",
								HostIP:    "169.254.1.100",
								Restarts:  int32(2),
								Ready:     true,
								Status:    "Running",
								CreatedAt: time.Now().In(time.UTC).Add(-12 * time.Hour),
								Ports: []clientTypes.PodPort{
									{
										Name:          "http",
										HostPort:      int32(30000),
										ContainerPort: int32(30000),
									},
									{
										Name:          "https",
										HostPort:      int32(30001),
										ContainerPort: int32(30001),
									},
									{
										Name:          "nginx-metrics",
										HostPort:      int32(30002),
										ContainerPort: int32(30002),
									},
								},
							},
							{
								Name:      "my-instance-75c8bdc6b9-bcdef",
								IP:        "169.254.1.101",
								HostIP:    "169.254.1.101",
								Ready:     false,
								Status:    "Running",
								CreatedAt: time.Now().In(time.UTC).Add(-12 * time.Hour),
								Ports: []clientTypes.PodPort{
									{
										Name:          "http",
										HostPort:      int32(30000),
										ContainerPort: int32(30000),
									},
									{
										Name:          "https",
										HostPort:      int32(30001),
										ContainerPort: int32(30001),
									},
									{
										Name:          "nginx-metrics",
										HostPort:      int32(30002),
										ContainerPort: int32(30002),
									},
								},
							},
							{
								Name:      "my-instance-75c8bdc6b9-cdefg",
								IP:        "169.254.1.102",
								HostIP:    "169.254.1.102",
								Ready:     true,
								Status:    "Running",
								CreatedAt: time.Now().In(time.UTC).Add(-12 * time.Hour),
								Ports: []clientTypes.PodPort{
									{
										Name:          "http",
										HostPort:      int32(30000),
										ContainerPort: int32(30000),
									},
									{
										Name:          "https",
										HostPort:      int32(30001),
										ContainerPort: int32(30001),
									},
									{
										Name:          "nginx-metrics",
										HostPort:      int32(30002),
										ContainerPort: int32(30002),
									},
								},
							},
							{
								Name:      "my-instance-123abc456f-aaaaa",
								IP:        "169.254.10.10",
								HostIP:    "169.254.10.10",
								Ready:     false,
								Status:    "Errored",
								Restarts:  int32(100),
								CreatedAt: time.Now().In(time.UTC).Add(-5 * time.Minute),
								Ports: []clientTypes.PodPort{
									{
										Name:          "http",
										HostPort:      int32(30000),
										ContainerPort: int32(30000),
									},
									{
										Name:          "https",
										HostPort:      int32(30001),
										ContainerPort: int32(30001),
									},
									{
										Name:          "nginx-metrics",
										HostPort:      int32(30002),
										ContainerPort: int32(30002),
									},
								},
								Errors: []clientTypes.PodError{
									{
										First:   time.Now().Add(-50 * time.Minute).UTC(),
										Last:    time.Now().Add(-30 * time.Minute).UTC(),
										Count:   int32(20),
										Message: "Back-off 5m0s restarting failed container=nginx pod=my-instance-123abc456f-aaaaa_default(pod uuid)",
									},
									{
										First:   time.Now().Add(-50 * time.Minute).UTC(),
										Last:    time.Now().Add(-50 * time.Minute).UTC(),
										Message: "Exec lifecycle hook ([/bin/sh -c nginx -t && touch /tmp/done]) for Container \"nginx\" in Pod \"my-instance-123abc456f-aaaaa_default(pod uuid)\" failed - error: command '/bin/sh -c nginx -t && touch /tmp/done' exited with 1: 2020/04/07 16:54:18 [emerg] 18#18: \"location\" directive is not allowed here in /etc/nginx/nginx.conf:118\nnginx: [emerg] \"location\" directive is not allowed here in /etc/nginx/nginx.conf:118\nnginx: configuration file /etc/nginx/nginx.conf test failed\n, message: \"2020/04/07 16:54:18 [emerg] 18#18: \\\"location\\\" directive is not allowed here in /etc/nginx/nginx.conf:118\\nnginx: [emerg] \\\"location\\\" directive is not allowed here in /etc/nginx/nginx.conf:118\\nnginx: configuration file /etc/nginx/nginx.conf test failed\\n\"",
									},
								},
							},
							{
								Name:      "my-instance-123abc456f-bbbbb",
								IP:        "169.254.10.11",
								HostIP:    "169.254.10.11",
								Ready:     false,
								Status:    "Errored",
								Restarts:  int32(100),
								CreatedAt: time.Now().In(time.UTC).Add(-5 * time.Minute),
								Ports: []clientTypes.PodPort{
									{
										Name:          "http",
										HostPort:      int32(30000),
										ContainerPort: int32(30000),
									},
									{
										Name:          "https",
										HostPort:      int32(30001),
										ContainerPort: int32(30001),
									},
									{
										Name:          "nginx-metrics",
										HostPort:      int32(30002),
										ContainerPort: int32(30002),
									},
								},
								Errors: []clientTypes.PodError{
									{
										First:   time.Now().Add(-50 * time.Minute).UTC(),
										Last:    time.Now().Add(-30 * time.Minute).UTC(),
										Count:   int32(20),
										Message: "Back-off 5m0s restarting failed container=nginx pod=my-instance-123abc456f-bbbbb_default(pod uuid)",
									},
								},
							},
						},
						Certificates: []clientTypes.CertificateInfo{
							{
								Name:               "default",
								DNSNames:           []string{"my-instance.test", "my-instance.example.com", ".my-instance.example.com", "*.my-instance.example.com"},
								ValidFrom:          time.Date(2020, time.August, 11, 19, 0, 0, 0, time.UTC),
								ValidUntil:         time.Date(2020, time.August, 11, 19, 0, 0, 0, time.UTC),
								PublicKeyAlgorithm: "RSA",
								PublicKeyBitSize:   4096,
							},
							{
								Name:               "default.ecdsa",
								DNSNames:           []string{"another-domain.example.com"},
								ValidFrom:          time.Date(2000, time.August, 00, 00, 0, 0, 0, time.UTC),
								ValidUntil:         time.Date(2050, time.August, 00, 00, 0, 0, 0, time.UTC),
								PublicKeyAlgorithm: "ECDSA",
								PublicKeyBitSize:   384,
							},
						},
					}, nil
				},
			},
			expected: `Name: my-instance
Dashboard: https://grafana.mycompany.io/my-instance
Description: some description
Tags: tag1, tag2, tag3
Team owner: some-team
Plan: basic
Flavors: flavor1, flavor2, flavor-N
Cluster: my-dedicated-cluster

Pods: 3
+------------------------------+---------------+---------+----------+-----+
| Name                         | Host          | Status  | Restarts | Age |
+------------------------------+---------------+---------+----------+-----+
| my-instance-75c8bdc6b9-abcde | 169.254.1.100 | Ready   |        2 | 12h |
| my-instance-75c8bdc6b9-bcdef | 169.254.1.101 | Running |        0 | 12h |
| my-instance-75c8bdc6b9-cdefg | 169.254.1.102 | Ready   |        0 | 12h |
| my-instance-123abc456f-aaaaa | 169.254.10.10 | Errored |      100 | 5m  |
| my-instance-123abc456f-bbbbb | 169.254.10.11 | Errored |      100 | 5m  |
+------------------------------+---------------+---------+----------+-----+

Ports:
+---------------+----------------+----------+-----------+
| Name          | Container port | Protocol | Host port |
+---------------+----------------+----------+-----------+
| http          |          30000 | TCP      |     30000 |
| https         |          30001 | TCP      |     30001 |
| nginx-metrics |          30002 | TCP      |     30002 |
+---------------+----------------+----------+-----------+

Errors:
+--------------------+------------------------------+----------------------------------------------+
| Age                | Pod                          | Message                                      |
+--------------------+------------------------------+----------------------------------------------+
| 30m (x20 over 50m) | my-instance-123abc456f-aaaaa | Back-off 5m0s restarting                     |
|                    |                              | failed container=nginx                       |
|                    |                              | pod=my-instance-123abc456f-aaaaa_default(pod |
|                    |                              | uuid)                                        |
| 50m                | my-instance-123abc456f-aaaaa | Exec lifecycle hook ([/bin/sh                |
|                    |                              | -c nginx -t && touch /tmp/done])             |
|                    |                              | for Container "nginx" in Pod                 |
|                    |                              | "my-instance-123abc456f-aaaaa_default(pod    |
|                    |                              | uuid)" failed - error: command               |
|                    |                              | '/bin/sh -c nginx -t && touch                |
|                    |                              | /tmp/done' exited with 1: 2020/04/07         |
|                    |                              | 16:54:18 [emerg] 18#18: "location"           |
|                    |                              | directive is not allowed here in             |
|                    |                              | /etc/nginx/nginx.conf:118 nginx: [emerg]     |
|                    |                              | "location" directive is not allowed          |
|                    |                              | here in /etc/nginx/nginx.conf:118 nginx:     |
|                    |                              | configuration file /etc/nginx/nginx.conf     |
|                    |                              | test failed , message: "2020/04/07           |
|                    |                              | 16:54:18 [emerg] 18#18: \"location\"         |
|                    |                              | directive is not allowed here in             |
|                    |                              | /etc/nginx/nginx.conf:118\nnginx: [emerg]    |
|                    |                              | \"location\" directive is not allowed        |
|                    |                              | here in /etc/nginx/nginx.conf:118\nnginx:    |
|                    |                              | configuration file /etc/nginx/nginx.conf     |
|                    |                              | test failed\n"                               |
| 30m (x20 over 50m) | my-instance-123abc456f-bbbbb | Back-off 5m0s restarting                     |
|                    |                              | failed container=nginx                       |
|                    |                              | pod=my-instance-123abc456f-bbbbb_default(pod |
|                    |                              | uuid)                                        |
+--------------------+------------------------------+----------------------------------------------+

Autoscale:
+----------+--------------------+
| Replicas | Target Utilization |
+----------+--------------------+
| Max: 5   | CPU: 55%           |
| Min: 2   | Memory: 77%        |
+----------+--------------------+

Binds:
+------------+------------+
| App        | Address    |
+------------+------------+
| some-name  | some-host  |
+------------+------------+
| some-name2 | some-host2 |
+------------+------------+

Addresses:
+------------+---------+--------+
| Hostname   | IP      | Status |
+------------+---------+--------+
| some-host  | 0.0.0.0 |        |
+------------+---------+--------+
| some-host2 | 0.0.0.1 |        |
+------------+---------+--------+

Certificates:
+---------------+--------------------+----------------------+----------------------------+
| Name          | Public Key Info    | Validity             | DNS names                  |
+---------------+--------------------+----------------------+----------------------------+
| default       |     Algorithm      |      Not before      |      my-instance.test      |
|               |        RSA         | 2020-08-11T19:00:00Z |  my-instance.example.com   |
|               |                    |                      |  .my-instance.example.com  |
|               | Key size (in bits) |      Not after       | *.my-instance.example.com  |
|               |        4096        | 2020-08-11T19:00:00Z |                            |
+---------------+--------------------+----------------------+----------------------------+
| default.ecdsa |     Algorithm      |      Not before      | another-domain.example.com |
|               |       ECDSA        | 2000-07-31T00:00:00Z |                            |
|               |                    |                      |                            |
|               | Key size (in bits) |      Not after       |                            |
|               |        384         | 2050-07-31T00:00:00Z |                            |
+---------------+--------------------+----------------------+----------------------------+

Blocks:
+---------+---------------------------------------+
| Context | Configuration                         |
+---------+---------------------------------------+
| http    | # some nginx config at http context   |
| server  | # some nginx config at server context |
+---------+---------------------------------------+

Routes:
+------------------+------------------------+--------------+-------------------------+
| Path             | Destination            | Force HTTPS? | Configuration           |
+------------------+------------------------+--------------+-------------------------+
| /app1            | app1.tsuru.example.com |              |                         |
| /login/provider1 | app2.tsuru.example.com |      ✓       |                         |
| /app3/           |                        |              | # some raw nginx config |
+------------------+------------------------+--------------+-------------------------+
`,
		},

		{
			name: "when pods have metrics",
			args: []string{"./rpaasv2", "info", "-s", "my-service", "-i", "my-instance"},
			client: &fake.FakeClient{
				FakeInfo: func(args client.InfoArgs) (*clientTypes.InstanceInfo, error) {
					require.Equal(t, args.Instance, "my-instance")
					return &clientTypes.InstanceInfo{
						Name:        "my-instance",
						Addresses:   []clientTypes.InstanceAddress{},
						Plan:        "basic",
						Binds:       []v1alpha1.Bind{},
						Replicas:    int32Ptr(3),
						Blocks:      []clientTypes.Block{},
						Routes:      []types.Route{},
						Team:        "some-team",
						Cluster:     "my-dedicated-cluster",
						Description: "some description",
						Tags:        []string{"tag1", "tag2", "tag3"},
						Flavors:     []string{"flavor1", "flavor2", "flavor-N"},
						Autoscale:   nil,
						Pods: []clientTypes.Pod{
							{
								Name:      "my-instance-75c8bdc6b9-abcde",
								IP:        "169.254.1.100",
								HostIP:    "169.254.1.100",
								Restarts:  int32(2),
								Ready:     true,
								Status:    "Running",
								CreatedAt: time.Now().In(time.UTC).Add(-12 * time.Hour),
								Ports: []clientTypes.PodPort{
									{
										Name:          "http",
										HostPort:      int32(80),
										ContainerPort: 8001,
									},
									{
										Name:          "https",
										HostPort:      int32(443),
										ContainerPort: 8002,
									},
									{
										Name:          "nginx-metrics",
										HostPort:      int32(30002),
										ContainerPort: 8003,
									},
								},
								Metrics: &clientTypes.PodMetrics{
									CPU:    "200m",
									Memory: "300Mi",
								},
							},
							{
								Name:      "my-instance-75c8bdc6b9-bcdef",
								IP:        "169.254.1.101",
								HostIP:    "169.254.1.101",
								Ready:     false,
								Status:    "Terminating",
								CreatedAt: time.Now().In(time.UTC).Add(-12 * time.Hour),
								Ports: []clientTypes.PodPort{
									{
										Name:          "http",
										HostPort:      int32(80),
										ContainerPort: 8001,
									},
									{
										Name:          "https",
										HostPort:      int32(443),
										ContainerPort: 8002,
									},
									{
										Name:          "nginx-metrics",
										HostPort:      int32(30002),
										ContainerPort: 8003,
									},
								},
								Metrics: &clientTypes.PodMetrics{
									CPU:    "2000m",
									Memory: "3000Mi",
								},
							},
						},
					}, nil
				},
			},
			expected: `Name: my-instance
Description: some description
Tags: tag1, tag2, tag3
Team owner: some-team
Plan: basic
Flavors: flavor1, flavor2, flavor-N
Cluster: my-dedicated-cluster

Pods: 3
+------------------------------+---------------+-------------+----------+-----+------+--------+
| Name                         | Host          | Status      | Restarts | Age | CPU  | Memory |
+------------------------------+---------------+-------------+----------+-----+------+--------+
| my-instance-75c8bdc6b9-abcde | 169.254.1.100 | Ready       |        2 | 12h | 20%  | 300Mi  |
| my-instance-75c8bdc6b9-bcdef | 169.254.1.101 | Terminating |        0 | 12h | 200% | 3000Mi |
+------------------------------+---------------+-------------+----------+-----+------+--------+

Ports:
+---------------+----------------+----------+-----------+
| Name          | Container port | Protocol | Host port |
+---------------+----------------+----------+-----------+
| http          |           8001 | TCP      |        80 |
| https         |           8002 | TCP      |       443 |
| nginx-metrics |           8003 | TCP      |     30002 |
+---------------+----------------+----------+-----------+

`,
		},

		{
			name: "when pods have different port set",
			args: []string{"./rpaasv2", "info", "-s", "my-service", "-i", "my-instance"},
			client: &fake.FakeClient{
				FakeInfo: func(args client.InfoArgs) (*clientTypes.InstanceInfo, error) {
					require.Equal(t, args.Instance, "my-instance")
					return &clientTypes.InstanceInfo{
						Name:        "my-instance",
						Addresses:   []clientTypes.InstanceAddress{},
						Plan:        "basic",
						Binds:       []v1alpha1.Bind{},
						Replicas:    int32Ptr(3),
						Blocks:      []clientTypes.Block{},
						Routes:      []types.Route{},
						Team:        "some-team",
						Cluster:     "my-dedicated-cluster",
						Description: "some description",
						Tags:        []string{"tag1", "tag2", "tag3"},
						Flavors:     []string{"flavor1", "flavor2", "flavor-N"},
						Autoscale:   nil,
						Pods: []clientTypes.Pod{
							{
								Name:      "my-instance-75c8bdc6b9-abcde",
								IP:        "169.254.1.100",
								HostIP:    "169.254.1.100",
								Restarts:  int32(2),
								Ready:     true,
								Status:    "Running",
								CreatedAt: time.Now().In(time.UTC).Add(-12 * time.Hour),
								Ports: []clientTypes.PodPort{
									{
										Name:          "http",
										HostPort:      int32(30000),
										ContainerPort: int32(30000),
									},
									{
										Name:          "https",
										HostPort:      int32(30001),
										ContainerPort: int32(30001),
									},
									{
										Name:          "nginx-metrics",
										HostPort:      int32(30002),
										ContainerPort: int32(30002),
									},
								},
							},
							{
								Name:      "my-instance-75c8bdc6b9-bcdef",
								IP:        "169.254.1.101",
								HostIP:    "169.254.1.101",
								Ready:     false,
								Status:    "Terminating",
								CreatedAt: time.Now().In(time.UTC).Add(-12 * time.Hour),
								Ports: []clientTypes.PodPort{
									{
										Name:          "http",
										HostPort:      int32(80),
										ContainerPort: 8001,
									},
									{
										Name:          "https",
										HostPort:      int32(443),
										ContainerPort: 8002,
									},
									{
										Name:          "nginx-metrics",
										HostPort:      int32(30002),
										ContainerPort: 8003,
									},
								},
							},
						},
					}, nil
				},
			},
			expected: `Name: my-instance
Description: some description
Tags: tag1, tag2, tag3
Team owner: some-team
Plan: basic
Flavors: flavor1, flavor2, flavor-N
Cluster: my-dedicated-cluster

Pods: 3
+------------------------------+---------------+-------------+----------+-----+--------------------------------+
| Name                         | Host          | Status      | Restarts | Age | Ports                          |
+------------------------------+---------------+-------------+----------+-----+--------------------------------+
| my-instance-75c8bdc6b9-abcde | 169.254.1.100 | Ready       |        2 | 12h | http(30000/TCP)                |
|                              |               |             |          |     | https(30001/TCP)               |
|                              |               |             |          |     | nginx-metrics(30002/TCP)       |
| my-instance-75c8bdc6b9-bcdef | 169.254.1.101 | Terminating |        0 | 12h | http(80/TCP) https(443/TCP)    |
|                              |               |             |          |     | nginx-metrics(30002/TCP)       |
+------------------------------+---------------+-------------+----------+-----+--------------------------------+

`,
		},

		{
			name: "when info route is successful and on json format",
			args: []string{"./rpaasv2", "info", "-s", "my-service", "-i", "my-instance", "--raw-output"},
			client: &fake.FakeClient{
				FakeInfo: func(args client.InfoArgs) (*clientTypes.InstanceInfo, error) {
					require.Equal(t, args.Instance, "my-instance")

					return &clientTypes.InstanceInfo{
						Name: "my-instance",
						Addresses: []clientTypes.InstanceAddress{
							{
								Type:     clientTypes.InstanceAddressTypeClusterExternal,
								Hostname: "some-host",
								IP:       "0.0.0.0",
								Status:   "ready",
							},
							{
								Type:     clientTypes.InstanceAddressTypeClusterExternal,
								Hostname: "some-host2",
								IP:       "0.0.0.1",
								Status:   "ready",
							},
						},
						Plan: "basic",
						Binds: []v1alpha1.Bind{
							{
								Name: "some-name",
								Host: "some-host",
							},
							{
								Name: "some-name2",
								Host: "some-host2",
							},
						},
						Replicas: int32Ptr(5),
						Routes: []types.Route{
							{
								Path:        "some-path",
								Destination: "some-destination",
							},
						},
						Team:        "some team",
						Description: "some description",
						Tags:        []string{"tag1", "tag2", "tag3"},
					}, nil
				},
			},
			expected: "{\n\t\"addresses\": [\n\t\t{\n\t\t\t\"type\": \"cluster-external\",\n\t\t\t\"serviceName\": \"\",\n\t\t\t\"hostname\": \"some-host\",\n\t\t\t\"ip\": \"0.0.0.0\",\n\t\t\t\"status\": \"ready\"\n\t\t},\n\t\t{\n\t\t\t\"type\": \"cluster-external\",\n\t\t\t\"serviceName\": \"\",\n\t\t\t\"hostname\": \"some-host2\",\n\t\t\t\"ip\": \"0.0.0.1\",\n\t\t\t\"status\": \"ready\"\n\t\t}\n\t],\n\t\"replicas\": 5,\n\t\"plan\": \"basic\",\n\t\"routes\": [\n\t\t{\n\t\t\t\"path\": \"some-path\",\n\t\t\t\"destination\": \"some-destination\"\n\t\t}\n\t],\n\t\"binds\": [\n\t\t{\n\t\t\t\"name\": \"some-name\",\n\t\t\t\"host\": \"some-host\"\n\t\t},\n\t\t{\n\t\t\t\"name\": \"some-name2\",\n\t\t\t\"host\": \"some-host2\"\n\t\t}\n\t],\n\t\"team\": \"some team\",\n\t\"name\": \"my-instance\",\n\t\"description\": \"some description\",\n\t\"tags\": [\n\t\t\"tag1\",\n\t\t\"tag2\",\n\t\t\"tag3\"\n\t]\n}\n",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			stdout := &bytes.Buffer{}
			stderr := &bytes.Buffer{}
			app := NewApp(stdout, stderr, tt.client)
			err := app.Run(tt.args)
			if tt.expectedError != "" {
				assert.EqualError(t, err, tt.expectedError)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.expected, stdout.String())
			assert.Empty(t, stderr.String())
		})
	}
}
