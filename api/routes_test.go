package api

import (
	"fmt"
	"net/http"
	"strings"
	"testing"

	"github.com/labstack/echo"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tsuru/rpaas-operator/rpaas"
	"github.com/tsuru/rpaas-operator/rpaas/fake"
)

func Test_deleteRoute(t *testing.T) {
	tests := []struct {
		name         string
		instance     string
		requestBody  string
		expectedCode int
		expectedBody string
		manager      rpaas.RpaasManager
	}{
		{
			name:         "when manager is not set",
			expectedCode: http.StatusInternalServerError,
			manager:      nil,
		},
		{
			name:         "when delete route is successful",
			instance:     "my-instance",
			requestBody:  "path=/my/custom/path",
			expectedCode: http.StatusOK,
			manager: &fake.RpaasManager{
				FakeDeleteRoute: func(instanceName, path string) error {
					assert.Equal(t, "my-instance", instanceName)
					assert.Equal(t, "/my/custom/path", path)
					return nil
				},
			},
		},
		{
			name:         "when delete route method returns some error",
			instance:     "my-instance",
			requestBody:  "path=/",
			expectedCode: http.StatusBadRequest,
			expectedBody: "some error",
			manager: &fake.RpaasManager{
				FakeDeleteRoute: func(instanceName, path string) error {
					assert.Equal(t, "my-instance", instanceName)
					assert.Equal(t, "/", path)
					return &rpaas.ValidationError{Msg: "some error"}
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			srv := newTestingServer(t, tt.manager)
			defer srv.Close()
			path := fmt.Sprintf("%s/resources/%s/route", srv.URL, tt.instance)
			request, err := http.NewRequest(http.MethodDelete, path, strings.NewReader(tt.requestBody))
			require.NoError(t, err)
			request.Header.Set(echo.HeaderContentType, echo.MIMEApplicationForm)
			rsp, err := srv.Client().Do(request)
			require.NoError(t, err)
			assert.Equal(t, tt.expectedCode, rsp.StatusCode)
			assert.Regexp(t, tt.expectedBody, bodyContent(rsp))
		})
	}
}
