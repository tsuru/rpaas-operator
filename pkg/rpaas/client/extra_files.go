// Copyright 2022 tsuru authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package client

import (
	"bytes"
	"context"
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

func (c *client) ExtraFiles(ctx context.Context, args ExtraFilesArgs) error {
	if err := args.Validate(); err != nil {
		return err
	}

	buffer := &bytes.Buffer{}
	w := multipart.NewWriter(buffer)
	for filePath, content := range args.Files {
		partWriter, err := w.CreateFormFile("files", filepath.Base(filePath))
		if err != nil {
			return err
		}
		partWriter.Write(content)
	}
	if err := w.Close(); err != nil {
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
