package cmd

import (
	"bytes"
	"fmt"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tsuru/rpaas-operator/pkg/rpaas/client"
	"github.com/tsuru/rpaas-operator/pkg/rpaas/client/fake"
	clientTypes "github.com/tsuru/rpaas-operator/pkg/rpaas/client/types"
)

func TestGetAutoscale(t *testing.T) {
	tests := []struct {
		name          string
		args          []string
		expected      string
		expectedError string
		client        client.Client
	}{
		{
			name:          "when GetAutoscale does not find the instance",
			args:          []string{"./rpaasv2", "autoscale", "get", "-s", "my-service", "-i", "my-instance"},
			expectedError: "not found error",
			client: &fake.FakeClient{
				FakeGetAutoscale: func(args client.GetAutoscaleArgs) (*clientTypes.Autoscale, *http.Response, error) {
					require.Equal(t, args.Instance, "my-instance")
					return nil, nil, fmt.Errorf("not found error")
				},
			},
		},
		{
			name: "when get autoscale route is successful",
			args: []string{"./rpaasv2", "autoscale", "get", "-s", "my-service", "-i", "my-instance"},
			client: &fake.FakeClient{
				FakeGetAutoscale: func(args client.GetAutoscaleArgs) (*clientTypes.Autoscale, *http.Response, error) {
					require.Equal(t, args.Instance, "my-instance")
					return &clientTypes.Autoscale{
						MaxReplicas: int32Ptr(5),
						MinReplicas: int32Ptr(2),
						CPU:         int32Ptr(50),
						Memory:      int32Ptr(55),
					}, nil, nil
				},
			},
			expected: `+----------+--------------------+
| REPLICAS | TARGET UTILIZATION |
+----------+--------------------+
| Max: 5   | CPU: 50%           |
| Min: 2   | Memory: 55%        |
+----------+--------------------+
`,
		},
		{
			name: "when get autoscale route is successful on JSON format",
			args: []string{"./rpaasv2", "autoscale", "get", "-s", "my-service", "-i", "my-instance", "--raw"},
			client: &fake.FakeClient{
				FakeGetAutoscale: func(args client.GetAutoscaleArgs) (*clientTypes.Autoscale, *http.Response, error) {
					require.Equal(t, args.Instance, "my-instance")
					return &clientTypes.Autoscale{
						MaxReplicas: int32Ptr(5),
						MinReplicas: int32Ptr(2),
						CPU:         int32Ptr(50),
						Memory:      int32Ptr(55),
					}, nil, nil
				},
			},
			expected: "{\n\t\"minReplicas\": 2,\n\t\"maxReplicas\": 5,\n\t\"cpu\": 50,\n\t\"memory\": 55\n}\n",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			stdout := &bytes.Buffer{}
			stderr := &bytes.Buffer{}
			app := NewApp(stdout, stderr, tt.client)
			err := app.Run(tt.args)
			if tt.expectedError != "" {
				assert.EqualError(t, err, tt.expectedError)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.expected, stdout.String())
			assert.Empty(t, stderr.String())
		})
	}
}

func TestCreateAutoscale(t *testing.T) {
	tests := []struct {
		name          string
		args          []string
		expected      string
		expectedError string
		client        client.Client
	}{
		{
			name:          "when Create Autoscale does not find the instance",
			args:          []string{"./rpaasv2", "autoscale", "add", "-s", "my-service", "-i", "my-instance", "--max", "5", "--min", "2", "--cpu", "50", "--memory", "45"},
			expectedError: "not found error",
			client: &fake.FakeClient{
				FakeCreateAutoscale: func(args client.CreateAutoscaleArgs) (*http.Response, error) {
					require.Equal(t, args.Instance, "my-instance")
					require.Equal(t, args.MaxReplicas, int32(5))
					require.Equal(t, args.MinReplicas, int32(2))
					require.Equal(t, args.CPU, int32(50))
					require.Equal(t, args.Memory, int32(45))
					return nil, fmt.Errorf("not found error")
				},
			},
		},
		{
			name: "when Create Autoscale is successful",
			args: []string{"./rpaasv2", "autoscale", "add", "-s", "my-service", "-i", "my-instance", "--max", "5", "--min", "2", "--cpu", "50", "--memory", "45"},
			client: &fake.FakeClient{
				FakeCreateAutoscale: func(args client.CreateAutoscaleArgs) (*http.Response, error) {
					require.Equal(t, args.Instance, "my-instance")
					require.Equal(t, args.MaxReplicas, int32(5))
					require.Equal(t, args.MinReplicas, int32(2))
					require.Equal(t, args.CPU, int32(50))
					require.Equal(t, args.Memory, int32(45))
					return &http.Response{
						Status:     "200 OK!",
						StatusCode: http.StatusOK,
						Proto:      "HTTP/1.0",
					}, nil
				},
			},
			expected: "Autoscale of my-service/my-instance successfuly created\n",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			stdout := &bytes.Buffer{}
			stderr := &bytes.Buffer{}
			app := NewApp(stdout, stderr, tt.client)
			err := app.Run(tt.args)
			if tt.expectedError != "" {
				assert.EqualError(t, err, tt.expectedError)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.expected, stdout.String())
			assert.Empty(t, stderr.String())
		})
	}
}

