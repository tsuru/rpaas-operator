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
	"github.com/tsuru/rpaas-operator/pkg/rpaas/client/types"
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
}

func (c *RpaasClient) GetPlans(ctx context.Context, instance *string) ([]types.Plan, error) {

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
		var plans []types.Plan
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

func (c *RpaasClient) GetFlavors(ctx context.Context, instance *string) ([]types.Flavor, error) {
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
		var flavors []types.Flavor
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

func (c *RpaasClient) getUrl(instance, pathName string) string {
	var url string
	if c.tsuruTarget != "" {
		if instance == "" {
			url = fmt.Sprintf("%s/services/proxy/%s?callback=%s",
				c.tsuruTarget, c.tsuruService, pathName)
			url = fmt.Sprintf("%s/services/%s/proxy/%s?callback=%s",
				c.tsuruTarget, c.tsuruService, instance, pathName)
		}
	} else {
		url = fmt.Sprintf("%s%s", c.hostAPI, pathName)
	}
	return url
}

func (c *RpaasClient) newRequest(method, instance, pathName string, body io.Reader) (*http.Request, error) {
	url := c.getUrl(instance, pathName)

	req, err := http.NewRequest(method, url, body)
	if err != nil {
		return nil, err
	}

	if c.tsuruToken != "" {
		req.Header.Add("Authorization", "Bearer "+c.tsuruToken)
	}

	return req, nil
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

func writeInfo(prefix string, data []interface{}) {
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
