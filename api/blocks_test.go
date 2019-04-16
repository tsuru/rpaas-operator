package api

import (
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/labstack/echo"
	"github.com/stretchr/testify/assert"
	"github.com/tsuru/rpaas-operator/rpaas"
	"github.com/tsuru/rpaas-operator/rpaas/fake"
)

func Test_updateBlock(t *testing.T) {
	instanceName := "my-instance"

	testCases := []struct {
		requestBody   string
		expectedCode  int
		expectedBody  string
		expectedError error
		manager       rpaas.RpaasManager
	}{
		{
			"",
			http.StatusBadRequest,
			fmt.Sprintf("{\"message\":\"Request body can't be empty\"}\n"),
			echo.NewHTTPError(http.StatusBadRequest, "Request body can't be empty"),
			&fake.RpaasManager{},
		},
		{
			"block_name=invalid-block&content=",
			http.StatusBadRequest,
			`rpaas: block is not valid (acceptable values are: [root http server])`,
			nil,
			&fake.RpaasManager{
				FakeUpdateBlock: func(i, b, c string) error {
					return rpaas.ErrBlockInvalid
				},
			},
		},
		{
			"block_name=server&content=%23%20My%20nginx%20custom%20conf",
			http.StatusCreated,
			"",
			nil,
			&fake.RpaasManager{},
		},
		{
			"block_name=server&content=%23%20My%20nginx%20custom%20conf",
			http.StatusInternalServerError,
			fmt.Sprintf("{\"message\":\"Internal Server Error\"}\n"),
			errors.New("just another error"),
			&fake.RpaasManager{
				FakeUpdateBlock: func(i, b, c string) error {
					return errors.New("just another error")
				},
			},
		},
	}

	for _, testCase := range testCases {
		t.Run("", func(t *testing.T) {
			path := fmt.Sprintf("/resources/%s/block", instanceName)
			request := httptest.NewRequest(http.MethodPost, path, strings.NewReader(testCase.requestBody))
			request.Header.Set(echo.HeaderContentType, echo.MIMEApplicationForm)
			recorder := httptest.NewRecorder()
			e := configEcho()
			context := e.NewContext(request, recorder)
			context.SetParamNames("instance")
			context.SetParamValues(instanceName)
			setManager(context, testCase.manager)
			err := updateBlock(context)
			assert.Equal(t, testCase.expectedError, err)
			e.HTTPErrorHandler(err, context)
			assert.Equal(t, testCase.expectedCode, recorder.Code)
			assert.Equal(t, testCase.expectedBody, recorder.Body.String())
		})
	}
}
