package client

import (
	"context"
	"fmt"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestClientThroughTsuru_Log(t *testing.T) {
	tests := []struct {
		name          string
		args          LogArgs
		expectedError string
		handler       http.HandlerFunc
	}{
		{
			name:          "when instance is empty",
			expectedError: "rpaasv2: instance cannot be empty",
		},
		{
			name: "simple log request",
			args: LogArgs{
				Instance: "my-instance",
			},
			handler: func(w http.ResponseWriter, r *http.Request) {
				assert.Equal(t, r.Method, "GET")
				assert.Equal(t, fmt.Sprintf("/1.20/services/%s/resources/%s/log?%s", FakeTsuruService, "my-instance", "color=false&follow=false"), r.URL.RequestURI())
				assert.Equal(t, "Bearer f4k3t0k3n", r.Header.Get("Authorization"))
				w.WriteHeader(http.StatusOK)
			},
		},
		{
			name: "all arguments log request",
			args: LogArgs{
				Instance:  "my-instance",
				Follow:    true,
				Pod:       "some-pod",
				Container: "some-container",
				Lines:     10,
			},
			handler: func(w http.ResponseWriter, r *http.Request) {
				assert.Equal(t, r.Method, "GET")
				assert.Equal(t, fmt.Sprintf("/1.20/services/%s/resources/%s/log?color=false&container=some-container&follow=true&lines=10&pod=some-pod", FakeTsuruService, "my-instance"), r.URL.RequestURI())
				assert.Equal(t, "Bearer f4k3t0k3n", r.Header.Get("Authorization"))
				w.WriteHeader(http.StatusOK)
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client, server := newClientThroughTsuru(t, tt.handler)
			defer server.Close()
			err := client.Log(context.TODO(), tt.args)
			if tt.expectedError != "" {
				assert.EqualError(t, err, tt.expectedError)
				return
			}
			assert.NoError(t, err)
		})
	}
}
