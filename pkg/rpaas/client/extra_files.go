// Copyright 2022 tsuru authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package client

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"mime/multipart"
	"net/http"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/tsuru/rpaas-operator/pkg/rpaas/client/types"
)

func (args ExtraFilesArgs) Validate() error {
	if args.Instance == "" {
		return ErrMissingInstance
	}

	if len(args.Files) == 0 {
		return ErrMissingFiles
	}

	return nil
}

func (args DeleteExtraFilesArgs) Validate() error {
	if args.Instance == "" {
		return ErrMissingInstance
	}
	if len(args.Files) == 0 {
		return ErrMissingFiles
	}

	return nil
}

func (args GetExtraFileArgs) Validate() error {
	if args.Instance == "" {
		return ErrMissingInstance
	}

	if args.FileName == "" {
		return ErrMissingFile
	}

	return nil
}

func prepareBodyRequest(files []types.RpaasFile) (*bytes.Buffer, *multipart.Writer, error) {
	buffer := &bytes.Buffer{}
	writer := multipart.NewWriter(buffer)
	for _, file := range files {
		partWriter, err := writer.CreateFormFile("files", filepath.Base(file.Name))
		if err != nil {
			return nil, nil, err
		}
		partWriter.Write(file.Content)
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
	req, err := c.newRequest(http.MethodPost, pathName, body)
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
	req, err := c.newRequest(http.MethodPut, pathName, body)
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
	req, err := c.newRequest(http.MethodDelete, pathName, body)
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

func (c *client) ListExtraFiles(ctx context.Context, args ListExtraFilesArgs) ([]types.RpaasFile, error) {
	if args.Instance == "" {
		return nil, ErrMissingInstance
	}

	pathName := fmt.Sprintf("/resources/%s/files?show-content=%s", args.Instance, strconv.FormatBool(args.ShowContent))
	req, err := c.newRequest(http.MethodGet, pathName, nil)
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

	var fileList []types.RpaasFile
	err = json.NewDecoder(response.Body).Decode(&fileList)
	if err != nil {
		return nil, err
	}
	return fileList, nil
}

func (c *client) GetExtraFile(ctx context.Context, args GetExtraFileArgs) (types.RpaasFile, error) {
	if err := args.Validate(); err != nil {
		return types.RpaasFile{}, err
	}

	pathName := fmt.Sprintf("/resources/%s/files/%s", args.Instance, args.FileName)
	req, err := c.newRequest(http.MethodGet, pathName, nil)
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
