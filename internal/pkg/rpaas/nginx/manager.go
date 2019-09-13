// Copyright 2019 tsuru authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package nginx

import (
	"fmt"
	"net/http"
	"time"

	"github.com/sirupsen/logrus"
)

const (
	defaultManagePort         = 8800
	defaultPurgeTimeout       = time.Duration(1 * time.Second)
	defaultPurgeLocation      = "/purge"
	defaultPurgeLocationMatch = "^/purge/(.+)"
	defaultVTSLocationMatch   = "/status"
)

type NginxManager struct {
	managePort    uint16
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
		managePort:    managePort(),
		purgeLocation: purgeLocationMatch(),
		client:        http.Client{Timeout: defaultPurgeTimeout},
	}
}

func managePort() uint16 {
	return defaultManagePort
}

func purgeLocationMatch() string {
	return defaultPurgeLocationMatch
}

func vtsLocationMatch() string {
	return defaultVTSLocationMatch
}

func (m NginxManager) PurgeCache(host, purgePath string, preservePath bool) error {
	for _, encoding := range []string{"gzip", "identity"} {
		headers := map[string]string{"Accept-Encoding": encoding}

		if preservePath {
			path := fmt.Sprintf("%s%s", defaultPurgeLocation, purgePath)
			if err := m.purgeRequest(host, path, headers); err != nil {
				return err
			}
		} else {
			for _, scheme := range []string{"http", "https"} {
				path := fmt.Sprintf("%s/%s%s", defaultPurgeLocation, scheme, purgePath)
				if err := m.purgeRequest(host, path, headers); err != nil {
					return err
				}
			}
		}
	}
	return nil
}

func (m NginxManager) purgeRequest(host, path string, headers map[string]string) error {
	resp, err := m.requestNginx(host, path, headers)
	if err != nil {
		errorMessage := fmt.Sprintf("cannot purge nginx cache - error requesting nginx server: %v", err)
		logrus.Error(errorMessage)
		return NginxError{Msg: errorMessage}
	}
	if resp.StatusCode != http.StatusOK {
		errorMessage := fmt.Sprintf("cannot purge nginx cache - unexpected response from nginx server: %v", resp)
		logrus.Error(errorMessage)
		return NginxError{Msg: errorMessage}
	}
	return nil
}

func (m NginxManager) requestNginx(host, path string, headers map[string]string) (*http.Response, error) {
	req, err := http.NewRequest("GET", fmt.Sprintf("http://%s:%d%s", host, m.managePort, path), nil)
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
