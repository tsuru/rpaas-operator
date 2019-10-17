package cmd

import (
	"encoding/json"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/tsuru/rpaas-operator/cmd/plugin/rpaasv2/proxy"
	"gotest.tools/assert"
)

func TestPostScale(t *testing.T) {
	testCase := struct {
		name      string
		scale     scaleArgs
		handler   http.HandlerFunc
		assertion func(t *testing.T, err error, output []byte)
	}{
		name: "when a valid command is passed",
		scale: scaleArgs{service: "fake-service", instance: "fake-instance", quantity: 2,
			prox: proxy.New("fake-service", "fake-instance", "POST", nil)},
		handler: func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, r.Method, "POST")
			assert.Equal(t, "/services/fake-service/proxy/fake-instance?callback=/resources/fake-instance/scale", r.URL.RequestURI())
			w.Header().Set("Content-Type", "application/json")
			helper := struct {
				quantity string `json:"quantity="`
			}{}
			reqBodyByte, err := ioutil.ReadAll(r.Body)
			assert.NilError(t, err)
			err = json.Unmarshal(reqBodyByte, &helper)
			assert.NilError(t, err)
			respBody, err := json.Marshal(helper)
			w.WriteHeader(http.StatusCreated)
			w.Write(respBody)
		},
		assertion: func(t *testing.T, err error, output []byte) {
			assert.Equal(t, "Instance successfully scaled to 2 unit(s)\n", string(output))
			assert.NilError(t, err)
		},
	}

	t.Run(testCase.name, func(t *testing.T) {
		ts := httptest.NewServer(testCase.handler)
		testCase.scale.prox.Server = &mockServer{ts: ts}
		defer ts.Close()
		saveStdout := os.Stdout
		r, w, err := os.Pipe()
		assert.NilError(t, err)
		os.Stdout = w
		err = runScale(scaleCmd, []string{"./rpaasv2", "scale", "-s", "fake-service", "-i", "fake-instance", "-q", "2"}, testCase.scale.prox.Server)
		w.Close()
		output, err := ioutil.ReadAll(r)
		assert.NilError(t, err)
		os.Stdout = saveStdout
		testCase.assertion(t, err, output)
	})
}
