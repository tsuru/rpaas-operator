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
	httpClient *http.Client
}

func New(hostAPI string) *RpaasClient {
	client := &RpaasClient{httpClient: &http.Client{}, hostAPI: hostAPI}
	return client
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
	urlString := fmt.Sprintf("%s%s", c.hostAPI, pathName)

	return http.NewRequest("POST", urlString, body)
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
