package cmd

import (
	"bytes"
	"encoding/json"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"

	"github.com/tsuru/rpaas-operator/cmd/plugin/rpaasv2/proxy"
	"gotest.tools/assert"
)

func TestPostScale(t *testing.T) {
	testCase := struct {
		name      string
		scale     scaleArgs
		handler   http.HandlerFunc
		quantity  int
		assertion func(t *testing.T, err error, output string)
	}{
		name: "when a valid command is passed",
		scale: scaleArgs{service: "fake-service", instance: "fake-instance",
			prox: proxy.New("fake-service", "fake-instance", "POST", nil)},
		handler: func(w http.ResponseWriter, r *http.Request) {
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
		quantity: 2,
		assertion: func(t *testing.T, err error, output string) {
			assert.NilError(t, err)
			assert.Equal(t, "Instance successfully scaled to 2 unit(s)\n", output)
		},
	}
	t.Run(testCase.name, func(t *testing.T) {
		ts := httptest.NewServer(testCase.handler)
		testCase.scale.prox.Server = &mockServer{ts: ts}
		defer ts.Close()

		testCase.scale.prox.Path = "/resources/" + testCase.scale.instance + "/scale"
		testCase.scale.prox.Headers["Content-Type"] = "application/json"
		bodyReq, err := json.Marshal(map[string]string{
			"quantity=": strconv.Itoa(testCase.quantity),
		})
		assert.NilError(t, err)
		testCase.scale.prox.Body = bytes.NewBuffer(bodyReq)

		output, err := postScale(testCase.scale.prox, testCase.quantity)
		testCase.assertion(t, err, output)
	})
}
