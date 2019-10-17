package client

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tsuru/rpaas-operator/pkg/rpaas/client/types"
)

func TestNewClient(t *testing.T) {
	t.Run("creating new client", func(t *testing.T) {
		rpaasClient := New("http://fakehosturi.example.com")
		require.NotNil(t, rpaasClient)
		assert.Equal(t, rpaasClient.hostAPI, "http://fakehosturi.example.com")
		assert.NotNil(t, rpaasClient.httpClient)
	})
}

func TestRpaasClient_Scale(t *testing.T) {
	testCases := []struct {
		instance    string
		replicas    int32
		expectedErr error
		handler     http.HandlerFunc
	}{
		{
			expectedErr: fmt.Errorf("instance can't be nil"),
		},
		{
			instance:    "test-instance",
			replicas:    int32(-1),
			expectedErr: fmt.Errorf("replicas number must be greater or equal to zero"),
		},
		{
			instance: "test-instance",
			replicas: int32(2),
			handler: func(w http.ResponseWriter, r *http.Request) {
				assert.Equal(t, r.Method, "POST")
				assert.Equal(t, r.URL.RequestURI(), "/resources/test-instance/scale")
				bodyBytes, err := ioutil.ReadAll(r.Body)
				require.NoError(t, err)
				defer r.Body.Close()
				assert.Equal(t, string(bodyBytes), "quantity=2")
				w.WriteHeader(http.StatusCreated)
			},
		},
		{
			instance: "test-instance",
			replicas: int32(2),
			handler: func(w http.ResponseWriter, r *http.Request) {
				assert.Equal(t, r.Method, "POST")
				assert.Equal(t, r.URL.RequestURI(), "/resources/test-instance/scale")
				bodyBytes, err := ioutil.ReadAll(r.Body)
				require.NoError(t, err)
				defer r.Body.Close()
				assert.Equal(t, string(bodyBytes), "quantity=2")
				w.Write([]byte("Some Error"))
				w.WriteHeader(http.StatusBadRequest)
			},
			expectedErr: fmt.Errorf("unexpected status code: body: Some Error"),
		},
	}

	for _, tt := range testCases {
		t.Run("", func(t *testing.T) {
			sv := httptest.NewServer(tt.handler)
			defer sv.Close()
			clientTest := &RpaasClient{httpClient: &http.Client{}, hostAPI: sv.URL}
			assert.Equal(t, tt.expectedErr, clientTest.Scale(context.TODO(), tt.instance, tt.replicas))
		})
	}
}

func TestScaleWithTsuru(t *testing.T) {
	type testStruct struct {
		name      string
		instance  string
		replicas  int32
		assertion func(t *testing.T, err error, clientTest *RpaasClient, tt testStruct)
		handler   http.HandlerFunc
	}
	testCases := []testStruct{
		{
			name:     "testing with existing service and string in tsuru target",
			instance: "rpaasv2-test",
			replicas: int32(1),
			assertion: func(t *testing.T, err error, clientTest *RpaasClient, tt testStruct) {
				assert.NoError(t, err)
				assert.Equal(t, nil, clientTest.Scale(context.TODO(), tt.instance, tt.replicas))
			},
			handler: func(w http.ResponseWriter, r *http.Request) {
				assert.Equal(t, r.Method, "POST")
				assert.Equal(t, "/services/example-service/proxy/rpaasv2-test?callback=/resources/rpaasv2-test/scale", r.URL.RequestURI())
				assert.Equal(t, "Bearer f4k3t0k3n", r.Header.Get("Authorization"))
				bodyBytes, err := ioutil.ReadAll(r.Body)
				require.NoError(t, err)
				defer r.Body.Close()
				assert.Equal(t, string(bodyBytes), "quantity=1")
				w.WriteHeader(http.StatusCreated)
				w.Write([]byte("Instance successfully scaled to 1 unit(s)"))
			},
		},
	}

	for _, tt := range testCases {
		t.Run(tt.name, func(t *testing.T) {
			sv := httptest.NewServer(tt.handler)
			defer sv.Close()
			clientTest, err := NewTsuruClient(sv.URL, "example-service", "f4k3t0k3n")
			tt.assertion(t, err, clientTest, tt)
		})
	}
}

