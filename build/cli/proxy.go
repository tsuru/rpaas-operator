package main

import (
	"io"
	"net/http"
	"os"
	"strings"
)

type Proxy struct {
	serviceName  string
	instanceName string
	path         string
	body         io.Reader
	headers      map[string]string
	method       string
}

func (p *Proxy) ProxyRequest() (*http.Response, error) {
	target := strings.TrimRight(os.Getenv("TSURU_TARGET"), "/")
	token := os.Getenv("TSURU_TOKEN")
	url := target + "/services/" + p.serviceName + "/proxy/" + p.instanceName + "?callback=" + p.path

	req, err := http.NewRequest("GET", url, p.body)
	if err != nil {
		return nil, err
	}
	req.Header.Add("Authorization", "bearer"+token)

	if p.headers != nil {
		for key, value := range p.headers {
			req.Header.Add(key, value)
		}
	}
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}

	return resp, nil
}
