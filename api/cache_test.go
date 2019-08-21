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

func Test_cachePurge(t *testing.T) {
	testCases := []struct {
		instanceName string
		requestBody  string
		expectedCode int
		expectedBody string
		manager      rpaas.RpaasManager
	}{
		{
			instanceName: "my-instance",
			requestBody:  "",
			expectedCode: http.StatusBadRequest,
			expectedBody: "Request body can't be empty",
			manager:      &fake.RpaasManager{},
		},
	}

	for _, tt := range testCases {
		t.Run("", func(t *testing.T) {
			srv := newTestingServer(t, tt.manager)
			defer srv.Close()
			path := fmt.Sprintf("%s/resources/%s/purge", srv.URL, tt.instanceName)
			request, err := http.NewRequest(http.MethodPost, path, strings.NewReader(tt.requestBody))
			require.NoError(t, err)
			request.Header.Set(echo.HeaderContentType, echo.MIMEApplicationForm)
			rsp, err := srv.Client().Do(request)
			require.NoError(t, err)
			assert.Equal(t, tt.expectedCode, rsp.StatusCode)
			assert.Regexp(t, tt.expectedBody, bodyContent(rsp))
		})
	}
}
