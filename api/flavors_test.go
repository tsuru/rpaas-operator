package api

import (
	"fmt"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tsuru/rpaas-operator/rpaas"
	"github.com/tsuru/rpaas-operator/rpaas/fake"
)

func Test_getServiceFlavors(t *testing.T) {
	tests := []struct {
		name         string
		manager      rpaas.RpaasManager
		expectedCode int
		expectedBody string
	}{
		{
			name:         "when no flavors are available, should return an empty array",
			expectedCode: http.StatusOK,
			expectedBody: `\[\]`,
			manager:      &fake.RpaasManager{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			srv := newTestingServer(t, tt.manager)
			defer srv.Close()
			path := fmt.Sprintf("%s/resources/flavors", srv.URL)
			request, err := http.NewRequest(http.MethodGet, path, nil)
			require.NoError(t, err)
			rsp, err := srv.Client().Do(request)
			require.NoError(t, err)
			assert.Equal(t, tt.expectedCode, rsp.StatusCode)
			assert.Regexp(t, tt.expectedBody, bodyContent(rsp))
		})
	}
}

func Test_getInstanceFlavors(t *testing.T) {
	tests := []struct {
		name         string
		manager      rpaas.RpaasManager
		instance     string
		expectedCode int
		expectedBody string
	}{
		{
			name:         "when no flavors are available, should return an empty array",
			manager:      &fake.RpaasManager{},
			instance:     "my-instance",
			expectedCode: http.StatusOK,
			expectedBody: `\[\]`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			srv := newTestingServer(t, tt.manager)
			defer srv.Close()
			path := fmt.Sprintf("%s/resources/%s/flavors", srv.URL, tt.instance)
			request, err := http.NewRequest(http.MethodGet, path, nil)
			require.NoError(t, err)
			rsp, err := srv.Client().Do(request)
			require.NoError(t, err)
			assert.Equal(t, tt.expectedCode, rsp.StatusCode)
			assert.Regexp(t, tt.expectedBody, bodyContent(rsp))
		})
	}
}
