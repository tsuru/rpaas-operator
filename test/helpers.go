// Copyright 2019 tsuru authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package test

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/tsuru/rpaas-operator/internal/pkg/rpaas"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

func createNamespace(ns string) (func() error, error) {
	nilFunc := func() error { return nil }
	if out, err := kubectl("create", "namespace", ns); err != nil {
		if strings.Contains(string(out)+err.Error(), "AlreadyExists") {
			return nilFunc, nil
		}
		return nilFunc, fmt.Errorf("failed to create namespace %q: %v - out: %v", ns, err, string(out))
	}
	return func() error {
		return deleteNamespace(ns)
	}, nil
}

func deleteNamespace(ns string) error {
	if _, err := kubectl("delete", "namespace", ns, "--timeout=30s"); err != nil {
		if strings.Contains(err.Error(), "timed out waiting") {
			return nil
		}
		return fmt.Errorf("failed to delete namespace %q: %v", ns, err)
	}
	return nil
}

func apply(file string, ns string) error {
	if _, err := kubectl("apply", "-f", file, "--namespace", ns); err != nil {
		return fmt.Errorf("failed to apply %q: %v", file, err)
	}
	return nil
}

func delete(file string, ns string) error {
	if _, err := kubectl("delete", "-f", file, "--namespace", ns); err != nil {
		return fmt.Errorf("failed to apply %q: %v", file, err)
	}
	return nil
}

func get(obj runtime.Object, name, ns string) error {
	out, err := kubectl("get", obj.GetObjectKind().GroupVersionKind().Kind, "-o", "json", name, "--namespace", ns)
	if err != nil {
		return err
	}
	return json.Unmarshal(out, obj)
}

func getConfigList(instanceName, namespaceName string) (*corev1.ConfigMapList, error) {
	configList := &corev1.ConfigMapList{}
	out, err := kubectl("get", "cm", "-l", fmt.Sprintf("instance=%s,type=config", instanceName), "-n", namespaceName, "-o", "json")
	if err != nil {
		return nil, err
	}
	if err = json.Unmarshal(out, configList); err != nil {
		return nil, err
	}
	return configList, nil
}

func portForward(ctx context.Context, ns, name, port string, fn func(localPort int)) error {
	cmd := exec.CommandContext(ctx, "kubectl", []string{
		"port-forward",
		"--namespace", ns,
		name, fmt.Sprintf(":%s", port),
		"--address", "127.0.0.1",
	}...)

	stderr, err := cmd.StderrPipe()
	if err != nil {
		return err
	}

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return err
	}

	if err = cmd.Start(); err != nil {
		return err
	}

	scanner := bufio.NewScanner(stdout)
	if scanner.Scan(); scanner.Err() != nil {
		defer cmd.Process.Kill()
		return scanner.Err()
	}

	var localPort int
	regexLocalPort := regexp.MustCompile(`:([0-9]+)`)

	matchs := regexLocalPort.FindStringSubmatch(scanner.Text())

	if len(matchs) < 2 {
		defer cmd.Process.Kill()
		return fmt.Errorf("could not find the local port: %q - %v", scanner.Text(), matchs)
	}

	localPort, err = strconv.Atoi(matchs[1])
	if err != nil {
		defer cmd.Process.Kill()
		return err
	}

	fn(localPort)

	err = cmd.Process.Kill()

	rawStderr, newErr := ioutil.ReadAll(stderr)
	fmt.Printf("Process standard error: %q - %v\n", string(rawStderr), newErr)

	rawStdout, newErr := ioutil.ReadAll(stdout)
	fmt.Printf("Process standard output: %q - %v\n", string(rawStdout), newErr)

	return err
}

func kubectl(arg ...string) ([]byte, error) {
	cmd := exec.CommandContext(context.TODO(), "kubectl", arg...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("Err running cmd %v: %v. Output: %s", cmd, err, string(out))
	}
	return out, nil
}

func kubectlWithRetry(args ...string) ([]byte, error) {
	timeout := time.Now().Add(time.Second * 30)
	for {
		out, err := kubectl(args...)

		if err == nil || time.Now().After(timeout) {
			return out, err
		}

		time.Sleep(time.Second)
	}
}

type rpaasApi struct {
	address string
	client  *http.Client
}

func (api *rpaasApi) createInstance(name, plan, team string) (func() error, error) {
	nilFunc := func() error { return nil }
	data := url.Values{"name": []string{name}, "plan": []string{plan}, "team": []string{team}}
	rsp, err := api.client.PostForm(fmt.Sprintf("%s/resources", api.address), data)
	if err != nil {
		return nilFunc, err
	}
	if rsp.StatusCode != http.StatusCreated {
		defer rsp.Body.Close()
		body, err := ioutil.ReadAll(rsp.Body)
		if err != nil {
			return nilFunc, err
		}
		return nilFunc, fmt.Errorf("could not create the instance %q: %v - Body %s", name, rsp, string(body))
	}
	return func() error {
		return api.deleteInstance(name)
	}, nil
}

