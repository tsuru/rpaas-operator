// Copyright 2019 tsuru authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package util

import (
	"context"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	k8sErrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	extensionsv1alpha1 "github.com/tsuru/rpaas-operator/api/v1alpha1"
)

// GetValue retrieves the content inside the Value object.
func GetValue(ctx context.Context, c client.Client, defaultNamespace string, v *extensionsv1alpha1.Value) (string, error) {
	if v == nil {
		return "", fmt.Errorf("value cannot be nil")
	}

	if v.Value != "" {
		return v.Value, nil
	}

	return getValueFromConfigMap(ctx, c, defaultNamespace, v.ValueFrom)
}

func getValueFromConfigMap(ctx context.Context, c client.Client, namespace string, vs *extensionsv1alpha1.ValueSource) (string, error) {
	if vs == nil || vs.ConfigMapKeyRef == nil {
		return "", fmt.Errorf("value source is missing")
	}

	isOptional := vs.ConfigMapKeyRef.Optional == nil || *vs.ConfigMapKeyRef.Optional

	if vs.Namespace != "" {
		namespace = vs.Namespace
	}

	cmName := types.NamespacedName{
		Name:      vs.ConfigMapKeyRef.Name,
		Namespace: namespace,
	}
	var cm corev1.ConfigMap
	if err := c.Get(ctx, cmName, &cm); err != nil {
		if isOptional && k8sErrors.IsNotFound(err) {
			return "", nil
		}

		return "", err
	}

	value, ok := cm.Data[vs.ConfigMapKeyRef.Key]
	if !ok && !isOptional {
		return "", fmt.Errorf("key %q cannot be found in configmap %v", vs.ConfigMapKeyRef.Key, cmName)
	}

	return value, nil
}
