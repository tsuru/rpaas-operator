package api

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/labstack/echo"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	nginxv1alpha1 "github.com/tsuru/nginx-operator/pkg/apis/nginx/v1alpha1"
	"github.com/tsuru/rpaas-operator/pkg/apis/extensions/v1alpha1"
	"github.com/tsuru/rpaas-operator/rpaas"
	"github.com/tsuru/rpaas-operator/rpaas/fake"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func Test_serviceCreate(t *testing.T) {
	testCases := []struct {
		requestBody  string
		expectedCode int
		expectedBody string
		manager      rpaas.RpaasManager
	}{
		{
			requestBody:  "",
			expectedCode: http.StatusBadRequest,
			expectedBody: "Request body can't be empty",
			manager:      &fake.RpaasManager{},
		},
		{
			requestBody:  "name=",
			expectedCode: http.StatusBadRequest,
			expectedBody: "name is required",
			manager: &fake.RpaasManager{
				FakeCreateInstance: func(rpaas.CreateArgs) error {
					return rpaas.ValidationError{Msg: "name is required"}
				},
			},
		},
		{
			requestBody:  "name=rpaas",
			expectedCode: http.StatusBadRequest,
			expectedBody: "plan is required",
			manager: &fake.RpaasManager{
				FakeCreateInstance: func(rpaas.CreateArgs) error {
					return rpaas.ValidationError{Msg: "plan is required"}
				},
			},
		},
		{
			requestBody:  "name=rpaas&plan=myplan",
			expectedCode: http.StatusBadRequest,
			expectedBody: "team name is required",
			manager: &fake.RpaasManager{
				FakeCreateInstance: func(rpaas.CreateArgs) error {
					return rpaas.ValidationError{Msg: "team name is required"}
				},
			},
		},
		{
			requestBody:  "name=rpaas&plan=plan2&team=myteam",
			expectedCode: http.StatusBadRequest,
			expectedBody: "invalid plan",
			manager: &fake.RpaasManager{
				FakeCreateInstance: func(rpaas.CreateArgs) error {
					return rpaas.ValidationError{Msg: "invalid plan"}
				},
			},
		},
		{
			requestBody:  "name=firstinstance&plan=myplan&team=myteam",
			expectedCode: http.StatusConflict,
			expectedBody: "firstinstance instance already exists",
			manager: &fake.RpaasManager{
				FakeCreateInstance: func(rpaas.CreateArgs) error {
					return rpaas.ConflictError{Msg: "firstinstance instance already exists"}
				},
			},
		},
		{
			requestBody:  "name=otherinstance&plan=myplan&team=myteam",
			expectedCode: http.StatusCreated,
			expectedBody: "",
			manager:      &fake.RpaasManager{},
		},
	}

	for _, tt := range testCases {
		t.Run("", func(t *testing.T) {
			srv := newTestingServer(t, tt.manager)
			defer srv.Close()
			path := fmt.Sprintf("%s/resources", srv.URL)
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

func Test_serviceDelete(t *testing.T) {
	testCases := []struct {
		instanceName string
		expectedCode int
		expectedBody string
		manager      rpaas.RpaasManager
	}{
		{
			instanceName: "unkwnown",
			expectedCode: http.StatusNotFound,
			expectedBody: "",
			manager: &fake.RpaasManager{
				FakeDeleteInstance: func(instance string) error {
					return rpaas.NotFoundError{Msg: "rpaas instance \"unkwnown\" not found"}
				},
			},
		},
		{
			instanceName: "my-instance",
			expectedCode: http.StatusOK,
			expectedBody: "",
			manager:      &fake.RpaasManager{},
		},
	}

	for _, tt := range testCases {
		t.Run("", func(t *testing.T) {
			srv := newTestingServer(t, tt.manager)
			defer srv.Close()
			path := fmt.Sprintf("%s/resources/%s", srv.URL, tt.instanceName)
			request, err := http.NewRequest(http.MethodDelete, path, nil)
			require.NoError(t, err)
			rsp, err := srv.Client().Do(request)
			require.NoError(t, err)
			assert.Equal(t, tt.expectedCode, rsp.StatusCode)
			assert.Regexp(t, tt.expectedBody, bodyContent(rsp))
		})
	}
}

func Test_servicePlans(t *testing.T) {
	testCases := []struct {
		expectedCode  int
		expectedPlans []plan
		manager       rpaas.RpaasManager
	}{
		{
			expectedCode:  http.StatusOK,
			expectedPlans: []plan{{Name: "my-plan", Description: "no plan description"}},
			manager: &fake.RpaasManager{
				FakeGetPlans: func() ([]v1alpha1.RpaasPlan, error) {
					return []v1alpha1.RpaasPlan{
						{
							TypeMeta: metav1.TypeMeta{
								Kind:       "RpaasPlan",
								APIVersion: "extensions.tsuru.io/v1alpha1",
							},
							ObjectMeta: metav1.ObjectMeta{
								Name: "my-plan",
							},
						},
					}, nil
				},
			},
		},
	}

	for _, tt := range testCases {
		t.Run("", func(t *testing.T) {
			srv := newTestingServer(t, tt.manager)
			defer srv.Close()
			path := fmt.Sprintf("%s/resources/plans", srv.URL)
			request, err := http.NewRequest(http.MethodGet, path, nil)
			require.NoError(t, err)
			rsp, err := srv.Client().Do(request)
			require.NoError(t, err)
			assert.Equal(t, tt.expectedCode, rsp.StatusCode)
			var p []plan
			require.NoError(t, json.Unmarshal([]byte(bodyContent(rsp)), &p))
			assert.Equal(t, tt.expectedPlans, p)
		})
	}
}

func Test_serviceInfo(t *testing.T) {
	getAddressOfInt32 := func(n int32) *int32 {
		return &n
	}

	testCases := []struct {
		instanceName string
		expectedCode int
		expectedInfo []map[string]string
		manager      rpaas.RpaasManager
	}{
		{
			instanceName: "my-instance",
			expectedCode: http.StatusOK,
			expectedInfo: []map[string]string{
				{
					"label": "Address",
					"value": "pending",
				},
				{
					"label": "Instances",
					"value": "0",
				},
				{
					"label": "Routes",
					"value": "",
				},
			},
			manager: &fake.RpaasManager{
				FakeGetInstance: func(string) (*v1alpha1.RpaasInstance, error) {
					return &v1alpha1.RpaasInstance{
						TypeMeta: metav1.TypeMeta{
							APIVersion: "extensions.tsuru.io/v1alpha1",
							Kind:       "RpaasInstance",
						},
						ObjectMeta: metav1.ObjectMeta{
							Name: "my-instance",
						},
						Spec: v1alpha1.RpaasInstanceSpec{},
					}, nil
				},
				FakeInstanceAddress: func(string) (string, error) {
					return "", nil
				},
			},
		},
		{
			instanceName: "my-instance",
			expectedCode: http.StatusOK,
			expectedInfo: []map[string]string{
				{
					"label": "Address",
					"value": "127.0.0.1",
				},
				{
					"label": "Instances",
					"value": "5",
				},
				{
					"label": "Routes",
					"value": "/status\n/admin",
				},
			},
			manager: &fake.RpaasManager{
				FakeGetInstance: func(string) (*v1alpha1.RpaasInstance, error) {
					return &v1alpha1.RpaasInstance{
						TypeMeta: metav1.TypeMeta{
							APIVersion: "extensions.tsuru.io/v1alpha1",
							Kind:       "RpaasInstance",
						},
						ObjectMeta: metav1.ObjectMeta{
							Name: "my-instance",
						},
						Spec: v1alpha1.RpaasInstanceSpec{
							Replicas: getAddressOfInt32(5),
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
					return "127.0.0.1", nil
				},
			},
		},
	}

	for _, tt := range testCases {
		t.Run("", func(t *testing.T) {
			srv := newTestingServer(t, tt.manager)
			defer srv.Close()
			path := fmt.Sprintf("%s/resources/%s", srv.URL, tt.instanceName)
			request, err := http.NewRequest(http.MethodGet, path, nil)
			require.NoError(t, err)
			rsp, err := srv.Client().Do(request)
			require.NoError(t, err)
			assert.Equal(t, tt.expectedCode, rsp.StatusCode)
			var info []map[string]string
			require.NoError(t, json.Unmarshal([]byte(bodyContent(rsp)), &info))
			assert.Equal(t, tt.expectedInfo, info)
		})
	}
}

func Test_serviceBindApp(t *testing.T) {
	tests := []struct {
		name         string
		requestBody  string
		expectedCode int
		manager      rpaas.RpaasManager
	}{
		{
			name:         "when no request body is sent",
			expectedCode: http.StatusBadRequest,
			manager:      &fake.RpaasManager{},
		},
		{
			name:         "when bind with application is sucessful",
			requestBody:  "app-host=app1.tsuru.example.com&app-name=app1&user=admin@tsuru.example.com&eventid=123456",
			expectedCode: http.StatusCreated,
			manager: &fake.RpaasManager{
				FakeBindApp: func(instanceName string, args rpaas.BindAppArgs) error {
					assert.Equal(t, "my-instance", instanceName)
					expected := rpaas.BindAppArgs{
						AppName: "app1",
						AppHost: "app1.tsuru.example.com",
						User:    "admin@tsuru.example.com",
						EventID: "123456",
					}
					assert.Equal(t, expected, args)
					return nil
				},
			},
		},
		{
			name:         "when BindApp method returns an error",
			expectedCode: http.StatusBadRequest,
			manager: &fake.RpaasManager{
				FakeBindApp: func(instanceName string, args rpaas.BindAppArgs) error {
					return &rpaas.ValidationError{Msg: "some error"}
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			srv := newTestingServer(t, tt.manager)
			defer srv.Close()
			path := fmt.Sprintf("%s/resources/my-instance/bind-app", srv.URL)
			request, err := http.NewRequest(http.MethodPost, path, strings.NewReader(tt.requestBody))
			require.NoError(t, err)
			request.Header.Set(echo.HeaderContentType, echo.MIMEApplicationForm)
			rsp, err := srv.Client().Do(request)
			require.NoError(t, err)
			assert.Equal(t, tt.expectedCode, rsp.StatusCode)
		})
	}
}

func Test_serviceUnbindApp(t *testing.T) {
	tests := []struct {
		name         string
		instance     string
		expectedCode int
		manager      rpaas.RpaasManager
	}{
		{
			name:         "when unbind method returns no error",
			instance:     "my-instance",
			expectedCode: http.StatusOK,
			manager: &fake.RpaasManager{
				FakeUnbindApp: func(instanceName string) error {
					assert.Equal(t, "my-instance", instanceName)
					return nil
				},
			},
		},
		{
			name:         "when UnbindApp returns an error",
			instance:     "my-instance",
			expectedCode: http.StatusBadRequest,
			manager: &fake.RpaasManager{
				FakeUnbindApp: func(instanceName string) error {
					return &rpaas.ValidationError{Msg: "some error"}
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			webApi, err := New(nil)
			require.NoError(t, err)
			webApi.rpaasManager = tt.manager
			srv := httptest.NewServer(webApi.Handler())
			defer srv.Close()
			path := fmt.Sprintf("%s/resources/%s/bind-app", srv.URL, tt.instance)
			request, err := http.NewRequest(http.MethodDelete, path, nil)
			require.NoError(t, err)
			request.Header.Set(echo.HeaderContentType, echo.MIMEApplicationForm)
			rsp, err := srv.Client().Do(request)
			require.NoError(t, err)
			assert.Equal(t, tt.expectedCode, rsp.StatusCode)
		})
	}
}

func Test_serviceBindUnit(t *testing.T) {
	t.Run("ensure bind unit route exists", func(t *testing.T) {
		instance := "my-instance"
		requestBody := "app-name=app1&app-hosts=app1.tsuru.example.com&unit-host=127.0.0.1:32123"
		srv := newTestingServer(t, &fake.RpaasManager{})
		defer srv.Close()
		path := fmt.Sprintf("%s/resources/%s/bind", srv.URL, instance)
		request, err := http.NewRequest(http.MethodPost, path, strings.NewReader(requestBody))
		require.NoError(t, err)
		request.Header.Set(echo.HeaderContentType, echo.MIMEApplicationForm)
		rsp, err := srv.Client().Do(request)
		require.NoError(t, err)
		assert.Equal(t, http.StatusCreated, rsp.StatusCode)
	})
}

func Test_serviceUnbindUnit(t *testing.T) {
	t.Run("ensure unbind unit route exists", func(t *testing.T) {
		instance := "my-instance"
		requestBody := "app-hosts=app1.tsuru.example.com&unit-host=127.0.0.1:32123"
		srv := newTestingServer(t, &fake.RpaasManager{})
		defer srv.Close()
		path := fmt.Sprintf("%s/resources/%s/bind", srv.URL, instance)
		request, err := http.NewRequest(http.MethodDelete, path, strings.NewReader(requestBody))
		require.NoError(t, err)
		request.Header.Set(echo.HeaderContentType, echo.MIMEApplicationForm)
		rsp, err := srv.Client().Do(request)
		require.NoError(t, err)
		assert.Equal(t, http.StatusOK, rsp.StatusCode)
	})
}

func bodyContent(rsp *http.Response) string {
	data, _ := ioutil.ReadAll(rsp.Body)
	return string(data)
}
