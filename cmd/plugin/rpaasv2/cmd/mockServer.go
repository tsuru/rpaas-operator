// Copyright 2019 tsuru authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package cmd

import (
	"net/http/httptest"
)

type mockServer struct {
	ts            *httptest.Server
	getURLfunc    func(string) (string, error)
	getTargetFunc func() (string, error)
	readTokenFunc func() (string, error)
}

func (ms *mockServer) GetURL(path string) (string, error) {
	if ms.getURLfunc == nil {
		return ms.ts.URL + path, nil
	}
	return ms.getURLfunc(path)
}

func (ms *mockServer) GetTarget() (string, error) {
	if ms.getTargetFunc == nil {
		return "", nil
	}
	return ms.getTargetFunc()
}

func (ms *mockServer) ReadToken() (string, error) {
	if ms.readTokenFunc == nil {
		return "", nil
	}
	return ms.readTokenFunc()
}