func (api *rpaasApi) deleteInstance(name string) error {
	req, err := http.NewRequest(http.MethodDelete, fmt.Sprintf("%s/resources/%s", api.address, name), nil)
	if err != nil {
		return err
	}
	rsp, err := api.client.Do(req)
	if err != nil {
		return err
	}
	if rsp.StatusCode != http.StatusOK {
		defer rsp.Body.Close()
		body, err := ioutil.ReadAll(rsp.Body)
		if err != nil {
			return err
		}
		return fmt.Errorf("could not delete the instance %q: %v - Body %s", name, rsp, string(body))
	}
	return nil
}

func (api *rpaasApi) createBlock(instanceName, blockName, blockContent string) (func() error, error) {
	nilFunc := func() error { return nil }
	data := url.Values{"block_name": []string{blockName}, "content": []string{blockContent}}
	rsp, err := api.client.PostForm(fmt.Sprintf("%s/resources/%s/block", api.address, instanceName), data)
	if err != nil {
		return nilFunc, err
	}
	if rsp.StatusCode != http.StatusOK {
		defer rsp.Body.Close()
		body, err := ioutil.ReadAll(rsp.Body)
		if err != nil {
			return nilFunc, err
		}
		return nilFunc, fmt.Errorf("could not create the block %q for instance %q: %v - Body %s", blockName, instanceName, rsp, string(body))
	}
	return func() error {
		return api.deleteBlock(instanceName, blockName)
	}, nil
}

func (api *rpaasApi) deleteBlock(instanceName, blockName string) error {
	req, err := http.NewRequest(http.MethodDelete, fmt.Sprintf("%s/resources/%s/block/%s", api.address, instanceName, blockName), nil)
	if err != nil {
		return err
	}
	rsp, err := api.client.Do(req)
	if err != nil {
		return err
	}
	if rsp.StatusCode != http.StatusOK {
		defer rsp.Body.Close()
		body, err := ioutil.ReadAll(rsp.Body)
		if err != nil {
			return err
		}
		return fmt.Errorf("could not delete the block %s from instance %q: %v - Body %s", blockName, instanceName, rsp, string(body))
	}
	return nil
}

func (api *rpaasApi) health() (bool, error) {
	rsp, err := api.client.Get(fmt.Sprintf("%s/healthcheck", api.address))
	if err != nil {
		return false, err
	}
	return rsp.StatusCode == http.StatusOK, nil
}

func (api *rpaasApi) bind(appName, instanceName, host string) error {
	data := url.Values{"app-hosts": []string{host}, "app-name": []string{appName}}
	rsp, err := api.client.PostForm(fmt.Sprintf("%s/resources/%s/bind-app", api.address, instanceName), data)
	if err != nil {
		return err
	}
	if rsp.StatusCode != http.StatusCreated {
		defer rsp.Body.Close()
		body, err := ioutil.ReadAll(rsp.Body)
		if err != nil {
			return err
		}
		return fmt.Errorf("could not bind the instance %q: %v - Body %v", instanceName, rsp, string(body))
	}
	return nil
}

func (api *rpaasApi) unbind(appName, instanceName, host string) error {
	requestBody := "app-name=" + appName
	request, err := http.NewRequest(http.MethodDelete, fmt.Sprintf("%s/resources/%s/bind-app", api.address, instanceName), strings.NewReader(requestBody))
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	if err != nil {
		return err
	}
	rsp, err := api.client.Do(request)
	if err != nil {
		return err
	}
	if rsp.StatusCode != http.StatusOK {
		defer rsp.Body.Close()
		body, err := ioutil.ReadAll(rsp.Body)
		if err != nil {
			return err
		}
		return fmt.Errorf("could not unbind the instance %q: %v - Body %v", instanceName, rsp, string(body))
	}
	return nil
}

func (api *rpaasApi) updateRoute(name string, r rpaas.Route) error {
	data := url.Values{
		"path":        []string{r.Path},
		"content":     []string{r.Content},
		"destination": []string{r.Destination},
		"https_only":  []string{fmt.Sprint(r.HTTPSOnly)},
	}
	rsp, err := api.client.PostForm(fmt.Sprintf("%s/resources/%s/route", api.address, name), data)
	if err != nil {
		return err
	}
	if rsp.StatusCode != http.StatusCreated {
		defer rsp.Body.Close()
		body, err := ioutil.ReadAll(rsp.Body)
		if err != nil {
			return err
		}
		return fmt.Errorf("could not update the route %q: %v - Body %v", r.Path, rsp, string(body))
	}
	return nil
}

func (api *rpaasApi) deleteRoute(name, path string) error {
	request, err := http.NewRequest(http.MethodDelete, fmt.Sprintf("%s/resources/%s/route", api.address, name), strings.NewReader(fmt.Sprintf("path=%s", path)))
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	if err != nil {
		return err
	}
	rsp, err := api.client.Do(request)
	if err != nil {
		return err
	}
	if rsp.StatusCode != http.StatusOK {
		defer rsp.Body.Close()
		body, err := ioutil.ReadAll(rsp.Body)
		if err != nil {
			return err
		}
		return fmt.Errorf("could not delete the route %q: %v - Body %v", path, rsp, string(body))
	}
	return nil
}
