// Copyright 2019 tsuru authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package cmd

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/urfave/cli"
	"gotest.tools/assert"
)

func testTableFlavors(writer io.Writer) {

	fmt.Fprintf(writer, `
+-------------+--------------------+
|   FLAVORS   |    DESCRIPTION     |
+-------------+--------------------+
| flavor name | flavor description |
+-------------+--------------------+
`)
}

func setupFlavorsApp() (*cli.App, *bytes.Buffer) {
	testApp := NewApp()

	buffer := bytes.NewBuffer(nil)
	writer := io.Writer(buffer)
	testApp.Writer = writer
	testApp.Commands = append(testApp.Commands, flavors())

	return testApp, buffer
}

func TestFlavors(t *testing.T) {
	testCases := []struct {
		name      string
		handler   http.HandlerFunc
		assertion func(t *testing.T, err error, bufferOut, bufferTest *bytes.Buffer)
		args      []string
	}{
		{
			name: "testing with valid service",
			handler: func(w http.ResponseWriter, r *http.Request) {
				assert.Equal(t, r.Method, "GET")
				assert.Equal(t, "/services/proxy/service/fake-service?callback=/resources/flavors", r.URL.RequestURI())
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
						Name:        "flavor name",
						Description: "flavor description",
					},
				}

				body, err := json.Marshal(helper)
				require.NoError(t, err)
				w.Write(body)
			},
			args: []string{"./rpaasv2", "flavors", "-s", "fake-service"},

			assertion: func(t *testing.T, err error, bufferOut, bufferTest *bytes.Buffer) {
				assert.Equal(t, bufferTest.String(), bufferOut.String())
				assert.NilError(t, err)
			},
		},
	}

	for _, tt := range testCases {
		t.Run(tt.name, func(t *testing.T) {
			ts := httptest.NewServer(tt.handler)
			defer ts.Close()
			testApp, bufferOut := setupFlavorsApp()
			os.Setenv("TSURU_TARGET", ts.URL)
			os.Setenv("TSURU_TOKEN", "f4k3t0k3n")
			//end of setup
			err := testApp.Run(tt.args)
			// unsetting env variables
			require.NoError(t, os.Unsetenv("TSURU_TARGET"))
			require.NoError(t, os.Unsetenv("TSURU_TOKEN"))
			bufferTest := bytes.NewBuffer(nil)
			testWriter := io.Writer(bufferTest)
			testTableFlavors(testWriter)
			tt.assertion(t, err, bufferOut, bufferTest)
		})
	}
}
