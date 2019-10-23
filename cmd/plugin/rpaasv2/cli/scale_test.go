package cli

import (
	"bytes"
	"io"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/tsuru/rpaas-operator/cmd/plugin/rpaasv2/types"
	"gotest.tools/assert"
)

func TestPostScale(t *testing.T) {
	testCase := struct {
		name      string
		manager   *types.Manager
		handler   http.HandlerFunc
		assertion func(t *testing.T, err error, buffer bytes.Buffer)
	}{
		name: "when a valid command is passed",
		manager: &types.Manager{
			Token: "f4k3t0k3n",
		},
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
		assertion: func(t *testing.T, err error, buffer bytes.Buffer) {
			str := buffer.String()
			assert.Equal(t, "Instance successfully scaled to 2 unit(s)\n", str)
			assert.NilError(t, err)
		},
	}

	t.Run(testCase.name, func(t *testing.T) {
		// setup
		ts := httptest.NewServer(testCase.handler)
		defer ts.Close()
		app := AppInit()
		testCase.manager.Target = ts.URL
		var buffer bytes.Buffer
		testCase.manager.Writer = io.Writer(&buffer)
		SetBeforeFunc(app, testCase.manager)
		app.Commands = append(app.Commands, CreateScale())
		//end of setup

		err := app.Run([]string{"./rpaasv2", "scale", "-s", "fake-service", "-i", "fake-instance", "-q", "2"})
		testCase.assertion(t, err, buffer)
	})
}
