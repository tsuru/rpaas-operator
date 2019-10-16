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
	"os"
	"strconv"
	"strings"

	"github.com/olekukonko/tablewriter"
)

type RpaasClient struct {
	hostAPI    string
	tsuruAPI   string
	token      string
	target     string
	service    string
	httpClient *http.Client
}

func New(hostAPI string) *RpaasClient {
	client := &RpaasClient{httpClient: &http.Client{}, hostAPI: hostAPI}
	return client
}

func NewTsuruClient(tsuruAPI, service, token string) (*RpaasClient, error) {
	if service == "" || tsuruAPI == "" {
		return nil, fmt.Errorf("service can't be nil")
	}
	return &RpaasClient{
		httpClient: &http.Client{},
		tsuruAPI:   tsuruAPI,
		token:      token,
		service:    service,
	}, nil
}

func (c *RpaasClient) Info(ctx context.Context, instance string, infoType string) error {
	if instance == "" {
		return fmt.Errorf("instance can't be nil")
	}

	pathName := fmt.Sprintf("/resources/%s/info", instance)
	req, err := c.newRequest("GET", instance, pathName, nil)
	if err != nil {
		return err
	}

	resp, err := c.do(ctx, req)
	if err != nil {
		return err
	}

	if resp.StatusCode == http.StatusOK {
		body, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			return fmt.Errorf("Error while trying to read body: %v", err)
		}
		var fp interface{}
		err = json.Unmarshal(body, &fp)
		if err != nil {
			return err
		}
		helperSlice := fp.([]interface{})
		WriteInfo(infoType, helperSlice)
		return nil
	}

	bodyString, err := getBodyString(resp)
	if err != nil {
		return err
	}
	return fmt.Errorf("unexpected status code: body: %v", bodyString)
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
	if c.tsuruAPI != "" {
		url = fmt.Sprintf("%s/services/%s/proxy/%s?callback=%s",
			c.tsuruAPI, c.service, instance, pathName)

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

// helper table writer functions
func prepareInfoSlice(data []interface{}) [][]string {
	dataSlice := [][]string{}
	for _, mapVal := range data {
		m := mapVal.(map[string]interface{})
		target := []string{fmt.Sprintf("%v", m["name"]),
			fmt.Sprintf("%v", m["description"])}
		dataSlice = append(dataSlice, target)
	}

	return dataSlice
}

func WriteInfo(prefix string, data []interface{}) {
	// flushing stdout
	fmt.Println()

	dataSlice := prepareInfoSlice(data)

	table := tablewriter.NewWriter(os.Stdout)
	table.SetRowLine(true)
	table.SetHeader([]string{prefix, "Description"})
	for _, v := range dataSlice {
		table.Append(v)
	}

	table.Render()
}
