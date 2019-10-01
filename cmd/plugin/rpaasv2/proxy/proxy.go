// Copyright 2019 tsuru authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package proxy

import (
	"io"
	"net/http"
)

type Proxy struct {
	ServiceName  string
	InstanceName string
	Path         string
	Body         io.Reader
	Headers      map[string]string
	Method       string
	Server       Server
}

func (p *Proxy) ProxyRequest() (*http.Response, error) {
	_, err := p.Server.GetTarget()
	if err != nil {
		return nil, err
	}
	token, err := p.Server.ReadToken()

	if err != nil {
		return nil, err
	}
	url, err := p.Server.GetURL("/services/" + p.ServiceName + "/proxy/" + p.InstanceName + "?callback=" + p.Path)
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequest("GET", url, p.Body)
	if err != nil {
		return nil, err
	}
	req.Header.Add("Authorization", "bearer "+token)

	if p.Headers != nil {
		for key, value := range p.Headers {
			req.Header.Add(key, value)
		}
	}
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}

	return resp, nil
}
