// Copyright 2021 tsuru authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package client

import (
	"context"
	"fmt"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestClientThroughTsuru_Info(t *testing.T) {
	tests := []struct {
		name          string
		args          InfoArgs
		expectedError string
		handler       http.HandlerFunc
	}{
		{
			name:          "when instance is empty",
			expectedError: "rpaasv2: instance cannot be empty",
		},
		{
			name: "when all args are valid",
			args: InfoArgs{
				Instance: "my-instance",
				Raw:      true,
			},
			handler: func(w http.ResponseWriter, r *http.Request) {
				assert.Equal(t, r.Method, "GET")
				assert.Equal(t, fmt.Sprintf("/services/%s/proxy/%s?callback=%s", FakeTsuruService, "my-instance", "/resources/my-instance/info"), r.URL.RequestURI())
				assert.Equal(t, "Bearer f4k3t0k3n", r.Header.Get("Authorization"))
				fmt.Fprintf(w, `{"address":[{"hostname":"some-host","ip":"0.0.0.0"},{"hostname":"some-host2","ip":"0.0.0.1"}],"replicas":5,"plan":"basic","locations":[{"path":"some-path","destination":"some-destination"}],"binds":[{"name":"some-name","host":"some-host"},{"name":"some-name2","host":"some-host2"}],"team":"some team","name":"my-instance","description":"some description","tags":["tag1","tag2","tag3"]}`)
				w.WriteHeader(http.StatusOK)
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client, server := newClientThroughTsuru(t, tt.handler)
			defer server.Close()
			_, err := client.Info(context.TODO(), tt.args)
			if tt.expectedError != "" {
				assert.EqualError(t, err, tt.expectedError)
				return
			}
			assert.NoError(t, err)
		})
	}
}
