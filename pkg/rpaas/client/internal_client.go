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
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/tsuru/rpaas-operator/pkg/rpaas/client/types"
)

var (
	ErrMissingTsuruTarget       = fmt.Errorf("rpaasv2: tsuru target cannot be empty")
	ErrMissingTsuruToken        = fmt.Errorf("rpaasv2: tsuru token cannot be empty")
	ErrMissingTsuruService      = fmt.Errorf("rpaasv2: tsuru service cannot be empty")
	ErrMissingInstance          = fmt.Errorf("rpaasv2: instance cannot be empty")
	ErrMissingBlockName         = fmt.Errorf("rpaasv2: block name cannot be empty")
	ErrMissingPath              = fmt.Errorf("rpaasv2: path cannot be empty")
	ErrInvalidMaxReplicasNumber = fmt.Errorf("rpaasv2: max replicas can't be lower than 1")
	ErrInvalidMinReplicasNumber = fmt.Errorf("rpaasv2: min replicas can't be lower than 1 and can't be higher than the maximum number of replicas")
	ErrInvalidCPUUsage          = fmt.Errorf("rpaasv2: CPU usage can't be lower than 1%%")
	ErrInvalidMemoryUsage       = fmt.Errorf("rpaasv2: memory usage can't be lower than 1%%")
	ErrMissingValues            = fmt.Errorf("rpaasv2: values can't be all empty")
)

type ErrUnexpectedStatusCode string

func (statusCode ErrUnexpectedStatusCode) Error() string {
	return fmt.Sprintf("rpaasv2: unexpected status code: %s", string(statusCode))
}

type ClientOptions struct {
	Timeout time.Duration
}

var DefaultClientOptions = ClientOptions{
	Timeout: 10 * time.Second,
}

func NewClient(address, user, password string) (Client, error) {
	return NewClientWithOptions(address, user, password, DefaultClientOptions)
}

func NewClientWithOptions(address, user, password string, opts ClientOptions) (Client, error) {
	if address == "" {
		return nil, fmt.Errorf("cannot create a client without address")
	}

	return &client{
		rpaasAddress:  address,
		rpaasUser:     user,
		rpaasPassword: password,
		client:        newHTTPClient(opts),
	}, nil
}

func NewClientThroughTsuru(target, token, service string) (Client, error) {
	return NewClientThroughTsuruWithOptions(target, token, service, DefaultClientOptions)
}

func NewClientThroughTsuruWithOptions(target, token, service string, opts ClientOptions) (Client, error) {
	if t, ok := os.LookupEnv("TSURU_TARGET"); target == "" && ok {
		target = t
	}

	if t, ok := os.LookupEnv("TSURU_TOKEN"); token == "" && ok {
		token = t
	}

	if target == "" {
		return nil, ErrMissingTsuruTarget
	}

	if token == "" {
		return nil, ErrMissingTsuruToken
	}

	if service == "" {
		return nil, ErrMissingTsuruService
	}

	return &client{
		tsuruTarget:  target,
		tsuruToken:   token,
		tsuruService: service,
		throughTsuru: true,
		client:       newHTTPClient(opts),
	}, nil
}

func newHTTPClient(opts ClientOptions) *http.Client {
	return &http.Client{
		Timeout: opts.Timeout,
	}
}

type client struct {
	rpaasAddress  string
	rpaasUser     string
	rpaasPassword string

	tsuruTarget  string
	tsuruToken   string
	tsuruService string
	throughTsuru bool

	client *http.Client
}

var _ Client = &client{}

func (c *client) GetPlans(ctx context.Context, instance string) ([]types.Plan, *http.Response, error) {
	return nil, nil, fmt.Errorf("not implemented yet")
}

func (c *client) GetFlavors(ctx context.Context, instance string) ([]types.Flavor, *http.Response, error) {
	return nil, nil, fmt.Errorf("not implemented yet")
}

func (args ScaleArgs) Validate() error {
	if args.Instance == "" {
		return ErrMissingInstance
	}

	if args.Replicas < int32(0) {
		return fmt.Errorf("rpaasv2: replicas must be greater or equal than zero")
	}

	return nil
}

