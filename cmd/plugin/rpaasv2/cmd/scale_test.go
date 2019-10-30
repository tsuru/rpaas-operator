// Copyright 2019 tsuru authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package cmd

import (
	"bytes"
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

func setupScaleApp() (*cli.App, *bytes.Buffer) {
	testApp := NewApp()

	buffer := bytes.NewBuffer(nil)
	writer := io.Writer(buffer)
	testApp.Writer = writer
	testApp.Commands = append(testApp.Commands, scale())

	return testApp, buffer
}

func TestScale(t *testing.T) {
	testCase := struct {
		name      string
		handler   http.HandlerFunc
		assertion func(t *testing.T, err error, buffer *bytes.Buffer)
		args      []string
	}{
		name: "when a valid command is passed",
		handler: func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, r.Method, "POST")
			assert.Equal(t, "/services/fake-service/proxy/fake-instance?callback=/resources/fake-instance/scale", r.URL.RequestURI())
			w.Header().Set("Content-Type", "application/x-www-form-urlencoded")
			reqBodyByte, err := ioutil.ReadAll(r.Body)
			assert.NilError(t, err)
			bodyString := string(reqBodyByte)
			assert.Equal(t, "quantity=2", bodyString)
			w.WriteHeader(http.StatusCreated)
		},
		assertion: func(t *testing.T, err error, buffer *bytes.Buffer) {
			str := buffer.String()
			assert.Equal(t, "Instance successfully scaled to 2 unit(s)\n", str)
			assert.NilError(t, err)
		},
		args: []string{"./rpaasv2", "scale", "-s", "fake-service", "-i", "fake-instance", "-q", "2"},
	}
	t.Run(testCase.name, func(t *testing.T) {
		// setup
		ts := httptest.NewServer(testCase.handler)
		defer ts.Close()
		testApp, buffer := setupScaleApp()
		os.Setenv("TSURU_TARGET", ts.URL)
		os.Setenv("TSURU_TOKEN", "f4k3t0k3n")
		//end of setup
		err := testApp.Run(testCase.args)
		// unsetting env variables
		require.NoError(t, os.Unsetenv("TSURU_TARGET"))
		require.NoError(t, os.Unsetenv("TSURU_TOKEN"))
		testCase.assertion(t, err, buffer)
	})
}