func TestUpdateAutoscale(t *testing.T) {
	tests := []struct {
		name          string
		args          []string
		expected      string
		expectedError string
		client        client.Client
	}{
		{
			name:          "when Update Autoscale does not find the instance",
			args:          []string{"./rpaasv2", "autoscale", "update", "-s", "my-service", "-i", "my-instance", "--max", "5", "--min", "2", "--cpu", "50", "--memory", "45"},
			expectedError: "not found error",
			client: &fake.FakeClient{
				FakeUpdateAutoscale: func(args client.UpdateAutoscaleArgs) (*http.Response, error) {
					require.Equal(t, args.Instance, "my-instance")
					require.Equal(t, args.MaxReplicas, int32(5))
					require.Equal(t, args.MinReplicas, int32(2))
					require.Equal(t, args.CPU, int32(50))
					require.Equal(t, args.Memory, int32(45))
					return nil, fmt.Errorf("not found error")
				},
			},
		},
		{
			name: "when Update Autoscale is successful",
			args: []string{"./rpaasv2", "autoscale", "update", "-s", "my-service", "-i", "my-instance", "--max", "5", "--min", "2", "--cpu", "50", "--memory", "45"},
			client: &fake.FakeClient{
				FakeUpdateAutoscale: func(args client.UpdateAutoscaleArgs) (*http.Response, error) {
					require.Equal(t, args.Instance, "my-instance")
					require.Equal(t, args.MaxReplicas, int32(5))
					require.Equal(t, args.MinReplicas, int32(2))
					require.Equal(t, args.CPU, int32(50))
					require.Equal(t, args.Memory, int32(45))
					return &http.Response{
						StatusCode: http.StatusCreated,
						Proto:      "HTTP/1.0",
					}, nil
				},
			},
			expected: "Autoscale of my-service/my-instance successfuly updated\n",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			stdout := &bytes.Buffer{}
			stderr := &bytes.Buffer{}
			app := NewApp(stdout, stderr, tt.client)
			err := app.Run(tt.args)
			if tt.expectedError != "" {
				assert.EqualError(t, err, tt.expectedError)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.expected, stdout.String())
			assert.Empty(t, stderr.String())
		})
	}
}

func TestRemoveAutoscale(t *testing.T) {
	tests := []struct {
		name          string
		args          []string
		expected      string
		expectedError string
		client        client.Client
	}{
		{
			name:          "when Remove Autoscale does not find the instance",
			args:          []string{"./rpaasv2", "autoscale", "remove", "-s", "my-service", "-i", "my-instance"},
			expectedError: "not found error",
			client: &fake.FakeClient{
				FakeRemoveAutoscale: func(args client.RemoveAutoscaleArgs) (*http.Response, error) {
					require.Equal(t, args.Instance, "my-instance")
					return nil, fmt.Errorf("not found error")
				},
			},
		},
		{
			name: "when remove autoscale route is successful",
			args: []string{"./rpaasv2", "autoscale", "remove", "-s", "my-service", "-i", "my-instance"},
			client: &fake.FakeClient{
				FakeRemoveAutoscale: func(args client.RemoveAutoscaleArgs) (*http.Response, error) {
					require.Equal(t, args.Instance, "my-instance")
					return &http.Response{
						Status:     "200 OK!",
						StatusCode: http.StatusOK,
						Proto:      "HTTP/1.0",
					}, nil
				},
			},
			expected: "Autoscale of my-service/my-instance successfuly removed\n",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			stdout := &bytes.Buffer{}
			stderr := &bytes.Buffer{}
			app := NewApp(stdout, stderr, tt.client)
			err := app.Run(tt.args)
			if tt.expectedError != "" {
				assert.EqualError(t, err, tt.expectedError)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.expected, stdout.String())
			assert.Empty(t, stderr.String())
		})
	}
}
