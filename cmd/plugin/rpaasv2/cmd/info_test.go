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

func setupInfoApp() (*cli.App, *bytes.Buffer) {
	testApp := NewApp()

	buffer := bytes.NewBuffer(nil)
	writer := io.Writer(buffer)
	testApp.Writer = writer
	testApp.Commands = append(testApp.Commands, info())

	return testApp, buffer
}

func testTableInfo(writer io.Writer) {

	fmt.Fprintf(writer, `
+-----------+------------------+---------+
|   PLANS   |   DESCRIPTION    | DEFAULT |
+-----------+------------------+---------+
| plan name | plan description | true    |
+-----------+------------------+---------+

+-------------+--------------------+
|   FLAVORS   |    DESCRIPTION     |
+-------------+--------------------+
| flavor name | flavor description |
+-------------+--------------------+
`)
}

func TestInfo(t *testing.T) {
	count := 0
	testCases := []struct {
		name          string
		handlerSwitch http.HandlerFunc
		assertion     func(t *testing.T, err error, bufferOut, bufferTest *bytes.Buffer)
		args          []string
	}{
		{
			name: "testing info route with valid arguments",
			handlerSwitch: func(w http.ResponseWriter, r *http.Request) {
				switch count {
				case 0:
					assert.Equal(t, r.Method, "GET")
					// log.Fatalf("request string = %s", r.URL.RequestURI())
					assert.Equal(t, "/services/fake-service/proxy/fake-instance?callback=/resources/fake-instance/plans", r.URL.RequestURI())
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
							Name:        "plan name",
							Description: "plan description",
							Default:     true,
						},
					}

					body, err := json.Marshal(helper)
					require.NoError(t, err)
					w.Write(body)
					count++
				case 1:
					assert.Equal(t, r.Method, "GET")
					assert.Equal(t, "/services/fake-service/proxy/fake-instance?callback=/resources/fake-instance/flavors", r.URL.RequestURI())
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
					count = 0
				default:
					require.FailNow(t, "error while counting")
				}
			},
			args: []string{"./rpaasv2", "info", "-s", "fake-service", "-i", "fake-instance"},

			assertion: func(t *testing.T, err error, bufferOut, bufferTest *bytes.Buffer) {
				assert.Equal(t, bufferTest.String(), bufferOut.String())
				assert.NilError(t, err)
			},
		},
	}
	for _, tt := range testCases {
		t.Run(tt.name, func(t *testing.T) {
			count = 0
			// setup
			ts := httptest.NewServer(tt.handlerSwitch)
			defer ts.Close()
			testApp, bufferOut := setupInfoApp()
			os.Setenv("TSURU_TARGET", ts.URL)
			os.Setenv("TSURU_TOKEN", "f4k3t0k3n")
			//end of setup
			err := testApp.Run(tt.args)
			// unsetting env variables
			require.NoError(t, os.Unsetenv("TSURU_TARGET"))
			require.NoError(t, os.Unsetenv("TSURU_TOKEN"))
			bufferTest := bytes.NewBuffer(nil)
			testWriter := io.Writer(bufferTest)
			testTableInfo(testWriter)
			tt.assertion(t, err, bufferOut, bufferTest)
		})
	}
}
