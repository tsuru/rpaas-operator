// Copyright 2025 tsuru authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package client

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/ajg/form"
)

func (args PurgeCacheArgs) Validate() error {
	if args.Instance == "" {
		return ErrMissingInstance
	}

	if args.Path == "" {
		return ErrMissingPath
	}

	return nil
}

func (c *client) PurgeCache(ctx context.Context, args PurgeCacheArgs) (int, error) {
	if err := args.Validate(); err != nil {
		return 0, err
	}

	purgeData := struct {
		Path         string              `json:"path" form:"path"`
		PreservePath bool                `json:"preserve_path,omitempty" form:"preserve_path,omitempty"`
		ExtraHeaders map[string][]string `json:"extra_headers,omitempty" form:"extra_headers,omitempty"`
	}{
		Path:         args.Path,
		PreservePath: args.PreservePath,
		ExtraHeaders: args.ExtraHeaders,
	}

	b, err := form.EncodeToString(purgeData)
	if err != nil {
		return 0, err
	}
	body := strings.NewReader(b)

	pathName := fmt.Sprintf("/resources/%s/purge", args.Instance)
	req, err := c.newRequest("POST", pathName, body)
	if err != nil {
		return 0, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	response, err := c.do(ctx, req)
	if err != nil {
		return 0, err
	}

	defer response.Body.Close()

	if response.StatusCode != http.StatusOK {
		return 0, newErrUnexpectedStatusCodeFromResponse(response)
	}

	bodyBytes, err := io.ReadAll(response.Body)
	if err != nil {
		return 0, err
	}

	var count int
	responseText := string(bodyBytes)
	_, err = fmt.Sscanf(responseText, "Object purged on %d servers", &count)
	if err != nil {
		return 0, nil
	}

	return count, nil
}

func (args PurgeCacheBulkArgs) Validate() error {
	if args.Instance == "" {
		return ErrMissingInstance
	}

	if len(args.Items) == 0 {
		return fmt.Errorf("at least one purge item is required")
	}

	for i, item := range args.Items {
		if item.Path == "" {
			return fmt.Errorf("path is required for item %d", i)
		}
	}

	return nil
}

func (c *client) PurgeCacheBulk(ctx context.Context, args PurgeCacheBulkArgs) ([]PurgeBulkResult, error) {
	if err := args.Validate(); err != nil {
		return nil, err
	}

	type purgeItem struct {
		Path         string              `json:"path"`
		PreservePath bool                `json:"preserve_path"`
		ExtraHeaders map[string][]string `json:"extra_headers,omitempty"`
	}

	var items []purgeItem
	for _, item := range args.Items {
		items = append(items, purgeItem{
			Path:         item.Path,
			PreservePath: item.PreservePath,
			ExtraHeaders: item.ExtraHeaders,
		})
	}

	jsonData, err := json.Marshal(items)
	if err != nil {
		return nil, err
	}

	pathName := fmt.Sprintf("/resources/%s/purge/bulk", args.Instance)
	req, err := c.newRequest("POST", pathName, strings.NewReader(string(jsonData)))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")

	response, err := c.do(ctx, req)
	if err != nil {
		return nil, err
	}

	defer response.Body.Close()

	if response.StatusCode != http.StatusOK && response.StatusCode != http.StatusInternalServerError {
		return nil, newErrUnexpectedStatusCodeFromResponse(response)
	}

	var results []PurgeBulkResult
	if err = unmarshalBody(response, &results); err != nil {
		return nil, err
	}

	return results, nil
}
