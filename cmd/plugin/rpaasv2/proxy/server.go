// Copyright 2019 tsuru authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package proxy

import tsuruCmd "github.com/tsuru/tsuru/cmd"

type Server interface {
	GetTarget() (string, error)
	GetURL(path string) (string, error)
	ReadToken() (string, error)
}

type TsuruServer struct{}

func (t *TsuruServer) GetTarget() (string, error) {
	return tsuruCmd.GetTarget()
}

func (t *TsuruServer) GetURL(path string) (string, error) {
	return tsuruCmd.GetURL(path)
}

func (t *TsuruServer) ReadToken() (string, error) {
	return tsuruCmd.ReadToken()
}
