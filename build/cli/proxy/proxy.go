package proxy

import (
	"fmt"
	"io"
	"net/http"
	"os"

	tsuruCmd "github.com/tsuru/tsuru/cmd"
)

type Proxy struct {
	ServiceName  string
	InstanceName string
	Path         string
	Body         io.Reader
	Headers      map[string]string
	Method       string
}

func (p *Proxy) ProxyRequest() (*http.Response, error) {
	_, err := tsuruCmd.GetTarget()
	if err != nil {
		return nil, err
	}

	// target = strings.TrimRight(target, "/")
	token := os.Getenv("TSURU_TOKEN")
	// url := "https:" + target + "/services/" + p.ServiceName + "/proxy/" + p.InstanceName + "?callback=" + p.Path
	url, err := tsuruCmd.GetURL("/services/" + p.ServiceName + "/proxy/" + p.InstanceName + "?callback=" + p.Path)
	fmt.Println("URL= ", url)
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequest("GET", url, p.Body)
	if err != nil {
		return nil, err
	}
	req.Header.Add("Authorization", "bearer "+token)
	fmt.Println(req)

	if p.Headers != nil {
		for key, value := range p.Headers {
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
