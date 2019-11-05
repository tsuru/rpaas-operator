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
	}, nil
}

func (c *RpaasClient) GetPlans(ctx context.Context, args InfoInstance) ([]types.Plan, error) {
	var pathName string
	var req *http.Request
	var err error

	switch args.Name {
	case nil:
		pathName = fmt.Sprintf("/resources/plans")
		req, err = c.newRequest("GET", "", pathName, nil)
		if err != nil {
			return nil, err
		}
	default:
		pathName = fmt.Sprintf("/resources/%s/plans", *args.Name)
		req, err = c.newRequest("GET", *args.Name, pathName, nil)
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

func (c *RpaasClient) GetFlavors(ctx context.Context, args InfoInstance) ([]types.Flavor, error) {
	var pathName string
	var req *http.Request
	var err error

	switch args.Name {
	case nil:
		pathName = fmt.Sprintf("/resources/flavors")
		req, err = c.newRequest("GET", "", pathName, nil)
		if err != nil {
			return nil, err
		}
	default:
		pathName = fmt.Sprintf("/resources/%s/flavors", *args.Name)
		req, err = c.newRequest("GET", *args.Name, pathName, nil)
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

func (c *RpaasClient) Certificate(ctx context.Context, args CertificateInstance) error {
	pathName := "/resources/" + args.Name + "/certificate"
	body, boundary, err := args.encode()
	if err != nil {
		return err
	}

	readerBody := strings.NewReader(body)
	req, err := c.newRequest("POST", args.Name, pathName, readerBody)
	if err != nil {
		return err
	}

	req.Header.Add("Content-Type", "multipart/form-data; boundary="+boundary)
	resp, err := c.do(ctx, req)

	respBody, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return err
	}

	if resp.StatusCode != http.StatusOK {
		bodyString := string(respBody)
		return fmt.Errorf("Status Code: %v\nResponse Body:\n%v", resp.Status, bodyString)
	}

	return nil
}

func (c *RpaasClient) Scale(ctx context.Context, args ScaleInstance) error {
	if err := scaleValidate(args.Name, args.Replicas); err != nil {
		return err
	}

	bodyStruct := url.Values{}
	bodyStruct.Set("quantity", strconv.Itoa(int(args.Replicas)))

	pathName := fmt.Sprintf("/resources/%s/scale", args.Name)
	bodyReader := strings.NewReader(bodyStruct.Encode())
	req, err := c.newRequest("POST", args.Name, pathName, bodyReader)
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
		} else {
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

type UpdateArgs struct {
	Flavors     []string
	PlanOverr   string
	Plan        string
	Team        string
	User        string
	Ip          string
	Tags        []string
	Description string
	Instance    string
}

func addOtherTags(tags []string, bodyStruct *url.Values) {
	for _, tag := range tags {
		bodyStruct.Add("tag", tag)
	}
}

func (c *RpaasClient) Update(ctx context.Context, args UpdateInstance) error {
	if err := args.validateUpdateArgs(); err != nil {
		return err
	}
	pathName := "/resources/" + args.Name
	bodyStruct := url.Values{}

	if args.Flavors != nil {
		flavorTag := "flavor=" + strings.Join(args.Flavors, ",")
		bodyStruct.Add("tag", flavorTag)
	}

	if args.PlanOverr != "" {
		planOvertag := "plan-override=" + args.PlanOverr
		bodyStruct.Add("tag", planOvertag)
	}

	if args.Ip != "" {
		ipTag := "ip=" + args.Ip
		bodyStruct.Add("tag", ipTag)
	}

	if args.Description != "" {
		bodyStruct.Set("description", args.Description)
	}

	addOtherTags(args.Tags, &bodyStruct)

	bodyStruct.Add("name", args.Name)
	bodyStruct.Add("plan", args.Plan)
	bodyStruct.Add("team", args.Team)
	bodyStruct.Add("user", args.User)

	bodyReader := strings.NewReader(bodyStruct.Encode())

	req, err := c.newRequest("PUT", args.Name, pathName, bodyReader)
	if err != nil {
		return err
	}
	req.Header.Add("Content-Type", "application/x-www-form-urlencoded")

	resp, err := c.do(ctx, req)
	if err != nil {
		return err
	}

	if resp.StatusCode != http.StatusOK {
		bodyString, err := getBodyString(resp)
		if err != nil {
			return err
		}
		return fmt.Errorf("unexpected status code: body: %v", bodyString)
	}

	return nil
}