func TestGetPlansTroughHostAPI(t *testing.T) {
	testCases := []struct {
		name      string
		instance  string
		assertion func(t *testing.T, plans []types.Plan, err error)
		handler   http.HandlerFunc
	}{
		{
			name:     "valid request",
			instance: "test-instance",
			assertion: func(t *testing.T, plans []types.Plan, err error) {
				assert.NoError(t, err)
				expectedPlans := []types.Plan{
					{
						Name:        "dsr",
						Description: "rpaas dsr",
						Default:     true,
					},
				}
				assert.Equal(t, expectedPlans, plans)
			},
			handler: func(w http.ResponseWriter, r *http.Request) {
				assert.Equal(t, r.Method, "GET")
				assert.Equal(t, r.URL.RequestURI(), "/resources/test-instance/plans")
				bodyBytes, err := ioutil.ReadAll(r.Body)
				require.NoError(t, err)
				defer r.Body.Close()
				assert.NotNil(t, bodyBytes)
				w.Header().Set("Content-Type", "application/json")
				helper := []struct {
					Name        string `json:"name"`
					Description string `json:"description"`
					Default     bool   `json:"default"`
				}{
					{
						Name:        "dsr",
						Description: "rpaas dsr",
						Default:     true,
					},
				}
				body, err := json.Marshal(helper)
				require.NoError(t, err)
				w.Write(body)
				w.WriteHeader(http.StatusOK)
			},
		},
		{
			name:     "some error returned on the request",
			instance: "test-instance",
			handler: func(w http.ResponseWriter, r *http.Request) {
				assert.Equal(t, r.Method, "GET")
				assert.Equal(t, r.URL.RequestURI(), "/resources/test-instance/plans")
				bodyBytes, err := ioutil.ReadAll(r.Body)
				require.NoError(t, err)
				defer r.Body.Close()
				assert.NotNil(t, bodyBytes)
				w.WriteHeader(http.StatusBadRequest)
				w.Write([]byte("Some Error"))
			},
			assertion: func(t *testing.T, plans []types.Plan, err error) {
				assert.Error(t, fmt.Errorf("unexpected status code: body: Some Error"), err)
			},
		},
	}

	for _, tt := range testCases {
		t.Run(tt.name, func(t *testing.T) {
			sv := httptest.NewServer(tt.handler)
			defer sv.Close()
			clientTest := &RpaasClient{httpClient: &http.Client{}, hostAPI: sv.URL}
			plans, err := clientTest.GetPlans(context.TODO(), &tt.instance)
			tt.assertion(t, plans, err)
		})
	}
}

func TestPlansTroughTsuru(t *testing.T) {
	testCases := []struct {
		name      string
		instance  string
		assertion func(t *testing.T, plans []types.Plan, err error)
		handler   http.HandlerFunc
	}{
		{
			name:     "testing with existing service and string in tsuru target",
			instance: "rpaasv2-test",
			assertion: func(t *testing.T, plans []types.Plan, err error) {
				assert.NoError(t, err)
				expectedPlans := []types.Plan{
					{
						Name:        "dsr",
						Description: "rpaas dsr",
						Default:     true,
					},
				}
				assert.Equal(t, expectedPlans, plans)
			},
			handler: func(w http.ResponseWriter, r *http.Request) {
				assert.Equal(t, r.Method, "GET")
				assert.Equal(t, "/services/example-service/proxy/rpaasv2-test?callback=/resources/rpaasv2-test/plans", r.URL.RequestURI())
				assert.Equal(t, "Bearer f4k3t0k3n", r.Header.Get("Authorization"))
				_, err := ioutil.ReadAll(r.Body)
				require.NoError(t, err)
				defer r.Body.Close()
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusOK)
				helper := []struct {
					Name        string `json:"name"`
					Description string `json:"description"`
					Default     bool   `json:"default"`
				}{
					{
						Name:        "dsr",
						Description: "rpaas dsr",
						Default:     true,
					},
				}

				body, err := json.Marshal(helper)
				require.NoError(t, err)
				w.Write(body)
			},
		},
	}

	for _, tt := range testCases {
		t.Run(tt.name, func(t *testing.T) {
			sv := httptest.NewServer(tt.handler)
			defer sv.Close()
			clientTest, err := NewTsuruClient(sv.URL, "example-service", "f4k3t0k3n")
			plans, err := clientTest.GetPlans(context.TODO(), &tt.instance)
			tt.assertion(t, plans, err)
		})
	}
}

