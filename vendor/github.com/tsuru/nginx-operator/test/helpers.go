package test

import (
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"

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
	if _, err := kubectl("delete", "namespace", ns); err != nil {
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

func kubectl(arg ...string) ([]byte, error) {
	cmd := exec.CommandContext(context.TODO(), "kubectl", arg...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("Err running cmd %v: %v. Output: %s", cmd, err, string(out))
	}
	return out, nil
}
