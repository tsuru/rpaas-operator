package api

import (
	"bytes"
	"mime/multipart"
)

const boundary = "XXXXXXXXXXXX"

type multipartFile struct {
	filename string
	content  string
}

func newMultipartFormBody(name string, files ...multipartFile) (string, error) {
	b := &bytes.Buffer{}
	w := multipart.NewWriter(b)
	w.SetBoundary(boundary)
	for _, f := range files {
		writer, err := w.CreateFormFile(name, f.filename)
		if err != nil {
			return "", err
		}
		if _, err = writer.Write([]byte(f.content)); err != nil {
			return "", err
		}
	}
	w.Close()
	return b.String(), nil
}