func TestGetFlavorsTroughHostAPI(t *testing.T) {
	testCases := []struct {
		name      string
		instance  string
		assertion func(t *testing.T, flavors []types.Flavor, err error)
		handler   http.HandlerFunc
	}{
		{
			name:     "valid request",
			instance: "test-instance",
			assertion: func(t *testing.T, flavors []types.Flavor, err error) {
				assert.NoError(t, err)
				expectedFlavors := []types.Flavor{
					{
						Name:        "dsr",
						Description: "rpaas dsr",
					},
				}
				assert.Equal(t, expectedFlavors, flavors)
			},
			handler: func(w http.ResponseWriter, r *http.Request) {
				assert.Equal(t, r.Method, "GET")
				assert.Equal(t, r.URL.RequestURI(), "/resources/test-instance/flavors")
				bodyBytes, err := ioutil.ReadAll(r.Body)
				require.NoError(t, err)
				defer r.Body.Close()
				assert.NotNil(t, bodyBytes)
				w.Header().Set("Content-Type", "application/json")
				helper := []struct {
					Name        string `json:"name"`
					Description string `json:"description"`
				}{
					{
						Name:        "dsr",
						Description: "rpaas dsr",
					},
				}
				body, err := json.Marshal(helper)
				require.NoError(t, err)
				w.Write(body)
				w.WriteHeader(http.StatusOK)
			},
		},
		{
			name:     "some error returned on the request",
			instance: "test-instance",
			handlerPlans: func(w http.ResponseWriter, r *http.Request) {
				assert.Equal(t, r.Method, "GET")
				assert.Equal(t, r.URL.RequestURI(), "/resources/test-instance/flavors")
				bodyBytes, err := ioutil.ReadAll(r.Body)
				require.NoError(t, err)
				defer r.Body.Close()
				assert.NotNil(t, bodyBytes)
				w.WriteHeader(http.StatusBadRequest)
				w.Write([]byte("Some Error"))
			},
			assertion: func(t *testing.T, flavors []types.Flavor, err error) {
				assert.Error(t, fmt.Errorf("unexpected status code: body: Some Error"), err)
			},
		},
	}

	for _, tt := range testCases {
		t.Run(tt.name, func(t *testing.T) {
			sv := httptest.NewServer(tt.handlerPlans)
			defer sv.Close()
			clientTest := &RpaasClient{httpClient: &http.Client{}, hostAPI: sv.URL}
			flavors, err := clientTest.GetFlavors(context.TODO(), &tt.instance)
			tt.assertion(t, flavors, err)
		})
	}
}

func TestFlavorsTroughTsuru(t *testing.T) {
	testCases := []struct {
		name      string
		instance  string
		assertion func(t *testing.T, flavors []types.Flavor, err error)
		handler   http.HandlerFunc
	}{
		{
			name:     "testing with existing service and string in tsuru target",
			instance: "rpaasv2-test",
			assertion: func(t *testing.T, flavors []types.Flavor, err error) {
				assert.NoError(t, err)
				expectedPlans := []types.Flavor{
					{
						Name:        "dsr",
						Description: "rpaas dsr",
					},
				}
				assert.Equal(t, expectedPlans, flavors)
			},
			handler: func(w http.ResponseWriter, r *http.Request) {
				assert.Equal(t, r.Method, "GET")
				assert.Equal(t, "/services/example-service/proxy/rpaasv2-test?callback=/resources/rpaasv2-test/flavors", r.URL.RequestURI())
				assert.Equal(t, "Bearer f4k3t0k3n", r.Header.Get("Authorization"))
				_, err := ioutil.ReadAll(r.Body)
				require.NoError(t, err)
				defer r.Body.Close()
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusOK)
				helper := []struct {
					Name        string `json:"name"`
					Description string `json:"description"`
				}{
					{
						Name:        "dsr",
						Description: "rpaas dsr",
					},
				}

				body, err := json.Marshal(helper)
				require.NoError(t, err)
				w.Write(body)
			},
		},
	}

	for _, tt := range testCases {
		t.Run(tt.name, func(t *testing.T) {
			sv := httptest.NewServer(tt.handler)
			defer sv.Close()
			clientTest, err := NewTsuruClient(sv.URL, "example-service", "f4k3t0k3n")
			flavors, err := clientTest.GetFlavors(context.TODO(), &tt.instance)
			tt.assertion(t, flavors, err)
		})
	}
}
