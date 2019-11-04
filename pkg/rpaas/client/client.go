// Copyright 2019 tsuru authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package client

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"mime/multipart"
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

func (c *RpaasClient) Certificate(ctx context.Context, service, instance, certificate, key, destName string) error {
	pathName := "/resources/" + instance + "/certificate"
	body, boundary, err := encodeCertKey(service, instance, certificate, key, destName)
	if err != nil {
		return err
	}

	readerBody := strings.NewReader(body)
	req, err := c.newRequest("POST", instance, pathName, readerBody)
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

func encodeCertKey(service, instance, certificate, key, destName string) (string, string, error) {
	certBytes, err := ioutil.ReadFile(certificate)
	if err != nil {
		return "", "", fmt.Errorf("Error while trying to read certificate file: %v", err)
	}

	keyFile, err := ioutil.ReadFile(key)
	if err != nil {
		return "", "", fmt.Errorf("Error while trying to read key file: %v", err)
	}

	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)

	certPart, err := writer.CreateFormFile("cert", certificate)
	if err != nil {
		return "", "", fmt.Errorf("Error while trying to create certificate form file: %v", err)
	}

	_, err = certPart.Write(certBytes)
	if err != nil {
		return "", "", fmt.Errorf("Error while trying to write the certificate to the file: %v", err)
	}

	keyPart, err := writer.CreateFormFile("key", key)
	if err != nil {
		return "", "", fmt.Errorf("Error while trying to create key form file: %v", err)
	}
	_, err = keyPart.Write(keyFile)
	if err != nil {
		return "", "", fmt.Errorf("Error while trying to write the key to the file: %v", err)
	}

	writer.WriteField("name", destName)
	err = writer.Close()
	if err != nil {
		return "", "", fmt.Errorf("Error while closing file: %v", err)
	}

	return body.String(), writer.Boundary(), nil
}

type UpdateArgs struct {
	Flavors   []string
	PlanOverr string
	Plan      string
	Team      string
	User      string
	Ip        string
	Tags      []string
	Instance  string
}

func addOtherTags(tags []string, bodyStruct *url.Values) {
	for _, tag := range tags {
		bodyStruct.Add("tag", tag)
	}
}

func validateUpdateArgs(args UpdateArgs) error {
	if args.Instance == "" || args.Plan == "" || args.Team == "" || args.User == "" {
		return fmt.Errorf("must provide a valid instance name, plan, team and user")
	}
	return nil
}

func (c *RpaasClient) Update(ctx context.Context, args UpdateArgs) error {
	if err := validateUpdateArgs(args); err != nil {
		return err
	}
	pathName := "/resources/" + args.Instance
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

	addOtherTags(args.Tags, &bodyStruct)

	bodyStruct.Add("name", args.Instance)
	bodyStruct.Add("plan", args.Plan)
	bodyStruct.Add("team", args.Team)
	bodyStruct.Add("user", args.User)

	bodyReader := strings.NewReader(bodyStruct.Encode())

	req, err := c.newRequest("PUT", args.Instance, pathName, bodyReader)
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
