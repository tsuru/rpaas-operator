package cmd

import (
	"encoding/json"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/tsuru/rpaas-operator/cmd/plugin/rpaasv2/proxy"
	"gotest.tools/assert"
)

func TestPostScale(t *testing.T) {
	testCase := struct {
		name      string
		scale     scaleArgs
		handler   http.HandlerFunc
		assertion func(t *testing.T, err error, output string)
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
		assertion: func(t *testing.T, err error, output string) {
			assert.NilError(t, err)
			assert.Equal(t, "Instance successfully scaled to 2 unit(s)\n", output)
		},
	}
	t.Run(testCase.name, func(t *testing.T) {
		ts := httptest.NewServer(testCase.handler)
		testCase.scale.prox.Server = &mockServer{ts: ts}
		defer ts.Close()
		stringResp, err := runScale(testCase.scale)
		testCase.assertion(t, err, stringResp)
	})
}
