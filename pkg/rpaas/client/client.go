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

type jsonInfo struct {
	Name        string `json:"name"`
	Description string `json:"description"`
}

func (c *RpaasClient) Info(ctx context.Context, instance string, infoType string) error {
	if instance == "" {
		return fmt.Errorf("instance can't be nil")
	}

	pathName := fmt.Sprintf("/resources/%s/%s", instance, infoType)
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
		var respData []jsonInfo
		err = json.Unmarshal(body, &respData)
		if err != nil {
			return err
		}

		writeInfo(infoType, respData)
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
	if c.tsuruTarget != "" {
		url = fmt.Sprintf("%s/services/%s/proxy/%s?callback=%s",
			c.tsuruTarget, c.tsuruService, instance, pathName)

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

func writeInfo(prefix string, data []jsonInfo) {
	// flushing stdout
	fmt.Println()

	table := tablewriter.NewWriter(os.Stdout)
	table.SetRowLine(true)
	table.SetHeader([]string{prefix, "Description"})
	for _, dataStruct := range data {
		table.Append([]string{dataStruct.Name, dataStruct.Description})
	}

	table.Render()
}
