// Copyright 2020 tsuru authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package api

import (
	"fmt"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	nginxv1alpha1 "github.com/tsuru/nginx-operator/pkg/apis/nginx/v1alpha1"
	"github.com/tsuru/rpaas-operator/internal/pkg/rpaas"
	"github.com/tsuru/rpaas-operator/internal/pkg/rpaas/fake"
	"github.com/tsuru/rpaas-operator/pkg/apis/extensions/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func Test_instanceInfo(t *testing.T) {
	getAddressOfInt32 := func(n int32) *int32 {
		return &n
	}
	tests := []struct {
		name         string
		manager      rpaas.RpaasManager
		expectedCode int
		expectedBody string
		instanceName string
	}{
		{
			name:         "when instance is found but with nil Spec",
			instanceName: "my-instance",
			manager: &fake.RpaasManager{
				FakeGetInstance: func(string) (*v1alpha1.RpaasInstance, error) {
					return &v1alpha1.RpaasInstance{
						TypeMeta: metav1.TypeMeta{
							APIVersion: "extensions.tsuru.io/v1alpha1",
							Kind:       "RpaasInstance",
						},
						ObjectMeta: metav1.ObjectMeta{
							Name: "my-instance",
							Annotations: map[string]string{
								"team-owner": "some-team",
							},
						},
						Spec: v1alpha1.RpaasInstanceSpec{},
					}, nil
				},
			},

			expectedCode: http.StatusOK,
			expectedBody: `{"address":{},"team":"some-team","name":"my-instance"}`,
		},
		{
			name:         "when instance has full InstanceInfo attributes",
			instanceName: "my-instance",
			manager: &fake.RpaasManager{
				FakeGetInstance: func(string) (*v1alpha1.RpaasInstance, error) {
					return &v1alpha1.RpaasInstance{
						TypeMeta: metav1.TypeMeta{
							APIVersion: "extensions.tsuru.io/v1alpha1",
							Kind:       "RpaasInstance",
						},
						ObjectMeta: metav1.ObjectMeta{
							Name: "my-instance",
							Annotations: map[string]string{
								"rpaas.extensions.tsuru.io/team-owner": "t1",
								"description":                          "some-description",
								"tags":                                 "tag1,tag2,tag3,tag4",
							},
						},
						Spec: v1alpha1.RpaasInstanceSpec{
							Replicas: getAddressOfInt32(5),
							Autoscale: &v1alpha1.RpaasInstanceAutoscaleSpec{
								MaxReplicas:                       3,
								MinReplicas:                       pointerToInt(1),
								TargetCPUUtilizationPercentage:    pointerToInt(70),
								TargetMemoryUtilizationPercentage: pointerToInt(1024),
							},

							PlanName: "my-plan",
							Service: &nginxv1alpha1.NginxService{
								LoadBalancerIP: "127.0.0.1",
							},
							Locations: []v1alpha1.Location{
								{Path: "/status"},
								{Path: "/admin"},
							},
						},
					}, nil
				},
				FakeInstanceAddress: func(string) (string, error) {
					return "fakeIP", nil
				},
			},
			expectedCode: http.StatusOK,
			expectedBody: `{"address":{"ip":"fakeIP"},"replicas":5,"plan":"my-plan","locations":[{"path":"/status"},{"path":"/admin"}],"service":{"loadBalancerIP":"127.0.0.1"},"autoscale":{"maxReplicas":3,"minReplicas":1,"targetCPUUtilizationPercentage":70,"targetMemoryUtilizationPercentage":1024},"team":"t1","name":"my-instance","description":"some-description","tags":["tag1","tag2","tag3",",tag4"]}`,
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
