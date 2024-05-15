// Copyright 2024 tsuru authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package controllers

import (
	"context"
	"reflect"
	"sort"

	"github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
	k8sErrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/tsuru/rpaas-operator/api/v1alpha1"
	"github.com/tsuru/rpaas-operator/internal/pkg/rpaas/nginx"
	"github.com/tsuru/rpaas-operator/pkg/util"
)

func listDefaultFlavors(ctx context.Context, c client.Client, namespace string) ([]v1alpha1.RpaasFlavor, error) {
	flavorList := &v1alpha1.RpaasFlavorList{}
	if err := c.List(ctx, flavorList, client.InNamespace(namespace)); err != nil {
		return nil, err
	}

	var result []v1alpha1.RpaasFlavor
	for _, flavor := range flavorList.Items {
		if flavor.Spec.Default {
			result = append(result, flavor)
		}
	}
	sort.SliceStable(result, func(i, j int) bool { return result[i].Name < result[j].Name })
	return result, nil
}

type referenceFinder struct {
	client    client.Client
	namespace string
	spec      *v1alpha1.RpaasInstanceSpec
}

func (r *referenceFinder) getConfigurationBlocks(ctx context.Context, plan *v1alpha1.RpaasPlan) (nginx.ConfigurationBlocks, error) {
	var blocks nginx.ConfigurationBlocks

	if plan.Spec.Template != nil {
		mainBlock, err := util.GetValue(ctx, r.client, "", plan.Spec.Template)
		if err != nil {
			return blocks, err
		}

		blocks.MainBlock = mainBlock
	}

	for blockType, blockValue := range r.spec.Blocks {
		content, err := util.GetValue(ctx, r.client, r.namespace, &blockValue)
		if err != nil {
			return blocks, err
		}

		switch blockType {
		case v1alpha1.BlockTypeRoot:
			blocks.RootBlock = content
		case v1alpha1.BlockTypeHTTP:
			blocks.HttpBlock = content
		case v1alpha1.BlockTypeServer:
			blocks.ServerBlock = content
		case v1alpha1.BlockTypeLuaServer:
			blocks.LuaServerBlock = content
		case v1alpha1.BlockTypeLuaWorker:
			blocks.LuaWorkerBlock = content
		}
	}

	return blocks, nil
}

func (r *referenceFinder) updateLocationValues(ctx context.Context) error {
	for _, location := range r.spec.Locations {
		if location.Content == nil {
			continue
		}

		content, err := util.GetValue(ctx, r.client, r.namespace, location.Content)
		if err != nil {
			return err
		}

		location.Content.Value = content
	}
	return nil
}

func reconcileConfigMap(ctx context.Context, c client.Client, configMap *corev1.ConfigMap) (hasChanged bool, err error) {
	found := &corev1.ConfigMap{}
	err = c.Get(ctx, types.NamespacedName{Name: configMap.ObjectMeta.Name, Namespace: configMap.ObjectMeta.Namespace}, found)
	if err != nil {
		if !k8sErrors.IsNotFound(err) {
			logrus.Errorf("Failed to get configMap: %v", err)
			return false, err
		}
		err = c.Create(ctx, configMap)
		if err != nil {
			logrus.Errorf("Failed to create configMap: %v", err)
			return false, err
		}
		return true, nil
	}

	configMap.ObjectMeta.ResourceVersion = found.ObjectMeta.ResourceVersion

	if reflect.DeepEqual(found.Data, configMap.Data) && reflect.DeepEqual(found.BinaryData, configMap.BinaryData) && reflect.DeepEqual(found.Labels, configMap.Labels) {
		return false, nil
	}

	err = c.Update(ctx, configMap)
	if err != nil {
		logrus.Errorf("Failed to update configMap: %v", err)
		return false, err
	}
	return true, nil
}
