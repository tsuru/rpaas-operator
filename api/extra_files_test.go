package api

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/labstack/echo"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tsuru/rpaas-operator/rpaas"
	"github.com/tsuru/rpaas-operator/rpaas/fake"
)

func Test_listExtraFiles(t *testing.T) {
	t.Skip("not implemented yet")
}

func Test_getExtraFiles(t *testing.T) {
	t.Skip("not implemented yet")
}

func Test_addExtraFiles(t *testing.T) {
	bodyWithNoFiles, err := newMultipartFormBody("files")
	require.NoError(t, err)

	bodyWithTwoFiles, err := newMultipartFormBody("files",
		multipartFile{
			filename: "my-waf-rules.conf",
			content:  "# some custom conf",
		},
		multipartFile{
			filename: "my-another-waf-rules.cnf",
			content:  "...",
		},
	)
	require.NoError(t, err)

	testCases := []struct {
		instance     string
		requestBody  string
		expectedCode int
		expectedBody string
		manager      rpaas.RpaasManager
	}{
		{
			instance:     "my-instance",
			requestBody:  "",
			expectedCode: http.StatusBadRequest,
			expectedBody: "multipart form files is not valid",
			manager:      &fake.RpaasManager{},
		},
		{
			instance:     "my-instance",
			requestBody:  bodyWithNoFiles,
			expectedCode: http.StatusBadRequest,
			expectedBody: `files form field is required`,
			manager:      &fake.RpaasManager{},
		},
		{
			instance:     "my-instance",
			requestBody:  bodyWithTwoFiles,
			expectedCode: http.StatusCreated,
			expectedBody: "New 2 files were added",
			manager:      &fake.RpaasManager{},
		},
	}

	for _, tt := range testCases {
		t.Run("", func(t *testing.T) {
			webApi := New(nil)
			webApi.rpaasManager = tt.manager
			srv := httptest.NewServer(webApi.Handler())
			defer srv.Close()
			path := fmt.Sprintf("%s/resources/%s/files", srv.URL, tt.instance)
			request, err := http.NewRequest(http.MethodPost, path, strings.NewReader(tt.requestBody))
			require.NoError(t, err)
			request.Header.Set(echo.HeaderContentType, fmt.Sprintf(`%s; boundary=%s`, echo.MIMEMultipartForm, boundary))
			rsp, err := srv.Client().Do(request)
			require.NoError(t, err)
			assert.Equal(t, tt.expectedCode, rsp.StatusCode)
			assert.Regexp(t, tt.expectedBody, bodyContent(rsp))
		})
	}
}

func Test_updateExtraFiles(t *testing.T) {
	t.Skip("not implemented yet")
}

func Test_deleteExtraFiles(t *testing.T) {
	t.Skip("not implemented yet")
}
