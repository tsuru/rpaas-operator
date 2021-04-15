// Copyright 2020 tsuru authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package web

import (
	"errors"
	"fmt"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/tsuru/rpaas-operator/api/v1alpha1"
	"github.com/tsuru/rpaas-operator/internal/pkg/rpaas"
	"github.com/tsuru/rpaas-operator/internal/pkg/rpaas/fake"
	clientTypes "github.com/tsuru/rpaas-operator/pkg/rpaas/client/types"
)

func Test_instanceInfo(t *testing.T) {
	tests := []struct {
		name         string
		manager      rpaas.RpaasManager
		expectedCode int
		expectedBody string
		instanceName string
	}{
		{
			name:         "when instance is found",
			instanceName: "my-instance",
			manager: &fake.RpaasManager{
				FakeGetInstanceInfo: func(instanceName string) (*clientTypes.InstanceInfo, error) {
					assert.Equal(t, "my-instance", instanceName)
					return &clientTypes.InstanceInfo{
						Addresses: []clientTypes.InstanceAddress{
							{
								ServiceName: "my-instance-service",
								Hostname:    "some host name",
								IP:          "0.0.0.0",
								Status:      "ready",
							},
							{
								ServiceName: "my-instance-service",
								Hostname:    "some host name 2",
								IP:          "0.0.0.1",
								Status:      "ready",
							},
						},
						Replicas:    int32Ptr(5),
						Plan:        "basic",
						Team:        "some team",
						Name:        "some rpaas instance name",
						Description: "some description",
						Tags: []string{
							"tag1",
							"tag2",
						},
						Binds: []v1alpha1.Bind{
							{
								Name: "app-default",
								Host: "some host ip address",
							},
							{
								Name: "app-backup",
								Host: "some host backup ip address",
							},
						},
						Routes: []clientTypes.Route{
							{
								Path:        "some location path",
								Destination: "some destination",
							},
							{
								Path:        "some location path 2",
								Destination: "some destination 2",
							},
						},
						Autoscale: &clientTypes.Autoscale{
							MaxReplicas: pointerToInt32(3),
							MinReplicas: pointerToInt32(1),
							CPU:         pointerToInt32(70),
							Memory:      pointerToInt32(1024),
						},
					}, nil
				},
			},
			expectedCode: http.StatusOK,
			expectedBody: "{\"addresses\":[{\"serviceName\":\"my-instance-service\",\"hostname\":\"some host name\",\"ip\":\"0.0.0.0\",\"status\":\"ready\"},{\"serviceName\":\"my-instance-service\",\"hostname\":\"some host name 2\",\"ip\":\"0.0.0.1\",\"status\":\"ready\"}],\"replicas\":5,\"plan\":\"basic\",\"routes\":[{\"path\":\"some location path\",\"destination\":\"some destination\"},{\"path\":\"some location path 2\",\"destination\":\"some destination 2\"}],\"autoscale\":{\"minReplicas\":1,\"maxReplicas\":3,\"cpu\":70,\"memory\":1024},\"binds\":[{\"name\":\"app-default\",\"host\":\"some host ip address\"},{\"name\":\"app-backup\",\"host\":\"some host backup ip address\"}],\"team\":\"some team\",\"name\":\"some rpaas instance name\",\"description\":\"some description\",\"tags\":[\"tag1\",\"tag2\"]}",
		},
		{
			name:         "when some error occurs while creating the info Payload",
			instanceName: "my-instance",
			manager: &fake.RpaasManager{
				FakeGetInstanceInfo: func(instanceName string) (*clientTypes.InstanceInfo, error) {
					assert.Equal(t, "my-instance", instanceName)
					return nil, errors.New("error while setting address")
				},
			},
			expectedCode: http.StatusInternalServerError,
			expectedBody: "{\"message\":\"error while setting address\"}",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			srv := newTestingServer(t, tt.manager)
			defer srv.Close()
			path := fmt.Sprintf("%s/resources/%s/info", srv.URL, tt.instanceName)
			request, err := http.NewRequest(http.MethodGet, path, nil)
			require.NoError(t, err)
			rsp, err := srv.Client().Do(request)
			require.NoError(t, err)
			assert.Equal(t, tt.expectedCode, rsp.StatusCode)
			assert.Equal(t, tt.expectedBody, bodyContent(rsp))
		})
	}
}