func (c *client) Scale(ctx context.Context, args ScaleArgs) (*http.Response, error) {
	if err := args.Validate(); err != nil {
		return nil, err
	}

	request, err := c.buildRequest("Scale", args)
	if err != nil {
		return nil, err
	}

	response, err := c.do(ctx, request)
	if err != nil {
		return response, err
	}

	if response.StatusCode != http.StatusOK {
		return response, ErrUnexpectedStatusCode(response.Status)
	}

	return response, nil
}

func (args InfoArgs) Validate() error {
	if args.Instance == "" {
		return ErrMissingInstance
	}

	return nil
}

func (c *client) Info(ctx context.Context, args InfoArgs) (*types.InstanceInfo, *http.Response, error) {
	if err := args.Validate(); err != nil {
		return nil, nil, err
	}

	request, err := c.buildRequest("Info", args)
	if err != nil {
		return nil, nil, err
	}

	response, err := c.do(ctx, request)
	if err != nil {
		return nil, nil, err
	}

	if response.StatusCode != http.StatusOK {
		return nil, response, ErrUnexpectedStatusCode(response.Status)
	}

	defer response.Body.Close()
	var infoPayload types.InstanceInfo
	err = json.NewDecoder(response.Body).Decode(&infoPayload)
	if err != nil {
		return nil, nil, err
	}

	return &infoPayload, response, nil
}

func (args UpdateCertificateArgs) Validate() error {
	if args.Instance == "" {
		return ErrMissingInstance
	}

	if args.Certificate == "" {
		return fmt.Errorf("rpaasv2: certificate cannot be empty")
	}

	if args.Key == "" {
		return fmt.Errorf("rpaasv2: key cannot be empty")
	}

	return nil
}

func (c *client) UpdateCertificate(ctx context.Context, args UpdateCertificateArgs) (*http.Response, error) {
	if err := args.Validate(); err != nil {
		return nil, err
	}

	request, err := c.buildRequest("UpdateCertificate", args)
	if err != nil {
		return nil, err
	}

	response, err := c.do(ctx, request)
	if err != nil {
		return response, err
	}

	if response.StatusCode != http.StatusOK {
		return response, ErrUnexpectedStatusCode(response.Status)
	}

	return response, nil
}

func (args UpdateBlockArgs) Validate() error {
	if args.Instance == "" {
		return ErrMissingInstance
	}

	if args.Name == "" {
		return ErrMissingBlockName
	}

	if args.Content == "" {
		return fmt.Errorf("rpaasv2: content cannot be empty")
	}

	return nil
}

func (c *client) UpdateBlock(ctx context.Context, args UpdateBlockArgs) (*http.Response, error) {
	if err := args.Validate(); err != nil {
		return nil, err
	}

	request, err := c.buildRequest("UpdateBlock", args)
	if err != nil {
		return nil, err
	}

	response, err := c.do(ctx, request)
	if err != nil {
		return response, err
	}

	if response.StatusCode != http.StatusOK {
		return response, ErrUnexpectedStatusCode(response.Status)
	}

	return response, nil
}

func (args DeleteBlockArgs) Validate() error {
	if args.Instance == "" {
		return ErrMissingInstance
	}

	if args.Name == "" {
		return ErrMissingBlockName
	}

	return nil
}

func (c *client) DeleteBlock(ctx context.Context, args DeleteBlockArgs) (*http.Response, error) {
	if err := args.Validate(); err != nil {
		return nil, err
	}

	request, err := c.buildRequest("DeleteBlock", args)
	if err != nil {
		return nil, err
	}

	response, err := c.do(ctx, request)
	if err != nil {
		return response, err
	}

	if response.StatusCode != http.StatusOK {
		return response, ErrUnexpectedStatusCode(response.Status)
	}

	return response, nil
}

func (args ListBlocksArgs) Validate() error {
	if args.Instance == "" {
		return ErrMissingInstance
	}

	return nil
}

func (c *client) ListBlocks(ctx context.Context, args ListBlocksArgs) ([]types.Block, *http.Response, error) {
	if err := args.Validate(); err != nil {
		return nil, nil, err
	}

	request, err := c.buildRequest("ListBlocks", args)
	if err != nil {
		return nil, nil, err
	}

	response, err := c.do(ctx, request)
	if err != nil {
		return nil, response, err
	}

	if response.StatusCode != http.StatusOK {
		return nil, response, ErrUnexpectedStatusCode(response.Status)
	}

	var blockList struct {
		Blocks []types.Block `json:"blocks"`
	}
	if err = unmarshalBody(response, &blockList); err != nil {
		return nil, response, err
	}

	return blockList.Blocks, response, nil
}

