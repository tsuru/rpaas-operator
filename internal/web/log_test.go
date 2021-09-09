package web

import (
	"errors"
	"fmt"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/tsuru/rpaas-operator/internal/pkg/rpaas"
	"github.com/tsuru/rpaas-operator/internal/pkg/rpaas/fake"
)

func Test_log(t *testing.T) {
	tests := []struct {
		name         string
		instance     string
		queryString  string
		expectedCode int
		expectedBody string
		manager      rpaas.RpaasManager
	}{
		{
			name:         "when log is successfull",
			instance:     "my-instance",
			expectedCode: http.StatusOK,
			queryString:  "pod=my-pod&container=my-container&lines=15&since=5&follow=true",
			manager: &fake.RpaasManager{
				FakeLog: func(instance string, args rpaas.LogArgs) error {
					assert.Equal(t, "my-instance", instance)
					assert.NotNil(t, args.Buffer)
					assert.Equal(t, "my-pod", args.Pod)
					assert.Equal(t, "my-container", args.Container)
					assert.Equal(t, int64(15), *args.Lines)
					assert.Equal(t, int64(5), *args.SinceSeconds)
					assert.True(t, args.Follow)
					assert.False(t, args.WithTimestamp)
					return nil
				},
			},
		},
		{
			name:         "when log returns an error",
			instance:     "my-instance",
			expectedCode: http.StatusInternalServerError,
			expectedBody: "couldn't fetch kubernetes logs",
			manager: &fake.RpaasManager{
				FakeLog: func(instance string, args rpaas.LogArgs) error {
					return errors.New("couldn't fetch kubernetes logs")
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			srv := newTestingServer(t, tt.manager)
			defer srv.Close()
			path := fmt.Sprintf("%s/resources/%s/log?%s", srv.URL, tt.instance, tt.queryString)
			request, err := http.NewRequest(http.MethodGet, path, nil)
			assert.NoError(t, err)
			rsp, err := srv.Client().Do(request)
			assert.NoError(t, err)
			assert.Equal(t, tt.expectedCode, rsp.StatusCode)
			assert.Regexp(t, tt.expectedBody, bodyContent(rsp))
		})
	}
}
