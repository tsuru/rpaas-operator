// Copyright 2022 tsuru authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package client

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"mime/multipart"
	"net/http"
	"path/filepath"
	"strings"

	"github.com/tsuru/rpaas-operator/pkg/rpaas/client/types"
)

func (args ExtraFilesArgs) Validate() error {
	if args.Instance == "" {
		return ErrMissingInstance
	}

	if args.Files == nil {
		return fmt.Errorf("rpaasv2: file list cannot be empty")
	}

	return nil
}

func (args DeleteExtraFilesArgs) Validate() error {
	if args.Instance == "" {
		return ErrMissingInstance
	}
	if len(args.Files) == 0 {
		return errors.New("rpaasv2: file list must not be empty")
	}

	return nil
}

func prepareBodyRequest(files map[string][]byte) (*bytes.Buffer, *multipart.Writer, error) {
	buffer := &bytes.Buffer{}
	writer := multipart.NewWriter(buffer)
	for filePath, content := range files {
		partWriter, err := writer.CreateFormFile("files", filepath.Base(filePath))
		if err != nil {
			return nil, nil, err
		}
		partWriter.Write(content)
	}
	if err := writer.Close(); err != nil {
		return nil, nil, err
	}
	return buffer, writer, nil
}

func (c *client) AddExtraFiles(ctx context.Context, args ExtraFilesArgs) error {
	if err := args.Validate(); err != nil {
		return err
	}

	buffer, w, err := prepareBodyRequest(args.Files)
	if err != nil {
		return err
	}

	body := strings.NewReader(buffer.String())
	pathName := fmt.Sprintf("/resources/%s/files", args.Instance)
	req, err := c.newRequest(http.MethodPost, pathName, body, args.Instance)
	req.Header.Set("Content-Type", fmt.Sprintf("multipart/form-data; boundary=%q", w.Boundary()))
	if err != nil {
		return err
	}

	response, err := c.do(ctx, req)
	if err != nil {
		return err
	}

	if response.StatusCode != http.StatusCreated {
		return newErrUnexpectedStatusCodeFromResponse(response)
	}

	return nil
}

func (c *client) UpdateExtraFiles(ctx context.Context, args ExtraFilesArgs) error {
	if err := args.Validate(); err != nil {
		return err
	}

	buffer, w, err := prepareBodyRequest(args.Files)
	if err != nil {
		return err
	}

	body := strings.NewReader(buffer.String())
	pathName := fmt.Sprintf("/resources/%s/files", args.Instance)
	req, err := c.newRequest(http.MethodPut, pathName, body, args.Instance)
	req.Header.Set("Content-Type", fmt.Sprintf("multipart/form-data; boundary=%q", w.Boundary()))
	if err != nil {
		return err
	}

	response, err := c.do(ctx, req)
	if err != nil {
		return err
	}

	if response.StatusCode != http.StatusOK {
		return newErrUnexpectedStatusCodeFromResponse(response)
	}

	return nil
}

func (c *client) DeleteExtraFiles(ctx context.Context, args DeleteExtraFilesArgs) error {
	if err := args.Validate(); err != nil {
		return err
	}

	b, err := json.Marshal(args.Files)
	if err != nil {
		return err
	}
	body := bytes.NewReader(b)

	pathName := fmt.Sprintf("/resources/%s/files", args.Instance)
	req, err := c.newRequest(http.MethodDelete, pathName, body, args.Instance)
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")

	response, err := c.do(ctx, req)
	if err != nil {
		return err
	}

	if response.StatusCode != http.StatusOK {
		return newErrUnexpectedStatusCodeFromResponse(response)
	}

	return nil
}

func (c *client) ListExtraFiles(ctx context.Context, instance string) ([]string, error) {
	if instance == "" {
		return nil, ErrMissingInstance
	}

	pathName := fmt.Sprintf("/resources/%s/files", instance)
	req, err := c.newRequest(http.MethodGet, pathName, nil, instance)
	if err != nil {
		return nil, err
	}

	response, err := c.do(ctx, req)
	if err != nil {
		return nil, err
	}

	if response.StatusCode != http.StatusOK {
		return nil, newErrUnexpectedStatusCodeFromResponse(response)
	}

	var fileList []string
	err = json.NewDecoder(response.Body).Decode(&fileList)
	if err != nil {
		return nil, err
	}
	return fileList, nil
}

func (c *client) GetExtraFile(ctx context.Context, instance, fileName string) (types.RpaasFile, error) {
	pathName := fmt.Sprintf("/resources/%s/files/%s", instance, fileName)
	req, err := c.newRequest(http.MethodGet, pathName, nil, instance)
	if err != nil {
		return types.RpaasFile{}, err
	}

	response, err := c.do(ctx, req)
	if err != nil {
		return types.RpaasFile{}, err
	}

	if response.StatusCode != http.StatusOK {
		return types.RpaasFile{}, newErrUnexpectedStatusCodeFromResponse(response)
	}

	var file types.RpaasFile
	err = json.NewDecoder(response.Body).Decode(&file)
	if err != nil {
		return types.RpaasFile{}, err
	}
	return file, nil
}