func (args DeleteRouteArgs) Validate() error {
	if args.Instance == "" {
		return ErrMissingInstance
	}

	if args.Path == "" {
		return ErrMissingPath
	}

	return nil
}

func (c *client) DeleteRoute(ctx context.Context, args DeleteRouteArgs) (*http.Response, error) {
	if err := args.Validate(); err != nil {
		return nil, err
	}

	request, err := c.buildRequest("DeleteRoute", args)
	if err != nil {
		return nil, err
	}

	response, err := c.do(ctx, request)
	if err != nil {
		return response, err
	}

	if response.StatusCode != http.StatusOK {
		return response, ErrUnexpectedStatusCode(response.Status)
	}

	return response, nil
}

func (args ListRoutesArgs) Validate() error {
	if args.Instance == "" {
		return ErrMissingInstance
	}

	return nil
}

func (c *client) ListRoutes(ctx context.Context, args ListRoutesArgs) ([]types.Route, *http.Response, error) {
	if err := args.Validate(); err != nil {
		return nil, nil, err
	}

	request, err := c.buildRequest("ListRoutes", args)
	if err != nil {
		return nil, nil, err
	}

	response, err := c.do(ctx, request)
	if err != nil {
		return nil, response, err
	}

	if response.StatusCode != http.StatusOK {
		return nil, response, ErrUnexpectedStatusCode(response.Status)
	}

	var routes struct {
		Routes []types.Route `json:"paths"`
	}
	if err = unmarshalBody(response, &routes); err != nil {
		return nil, response, err
	}

	return routes.Routes, response, nil
}

func (args UpdateRouteArgs) Validate() error {
	if args.Instance == "" {
		return ErrMissingInstance
	}

	if args.Path == "" {
		return ErrMissingPath
	}

	return nil
}

func (c *client) UpdateRoute(ctx context.Context, args UpdateRouteArgs) (*http.Response, error) {
	if err := args.Validate(); err != nil {
		return nil, err
	}

	request, err := c.buildRequest("UpdateRoute", args)
	if err != nil {
		return nil, err
	}

	response, err := c.do(ctx, request)
	if err != nil {
		return response, err
	}

	if response.StatusCode != http.StatusCreated {
		return response, ErrUnexpectedStatusCode(response.Status)
	}

	return response, nil
}

func (args GetAutoscaleArgs) Validate() error {
	if args.Instance == "" {
		return ErrMissingInstance
	}

	return nil
}

func (c *client) GetAutoscale(ctx context.Context, args GetAutoscaleArgs) (*types.Autoscale, *http.Response, error) {
	if err := args.Validate(); err != nil {
		return nil, nil, err
	}

	request, err := c.buildRequest("GetAutoscale", args)
	if err != nil {
		return nil, nil, err
	}

	resp, err := c.do(ctx, request)
	if err != nil {
		return nil, resp, err
	}

	if resp.StatusCode != http.StatusOK {
		return nil, resp, ErrUnexpectedStatusCode(resp.Status)
	}

	defer resp.Body.Close()
	var spec *types.Autoscale
	err = json.NewDecoder(resp.Body).Decode(&spec)
	if err != nil {
		return nil, nil, err
	}

	return spec, resp, nil
}

func (args UpdateAutoscaleArgs) Validate() error {
	if args.Instance == "" {
		return ErrMissingInstance
	}

	if args.MaxReplicas == 0 && args.MaxReplicas == args.MinReplicas && args.MaxReplicas == args.CPU && args.MaxReplicas == args.Memory {
		return ErrMissingValues
	}
	return nil
}

func (c *client) shouldCreate(ctx context.Context, instance string) (bool, error) {
	_, resp, err := c.GetAutoscale(ctx, GetAutoscaleArgs{Instance: instance})
	if err != nil {
		if resp.StatusCode == http.StatusNotFound {
			return true, nil
		}
		return false, err
	}
	return false, nil
}

