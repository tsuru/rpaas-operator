// Copyright 2022 tsuru authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package cmd

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	rpaasclient "github.com/tsuru/rpaas-operator/pkg/rpaas/client"
	"github.com/tsuru/rpaas-operator/pkg/rpaas/client/fake"
	"github.com/tsuru/rpaas-operator/pkg/rpaas/client/types"
)

func TestDeleteExtraFiles(t *testing.T) {
	tests := []struct {
		name          string
		args          []string
		assertion     func(t *testing.T, stdout, stderr *bytes.Buffer, err error)
		expectedError string
		client        rpaasclient.Client
	}{
		{
			name:          "when DeleteExtraFiles returns an error",
			args:          []string{"./rpaasv2", "extra-files", "delete", "-i", "my-instance", "--files", "f1"},
			expectedError: "some error",
			assertion: func(t *testing.T, stdout, stderr *bytes.Buffer, err error) {
				assert.Error(t, err)
				assert.EqualError(t, err, "some error")
			},
			client: &fake.FakeClient{
				FakeDeleteExtraFiles: func(args rpaasclient.DeleteExtraFilesArgs) error {
					expected := rpaasclient.DeleteExtraFilesArgs{
						Instance: "my-instance",
						Files:    []string{"f1"},
					}
					assert.Equal(t, expected, args)
					return fmt.Errorf("some error")
				},
			},
		},
		{
			name: "when DeleteExtraFiles returns no error",
			args: []string{"./rpaasv2", "extra-files", "delete", "-i", "my-instance", "--files", "f1", "--files", "f2"},
			assertion: func(t *testing.T, stdout, stderr *bytes.Buffer, err error) {
				require.NoError(t, err)
				s1 := "Removed [f1, f2] from my-instance\n"
				s2 := "Removed [f2, f1] from my-instance\n"
				stdoutString := stdout.String()
				if s1 != stdoutString {
					assert.Equal(t, s2, stdoutString)
				} else {
					// this is just to force the assertion of both strings instead of just one
					assert.Equal(t, s1, stdoutString)
				}
				assert.Empty(t, stderr.String())
			},
			client: &fake.FakeClient{
				FakeDeleteExtraFiles: func(args rpaasclient.DeleteExtraFilesArgs) error {
					expected := rpaasclient.DeleteExtraFilesArgs{
						Instance: "my-instance",
						Files:    []string{"f1", "f2"},
					}
					assert.Equal(t, expected, args)
					return nil
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			stdout := &bytes.Buffer{}
			stderr := &bytes.Buffer{}
			app := NewApp(stdout, stderr, tt.client)
			err := app.Run(tt.args)
			tt.assertion(t, stdout, stderr, err)
		})
	}
}

func TestAddExtraFiles(t *testing.T) {
	c1 := "content1"
	c2 := "content2"
	f1, err := ioutil.TempFile("", "f1")
	require.NoError(t, err)
	f2, err := ioutil.TempFile("", "f2")
	require.NoError(t, err)
	_, err = f1.Write([]byte(c1))
	require.NoError(t, err)
	require.NoError(t, f1.Close())
	_, err = f2.Write([]byte(c2))
	require.NoError(t, err)
	require.NoError(t, f2.Close())
	defer func() {
		os.Remove(f1.Name())
		os.Remove(f2.Name())
	}()
	tests := []struct {
		name      string
		args      []string
		assertion func(t *testing.T, stdout, stderr *bytes.Buffer, err error, f1Name, f2Name string)
		client    rpaasclient.Client
	}{
		{
			name: "when AddExtraFiles returns an error",
			args: []string{"./rpaasv2", "extra-files", "add", "-i", "my-instance", "--files", f1.Name()},
			assertion: func(t *testing.T, stdout, stderr *bytes.Buffer, err error, f1Name, f2Name string) {
				assert.Error(t, err)
				assert.EqualError(t, err, "some error")
			},
			client: &fake.FakeClient{
				FakeAddExtraFiles: func(args rpaasclient.ExtraFilesArgs) error {
					expected := rpaasclient.ExtraFilesArgs{
						Instance: "my-instance",
						Files:    map[string][]byte{f1.Name(): []byte(c1)},
					}
					assert.Equal(t, expected, args)
					return fmt.Errorf("some error")
				},
			},
		},
		{
			name: "when AddExtraFiles returns no error",
			args: []string{"./rpaasv2", "extra-files", "add", "-i", "my-instance", "--files", f1.Name(), "--files", f2.Name()},
			assertion: func(t *testing.T, stdout, stderr *bytes.Buffer, err error, f1Name, f2Name string) {
				require.NoError(t, err)
				s1 := fmt.Sprintf("Added [%s, %s] to my-instance\n", f1Name, f2Name)
				s2 := fmt.Sprintf("Added [%s, %s] to my-instance\n", f2Name, f1Name)
				stdoutString := stdout.String()
				if s1 != stdoutString {
					assert.Equal(t, s2, stdoutString)
				} else {
					// this is just to force the assertion of both strings instead of just one assertion + if condition
					assert.Equal(t, s1, stdoutString)
				}
				assert.Empty(t, stderr.String())
			},
			client: &fake.FakeClient{
				FakeAddExtraFiles: func(args rpaasclient.ExtraFilesArgs) error {
					expected := rpaasclient.ExtraFilesArgs{
						Instance: "my-instance",
						Files:    map[string][]byte{f1.Name(): []byte(c1), f2.Name(): []byte(c2)},
					}
					assert.Equal(t, expected, args)
					return nil
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			stdout := &bytes.Buffer{}
			stderr := &bytes.Buffer{}
			app := NewApp(stdout, stderr, tt.client)
			err := app.Run(tt.args)
			tt.assertion(t, stdout, stderr, err, f1.Name(), f2.Name())
		})
	}
}

func TestUpdateExtraFiles(t *testing.T) {
	c1 := "content1"
	c2 := "content2"
	f1, err := ioutil.TempFile("", "f1")
	require.NoError(t, err)
	f2, err := ioutil.TempFile("", "f2")
	require.NoError(t, err)
	_, err = f1.Write([]byte(c1))
	require.NoError(t, err)
	require.NoError(t, f1.Close())
	_, err = f2.Write([]byte(c2))
	require.NoError(t, err)
	require.NoError(t, f2.Close())
	defer func() {
		os.Remove(f1.Name())
		os.Remove(f2.Name())
	}()
	tests := []struct {
		name      string
		args      []string
		assertion func(t *testing.T, stdout, stderr *bytes.Buffer, err error, f1Name, f2Name string)
		client    rpaasclient.Client
	}{
		{
			name: "when UpdateExtraFiles returns an error",
			args: []string{"./rpaasv2", "extra-files", "update", "-i", "my-instance", "--files", f1.Name()},
			assertion: func(t *testing.T, stdout, stderr *bytes.Buffer, err error, f1Name, f2Name string) {
				assert.Error(t, err)
				assert.EqualError(t, err, "some error")
			},
			client: &fake.FakeClient{
				FakeUpdateExtraFiles: func(args rpaasclient.ExtraFilesArgs) error {
					expected := rpaasclient.ExtraFilesArgs{
						Instance: "my-instance",
						Files:    map[string][]byte{f1.Name(): []byte(c1)},
					}
					assert.Equal(t, expected, args)
					return fmt.Errorf("some error")
				},
			},
		},
		{
			name: "when Update returns no error",
			args: []string{"./rpaasv2", "extra-files", "update", "-i", "my-instance", "--files", f1.Name(), "--files", f2.Name()},
			assertion: func(t *testing.T, stdout, stderr *bytes.Buffer, err error, f1Name, f2Name string) {
				require.NoError(t, err)
				s1 := fmt.Sprintf("Updated [%s, %s] on my-instance\n", f1Name, f2Name)
				s2 := fmt.Sprintf("Updated [%s, %s] on my-instance\n", f2Name, f1Name)
				stdoutString := stdout.String()
				if s1 != stdoutString {
					assert.Equal(t, s2, stdoutString)
				} else {
					// this is just to force the assertion of both strings instead of just one assertion + if condition
					assert.Equal(t, s1, stdoutString)
				}
				assert.Empty(t, stderr.String())
			},
			client: &fake.FakeClient{
				FakeUpdateExtraFiles: func(args rpaasclient.ExtraFilesArgs) error {
					expected := rpaasclient.ExtraFilesArgs{
						Instance: "my-instance",
						Files:    map[string][]byte{f1.Name(): []byte(c1), f2.Name(): []byte(c2)},
					}
					assert.Equal(t, expected, args)
					return nil
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			stdout := &bytes.Buffer{}
			stderr := &bytes.Buffer{}
			app := NewApp(stdout, stderr, tt.client)
			err := app.Run(tt.args)
			tt.assertion(t, stdout, stderr, err, f1.Name(), f2.Name())
		})
	}
}

func TestListExtraFiles(t *testing.T) {
	counter := 0
	tests := []struct {
		name      string
		args      []string
		assertion func(t *testing.T, stdout, stderr *bytes.Buffer, err error)
		client    rpaasclient.Client
	}{
		{
			name: "when ListExtraFiles returns an error",
			args: []string{"./rpaasv2", "extra-files", "list", "-i", "my-instance"},
			assertion: func(t *testing.T, stdout, stderr *bytes.Buffer, err error) {
				assert.Error(t, err)
				assert.EqualError(t, err, "some error")
			},
			client: &fake.FakeClient{
				FakeListExtraFiles: func(instance string) ([]string, error) {
					expectedInstance := "my-instance"
					assert.Equal(t, expectedInstance, instance)
					return nil, fmt.Errorf("some error")
				},
			},
		},
		{
			name: "when ListExtraFiles returns no error",
			args: []string{"./rpaasv2", "extra-files", "list", "-i", "my-instance"},
			assertion: func(t *testing.T, stdout, stderr *bytes.Buffer, err error) {
				require.NoError(t, err)
				s1 := "f1\nf2\n"
				s2 := "f2\nf1\n"
				stdoutString := stdout.String()
				if s1 != stdoutString {
					assert.Equal(t, s2, stdoutString)
				} else {
					// this is just to force the assertion of both strings instead of just one assertion + if condition
					assert.Equal(t, s1, stdoutString)
				}
				assert.Empty(t, stderr.String())
			},
			client: &fake.FakeClient{
				FakeListExtraFiles: func(instance string) ([]string, error) {
					expectedInstance := "my-instance"
					assert.Equal(t, expectedInstance, instance)
					return []string{"f1", "f2"}, nil
				},
			},
		},
		{
			name: "when ListExtraFiles returns no error and --show-content",
			args: []string{"./rpaasv2", "extra-files", "list", "-i", "my-instance", "--show-content"},
			assertion: func(t *testing.T, stdout, stderr *bytes.Buffer, err error) {
				require.NoError(t, err)
				s1 := `+------+----------------+
| Name |    Content     |
+------+----------------+
| f1   | some content 1 |
+------+----------------+
| f2   | some content 2 |
+------+----------------+
`
				s2 := `+------+----------------+
| Name |    Content     |
+------+----------------+
| f2   | some content 2 |
+------+----------------+
| f1   | some content 1 |
+------+----------------+
`
				stdoutString := stdout.String()
				if s1 != stdoutString {
					assert.Equal(t, s2, stdoutString)
				} else {
					// this is just to force the assertion of both strings instead of just one assertion + if condition
					assert.Equal(t, s1, stdoutString)
				}
				assert.Empty(t, stderr.String())
			},
			client: &fake.FakeClient{
				FakeListExtraFiles: func(instance string) ([]string, error) {
					expectedInstance := "my-instance"
					assert.Equal(t, expectedInstance, instance)
					return []string{"f1", "f2"}, nil
				},
				FakeGetExtraFile: func(args rpaasclient.GetExtraFileArgs) (types.RpaasFile, error) {
					counter++
					switch counter {
					case 1:
						return types.RpaasFile{
							Name:    "f1",
							Content: []byte("some content 1"),
						}, nil
					case 2:
						return types.RpaasFile{
							Name:    "f2",
							Content: []byte("some content 2"),
						}, nil
					}
					return types.RpaasFile{}, nil
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			stdout := &bytes.Buffer{}
			stderr := &bytes.Buffer{}
			app := NewApp(stdout, stderr, tt.client)
			err := app.Run(tt.args)
			tt.assertion(t, stdout, stderr, err)
		})
	}
}

func TestGetExtraFile(t *testing.T) {
	tests := []struct {
		name          string
		args          []string
		expected      string
		expectedError string
		client        rpaasclient.Client
	}{
		{
			name:          "when GetExtraFile returns an error",
			args:          []string{"./rpaasv2", "extra-files", "get", "-i", "my-instance", "--file", "f1"},
			expectedError: "some error",
			client: &fake.FakeClient{
				FakeGetExtraFile: func(args rpaasclient.GetExtraFileArgs) (types.RpaasFile, error) {
					expectedFileName := "f1"
					expectedInstance := "my-instance"
					assert.Equal(t, expectedFileName, args.FileName)
					assert.Equal(t, expectedInstance, args.Instance)
					return types.RpaasFile{}, fmt.Errorf("some error")
				},
			},
		},
		{
			name:     "when GetExtraFile returns no error",
			args:     []string{"./rpaasv2", "extra-files", "get", "-i", "my-instance", "--file", "f1"},
			expected: "some content\n",
			client: &fake.FakeClient{
				FakeGetExtraFile: func(args rpaasclient.GetExtraFileArgs) (types.RpaasFile, error) {
					expectedFileName := "f1"
					expectedInstance := "my-instance"
					assert.Equal(t, expectedFileName, args.FileName)
					assert.Equal(t, expectedInstance, args.Instance)
					return types.RpaasFile{
						Name:    args.FileName,
						Content: []byte("some content"),
					}, nil
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			stdout := &bytes.Buffer{}
			stderr := &bytes.Buffer{}
			app := NewApp(stdout, stderr, tt.client)
			err := app.Run(tt.args)
			if tt.expectedError != "" {
				assert.Error(t, err)
				assert.EqualError(t, err, tt.expectedError)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.expected, stdout.String())
			assert.Empty(t, stderr.String())
		})
	}
}
