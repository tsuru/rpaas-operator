// Copyright 2019 tsuru authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package client

import (
	"context"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"net/url"
	"strconv"
	"strings"
)

type RpaasClient struct {
	hostAPI    string
	tsuruAPI   string
	token      string
	target     string
	service    string
	instance   string
	httpClient *http.Client
}

func New(hostAPI string) *RpaasClient {
	client := &RpaasClient{httpClient: &http.Client{}, hostAPI: hostAPI}
	return client
}

func NewTsuruClient(tsuruAPI, service, instance, token string) *RpaasClient {
	return &RpaasClient{
		httpClient: &http.Client{},
		tsuruAPI:   tsuruAPI,
		token:      token,
		service:    service,
		instance:   instance,
	}
}

func (c *RpaasClient) Scale(ctx context.Context, instance string, replicas int32) error {
	if err := scaleValidate(instance, replicas); err != nil {
		return err
	}

	bodyStruct := url.Values{}
	bodyStruct.Set("quantity", strconv.Itoa(int(replicas)))

	pathName := fmt.Sprintf("/resources/%s/scale", instance)
	bodyReader := strings.NewReader(bodyStruct.Encode())
	req, err := c.newRequest("POST", pathName, bodyReader)
	if err != nil {
		return err
	}
	req.Header.Add("Content-Type", "application/x-www-form-urlencoded")

	resp, err := c.do(ctx, req)
	if err != nil {
		return err
	}
	if resp.StatusCode == http.StatusCreated {
		return nil
	}

	bodyString, err := getBodyString(resp)
	if err != nil {
		return err
	}
	return fmt.Errorf("unexpected status code: body: %v", bodyString)
}

func (c *RpaasClient) do(ctx context.Context, req *http.Request) (*http.Response, error) {
	req = req.WithContext(ctx)
	return c.httpClient.Do(req)
}

func (c *RpaasClient) newRequest(method, pathName string, body io.Reader) (*http.Request, error) {
	var url string
	if c.tsuruAPI != "" {
		url = fmt.Sprintf("%s/services/%s/proxy/%s?callback=%s",
			c.tsuruAPI, c.service, c.instance, pathName)

	} else {
		url = fmt.Sprintf("%s%s", c.hostAPI, pathName)
	}

	req, err := http.NewRequest(method, url, body)
	if err != nil {
		return nil, err
	}

	if c.token != "" {
		req.Header.Add("Authorization", "Bearer "+c.token)
	}

	return req, nil
}

func scaleValidate(instance string, replicas int32) error {
	if instance == "" {
		return fmt.Errorf("instance can't be nil")
	}

	if replicas < 0 {
		return fmt.Errorf("replicas number must be greater or equal to zero")
	}

	return nil
}

func getBodyString(resp *http.Response) (string, error) {
	bodyBytes, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("unable to read body response: %v", err)
	}
	defer resp.Body.Close()
	return string(bodyBytes), nil
}