func (c *client) UpdateAutoscale(ctx context.Context, args UpdateAutoscaleArgs) (*http.Response, error) {
	if err := args.Validate(); err != nil {
		return nil, err
	}

	var request *http.Request
	shouldCreate, err := c.shouldCreate(ctx, args.Instance)
	if err != nil {
		return nil, err
	}
	if shouldCreate {
		request, err = c.buildRequest("CreateAutoscale", args)
		if err != nil {
			return nil, err
		}
	} else {
		request, err = c.buildRequest("UpdateAutoscale", args)
		if err != nil {
			return nil, err
		}
	}

	resp, err := c.do(ctx, request)
	if err != nil {
		return resp, err
	}

	if resp.StatusCode != http.StatusCreated {
		return resp, ErrUnexpectedStatusCode(resp.Status)
	}

	return resp, nil
}

func (args RemoveAutoscaleArgs) Validate() error {
	if args.Instance == "" {
		return ErrMissingInstance
	}
	return nil
}

func (c *client) RemoveAutoscale(ctx context.Context, args RemoveAutoscaleArgs) (*http.Response, error) {
	if err := args.Validate(); err != nil {
		return nil, err
	}

	request, err := c.buildRequest("RemoveAutoscale", args)
	if err != nil {
		return nil, err
	}

	resp, err := c.do(ctx, request)
	if err != nil {
		return resp, err
	}

	if resp.StatusCode != http.StatusOK {
		return resp, ErrUnexpectedStatusCode(resp.Status)
	}

	return resp, nil
}

