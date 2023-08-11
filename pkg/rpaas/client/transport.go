// Copyright 2023 tsuru authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package client

import (
	"encoding/base64"
	"fmt"
	"net/http"
)

var _ http.RoundTripper = (*BasicAuthTransport)(nil)

type BasicAuthTransport struct {
	Username string
	Password string
	Base     http.RoundTripper
}

func (bat BasicAuthTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	req.Header.Set("Authorization", fmt.Sprintf("Basic %s",
		base64.StdEncoding.EncodeToString([]byte(fmt.Sprintf("%s:%s", bat.Username, bat.Password)))))

	base := bat.Base
	if base == nil {
		base = http.DefaultTransport
	}

	return base.RoundTrip(req)
}
