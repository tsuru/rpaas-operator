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
	req, err := c.newRequest("POST", pathName, body, args.Instance)
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
	req, err := c.newRequest("PUT", pathName, body, args.Instance)
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
	if len(args.Files) == 0 {
		return errors.New("rpaasv2: file list must not be empty")
	}

	b, err := json.Marshal(args.Files)
	if err != nil {
		return err
	}
	body := bytes.NewReader(b)

	pathName := fmt.Sprintf("/resources/%s/files", args.Instance)
	req, err := c.newRequest("DELETE", pathName, body, args.Instance)
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
