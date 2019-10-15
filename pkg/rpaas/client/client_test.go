package client

import (
	"context"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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
	testCases := []struct {
		name        string
		instance    string
		replicas    int32
		expectedErr error
		handler     http.HandlerFunc
	}{
		{
			name:        "testing with existing service and string in tsuru target",
			instance:    "rpaasv2-test",
			replicas:    int32(1),
			expectedErr: nil,
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
			clientTest := NewTsuruClient(sv.URL, "example-service", tt.instance, "f4k3t0k3n")
			assert.Equal(t, tt.expectedErr, clientTest.Scale(context.TODO(), tt.instance, tt.replicas))
		})
	}
}