func (c *client) buildRequest(operation string, data interface{}) (req *http.Request, err error) {
	switch operation {
	case "Scale":
		args := data.(ScaleArgs)
		pathName := fmt.Sprintf("/resources/%s/scale", args.Instance)
		values := url.Values{}
		values.Set("quantity", fmt.Sprint(args.Replicas))
		body := strings.NewReader(values.Encode())
		req, err = c.newRequest("POST", pathName, body, args.Instance)
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	case "Info":
		args := data.(InfoArgs)
		pathName := fmt.Sprintf("/resources/%s/info", args.Instance)
		req, err = c.newRequest("GET", pathName, nil, args.Instance)

	case "UpdateCertificate":
		args := data.(UpdateCertificateArgs)
		buffer := &bytes.Buffer{}
		w := multipart.NewWriter(buffer)

		if args.boundary != "" {
			if err = w.SetBoundary(args.boundary); err != nil {
				return nil, err
			}
		}

		var part io.Writer
		{
			part, err = w.CreateFormFile("cert", "cert.pem")
			if err != nil {
				return nil, err
			}

			part.Write([]byte(args.Certificate))
		}
		{
			part, err = w.CreateFormFile("key", "key.pem")
			if err != nil {
				return nil, err
			}

			part.Write([]byte(args.Key))
		}

		if err = w.WriteField("name", args.Name); err != nil {
			return nil, err
		}

		if err = w.Close(); err != nil {
			return nil, err
		}

		body := strings.NewReader(buffer.String())
		pathName := fmt.Sprintf("/resources/%s/certificate", args.Instance)
		req, err = c.newRequest("POST", pathName, body, args.Instance)
		req.Header.Set("Content-Type", fmt.Sprintf("multipart/form-data; boundary=%q", w.Boundary()))

	case "UpdateBlock":
		args := data.(UpdateBlockArgs)
		values := url.Values{}
		values.Set("block_name", args.Name)
		values.Set("content", args.Content)
		body := strings.NewReader(values.Encode())
		pathName := fmt.Sprintf("/resources/%s/block", args.Instance)
		req, err = c.newRequest("POST", pathName, body, args.Instance)
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	case "DeleteBlock":
		args := data.(DeleteBlockArgs)
		pathName := fmt.Sprintf("/resources/%s/block/%s", args.Instance, args.Name)
		req, err = c.newRequest("DELETE", pathName, nil, args.Instance)

	case "ListBlocks":
		args := data.(ListBlocksArgs)
		pathName := fmt.Sprintf("/resources/%s/block", args.Instance)
		req, err = c.newRequest("GET", pathName, nil, args.Instance)

	case "GetAutoscale":
		args := data.(GetAutoscaleArgs)
		pathName := fmt.Sprintf("/resources/%s/autoscale", args.Instance)
		req, err = c.newRequest("GET", pathName, nil, args.Instance)

	case "RemoveAutoscale":
		args := data.(RemoveAutoscaleArgs)
		pathName := fmt.Sprintf("/resources/%s/autoscale", args.Instance)
		req, err = c.newRequest("DELETE", pathName, nil, args.Instance)

	case "CreateAutoscale":
		args := data.(UpdateAutoscaleArgs)
		pathName := fmt.Sprintf("/resources/%s/autoscale", args.Instance)
		values := url.Values{}
		values.Set("max", fmt.Sprint(args.MaxReplicas))
		values.Set("min", fmt.Sprint(args.MinReplicas))
		values.Set("cpu", fmt.Sprint(args.CPU))
		values.Set("memory", fmt.Sprint(args.Memory))
		body := strings.NewReader(values.Encode())
		req, err = c.newRequest("POST", pathName, body, args.Instance)
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	case "UpdateAutoscale":
		args := data.(UpdateAutoscaleArgs)
		pathName := fmt.Sprintf("/resources/%s/autoscale", args.Instance)
		values := url.Values{}
		if args.MaxReplicas > 0 {
			values.Set("max", fmt.Sprint(args.MaxReplicas))
		}
		if args.MinReplicas > 0 {
			values.Set("min", fmt.Sprint(args.MinReplicas))
		}
		if args.CPU > 0 {
			values.Set("cpu", fmt.Sprint(args.CPU))
		}
		if args.Memory > 0 {
			values.Set("memory", fmt.Sprint(args.Memory))
		}
		body := strings.NewReader(values.Encode())
		req, err = c.newRequest("PATCH", pathName, body, args.Instance)
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	case "DeleteRoute":
		args := data.(DeleteRouteArgs)
		pathName := fmt.Sprintf("/resources/%s/route", args.Instance)
		values := url.Values{}
		values.Set("path", args.Path)
		body := strings.NewReader(values.Encode())
		req, err = c.newRequest("DELETE", pathName, body, args.Instance)
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	case "ListRoutes":
		args := data.(ListRoutesArgs)
		pathName := fmt.Sprintf("/resources/%s/route", args.Instance)
		req, err = c.newRequest("GET", pathName, nil, args.Instance)

	case "UpdateRoute":
		args := data.(UpdateRouteArgs)
		pathName := fmt.Sprintf("/resources/%s/route", args.Instance)
		values := url.Values{
			"path":        []string{args.Path},
			"destination": []string{args.Destination},
			"https_only":  []string{strconv.FormatBool(args.HTTPSOnly)},
			"content":     []string{args.Content},
		}
		body := strings.NewReader(values.Encode())
		req, err = c.newRequest("POST", pathName, body, args.Instance)
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	default:
		err = fmt.Errorf("rpaasv2: unknown operation")
	}

	return
}

func (c *client) newRequest(method, pathName string, body io.Reader, instance string) (*http.Request, error) {
	url := c.formatURL(pathName, instance)
	request, err := http.NewRequest(method, url, body)
	if err != nil {
		return nil, err
	}

	if c.throughTsuru {
		request.Header.Set("Authorization", fmt.Sprintf("Bearer %s", c.tsuruToken))
		return request, nil
	}

	if c.rpaasUser != "" && c.rpaasPassword != "" {
		request.SetBasicAuth(c.rpaasUser, c.rpaasPassword)
	}

	return request, nil
}

func (c *client) do(ctx context.Context, request *http.Request) (*http.Response, error) {
	return c.client.Do(request.WithContext(ctx))
}

func (c *client) formatURL(pathName, instance string) string {
	if !c.throughTsuru {
		return fmt.Sprintf("%s%s", c.rpaasAddress, pathName)
	}

	return fmt.Sprintf("%s/services/%s/proxy/%s?callback=%s", c.tsuruTarget, c.tsuruService, instance, pathName)
}

func unmarshalBody(resp *http.Response, dst interface{}) error {
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return err
	}

	defer resp.Body.Close()
	return json.Unmarshal(body, dst)
}
