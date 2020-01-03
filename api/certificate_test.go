package api

import (
	"fmt"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tsuru/rpaas-operator/internal/pkg/rpaas"
	"github.com/tsuru/rpaas-operator/internal/pkg/rpaas/fake"
)

func Test_GetCertificates(t *testing.T) {
	tests := []struct {
		name         string
		manager      rpaas.RpaasManager
		instance     string
		expectedCode int
		expectedBody string
	}{
		{
			name:         "when the instance does not exist",
			manager:      &fake.RpaasManager{},
			instance:     "my-instance",
			expectedCode: http.StatusOK,
			expectedBody: "[]\n",
		},
		{
			name: "when the instance and certificate exists",
			manager: &fake.RpaasManager{
				FakeGetCertificates: func(instanceName string) ([]rpaas.CertificateData, error) {
					return []rpaas.CertificateData{
						{
							Name:        "cert-name",
							Certificate: `my-certificate`,
							Key:         `my-key`,
						},
					}, nil
				},
			},
			instance:     "real-instance",
			expectedCode: http.StatusOK,
			expectedBody: "[{\"name\":\"cert-name\",\"certificate\":\"my-certificate\",\"key\":\"my-key\"}]\n",
		},
		{
			name: "when the instance exists but the certificate has a missing key",
			manager: &fake.RpaasManager{
				FakeGetCertificates: func(instanceName string) ([]rpaas.CertificateData, error) {
					return nil, fmt.Errorf("key data not found")
				},
			},
			instance:     "real-instance",
			expectedCode: http.StatusInternalServerError,
			expectedBody: "{\"message\":\"Internal Server Error\"}\n",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			srv := newTestingServer(t, tt.manager)
			defer srv.Close()
			path := fmt.Sprintf("%s/resources/%s/certificate", srv.URL, tt.instance)
			request, err := http.NewRequest(http.MethodGet, path, nil)
			require.NoError(t, err)
			rsp, err := srv.Client().Do(request)
			require.NoError(t, err)
			assert.Equal(t, tt.expectedCode, rsp.StatusCode)
			assert.Equal(t, tt.expectedBody, bodyContent(rsp))
		})
	}

}
