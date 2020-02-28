// Copyright 2019 tsuru authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package api

import (
	"bytes"
	"io/ioutil"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/tsuru/rpaas-operator/internal/pkg/rpaas"
)

const boundary = "XXXXXXXXXXXX"

type multipartFile struct {
	filename string
	content  string
}

func newMultipartFormBody(name string, files ...multipartFile) (string, error) {
	b := &bytes.Buffer{}
	w := multipart.NewWriter(b)
	w.SetBoundary(boundary)
	for _, f := range files {
		writer, err := w.CreateFormFile(name, f.filename)
		if err != nil {
			return "", err
		}
		if _, err = writer.Write([]byte(f.content)); err != nil {
			return "", err
		}
	}
	w.Close()
	return b.String(), nil
}

func newTestingServer(t *testing.T, m rpaas.RpaasManager) *httptest.Server {
	webApi, err := New(nil)
	require.NoError(t, err)
	webApi.rpaasManager = m
	return httptest.NewServer(webApi.Handler())
}

func bodyContent(rsp *http.Response) string {
	data, _ := ioutil.ReadAll(rsp.Body)
	return strings.TrimSuffix(string(data), "\n")
}
