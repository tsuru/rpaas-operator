// Copyright 2019 tsuru authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package nginx

import (
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/sirupsen/logrus"
)

const (
	PortNameHTTP  = "http"
	PortNameHTTPS = "https"

	PortNameProxyProtocolHTTP  = "proxy-protocol-http"
	PortNameProxyProtocolHTTPS = "proxy-protocol-https"
	PortNameMetrics            = "nginx-metrics"
	PortNameManagement         = PortNameMetrics

	DefaultManagePort         = 8800
	defaultPurgeTimeout       = time.Duration(1 * time.Second)
	defaultPurgeLocation      = "/purge"
	defaultPurgeLocationMatch = "^/purge/(.+)"
	defaultVTSLocationMatch   = "/status"
)

type NginxManager struct {
	purgeLocation string
	client        http.Client
}

type NginxError struct {
	Msg string
}

func (e NginxError) Error() string {
	return e.Msg
}

func NewNginxManager() NginxManager {
	return NginxManager{
		purgeLocation: purgeLocationMatch(),
		client:        http.Client{Timeout: defaultPurgeTimeout},
	}
}

func purgeLocationMatch() string {
	return defaultPurgeLocationMatch
}

func vtsLocationMatch() string {
	return defaultVTSLocationMatch
}

func (m NginxManager) PurgeCache(host, purgePath string, port int32, preservePath bool, extraHeaders http.Header) (bool, error) {
	purged := false
	for _, encoding := range []string{"gzip", "identity"} {
		var err error
		status := false
		headers := map[string]string{"Accept-Encoding": encoding}
		if len(extraHeaders) > 0 {
			for header := range extraHeaders {
				headers[header] = extraHeaders.Get(header)
				status, err = m.purge(host, purgePath, port, preservePath, headers)
				if err != nil {
					return false, err
				}
			}
		} else {
			status, err = m.purge(host, purgePath, port, preservePath, headers)
			if err != nil {
				return false, err
			}
		}
		if status {
			purged = true
		}
	}
	return purged, nil
}

func (m NginxManager) purge(host, purgePath string, port int32, preservePath bool, headers map[string]string) (bool, error) {
	separator := "/"
	if strings.HasPrefix(purgePath, "/") {
		separator = ""
	}
	if preservePath {
		path := fmt.Sprintf("%s%s%s", defaultPurgeLocation, separator, purgePath)
		status, err := m.purgeRequest(host, path, port, headers)
		if err != nil {
			return status, err
		}
		return status, nil
	} else {
		purged := false
		for _, scheme := range []string{"http", "https"} {
			path := fmt.Sprintf("%s/%s%s%s", defaultPurgeLocation, scheme, separator, purgePath)
			status, err := m.purgeRequest(host, path, port, headers)
			if err != nil {
				return status, err
			}
			if status {
				purged = true
			}
		}
		return purged, nil
	}
}

func (m NginxManager) purgeRequest(host, path string, port int32, headers map[string]string) (bool, error) {
	resp, err := m.requestNginx(host, path, port, headers)
	if err != nil {
		errorMessage := fmt.Sprintf("cannot purge nginx cache - error requesting nginx server: %v", err)
		logrus.Error(errorMessage)
		return false, NginxError{Msg: errorMessage}
	}
	switch resp.StatusCode {
	case http.StatusOK:
		return true, nil
	case http.StatusNotFound:
		return false, nil
	default:
		errorMessage := fmt.Sprintf("cannot purge nginx cache - unexpected response from nginx server: %d", resp.StatusCode)
		logrus.Error(errorMessage)
		return false, NginxError{Msg: errorMessage}
	}
}

func (m NginxManager) requestNginx(host, path string, port int32, headers map[string]string) (*http.Response, error) {
	if port == 0 {
		port = DefaultManagePort
	}
	req, err := http.NewRequest("GET", fmt.Sprintf("http://%s:%d%s", host, port, path), nil)
	if err != nil {
		return nil, err
	}
	for key, value := range headers {
		req.Header.Add(key, value)
	}
	resp, err := m.client.Do(req)
	if err != nil {
		return nil, err
	}
	return resp, nil
}
