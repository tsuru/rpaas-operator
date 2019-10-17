// Copyright 2019 tsuru authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package client

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"net/url"
	"strconv"
	"strings"
)

type RpaasClient struct {
	hostAPI      string
	tsuruTarget  string
	tsuruToken   string
	target       string
	tsuruService string
	httpClient   *http.Client
}

func New(hostAPI string) *RpaasClient {
	client := &RpaasClient{httpClient: &http.Client{}, hostAPI: hostAPI}
	return client
}

func NewTsuruClient(tsuruAPI, service, token string) (*RpaasClient, error) {
	if tsuruAPI == "" || token == "" || service == "" {
		return nil, fmt.Errorf("cannot create client without either tsuru target, token or service")
	}
	return &RpaasClient{
		httpClient:   &http.Client{},
		tsuruTarget:  tsuruAPI,
		tsuruToken:   token,
		tsuruService: service,
	}, nil
}

func (c *RpaasClient) GetPlans(ctx context.Context, instance *string) ([]Plan, error) {

	var pathName string
	var req *http.Request
	var err error
	switch instance {
	case nil:
		pathName = fmt.Sprintf("/resources/plans")
		req, err = c.newRequest("GET", "", pathName, nil)
		if err != nil {
			return nil, err
		}
	default:
		pathName = fmt.Sprintf("/resources/%s/plans", *instance)
		req, err = c.newRequest("GET", *instance, pathName, nil)
		if err != nil {
			return nil, err
		}
	}

	if err != nil {
		return nil, err
	}

	resp, err := c.do(ctx, req)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode == http.StatusOK {
		body, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			return nil, fmt.Errorf("Error while trying to read body: %v", err)
		}
		var plans []Plan
		err = json.Unmarshal(body, &plans)
		if err != nil {
			return nil, err
		}
		return plans, nil
	}
	bodyString, err := getBodyString(resp)
	if err != nil {
		return nil, err
	}
	return nil, fmt.Errorf("unexpected status code: body: %v", bodyString)
}

func (c *RpaasClient) GetFlavors(ctx context.Context, instance *string) ([]Flavor, error) {
	var pathName string
	var req *http.Request
	var err error

	switch instance {
	case nil:
		pathName = fmt.Sprintf("/resources/flavors")
		req, err = c.newRequest("GET", "", pathName, nil)
		if err != nil {
			return nil, err
		}
	default:
		pathName = fmt.Sprintf("/resources/%s/flavors", *instance)
		req, err = c.newRequest("GET", "", pathName, nil)
		if err != nil {
			return nil, err
		}
	}

	if err != nil {
		return nil, err
	}

	resp, err := c.do(ctx, req)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode == http.StatusOK {
		body, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			return nil, fmt.Errorf("Error while trying to read body: %v", err)
		}
		var flavors []Flavor
		err = json.Unmarshal(body, &flavors)
		if err != nil {
			return nil, err
		}
		return flavors, nil
	}
	bodyString, err := getBodyString(resp)
	if err != nil {
		return nil, err
	}
	return nil, fmt.Errorf("unexpected status code: body: %v", bodyString)
}

func (c *RpaasClient) Scale(ctx context.Context, instance string, replicas int32) error {
	if err := scaleValidate(instance, replicas); err != nil {
		return err
	}

	bodyStruct := url.Values{}
	bodyStruct.Set("quantity", strconv.Itoa(int(replicas)))

	pathName := fmt.Sprintf("/resources/%s/scale", instance)
	bodyReader := strings.NewReader(bodyStruct.Encode())
	req, err := c.newRequest("POST", instance, pathName, bodyReader)
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

func (c *RpaasClient) newRequest(method, instance, pathName string, body io.Reader) (*http.Request, error) {
	var url string
	if c.tsuruTarget != "" {
		if instance == "" {
			url = fmt.Sprintf("%s/services/proxy/%s?callback=%s",
				c.tsuruTarget, c.tsuruService, pathName)
		} else {
			url = fmt.Sprintf("%s/services/%s/proxy/%s?callback=%s",
				c.tsuruTarget, c.tsuruService, instance, pathName)
		}
	} else {
		url = fmt.Sprintf("%s%s", c.hostAPI, pathName)
	}

	req, err := http.NewRequest(method, url, body)
	if err != nil {
		return nil, err
	}

	if c.tsuruToken != "" {
		req.Header.Add("Authorization", "Bearer "+c.tsuruToken)
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
