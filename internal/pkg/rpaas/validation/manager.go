// Copyright 2024 tsuru authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
package validation

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/tsuru/rpaas-operator/api/v1alpha1"
	"github.com/tsuru/rpaas-operator/internal/pkg/rpaas"
	corev1 "k8s.io/api/core/v1"
	k8sErrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/klog/v2"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var _ rpaas.RpaasManager = &validationManager{}

type validationManager struct {
	rpaas.RpaasManager
	cli client.Client
}

func New(manager rpaas.RpaasManager, cli client.Client) rpaas.RpaasManager {
	return &validationManager{
		RpaasManager: manager,
		cli:          cli,
	}
}

var errNotImplementedYet = errors.New("not implemented yet")

func (v *validationManager) DeleteBlock(ctx context.Context, instanceName, blockName string) error {
	validation, err := v.validationCRD(ctx, instanceName)
	if err != nil {
		return err
	}

	err = rpaas.NewMutation(&validation.Spec).DeleteBlock(blockName)
	if err != nil {
		return err
	}

	err = v.waitController(ctx, validation)
	if err != nil {
		return err
	}

	return v.RpaasManager.DeleteBlock(ctx, instanceName, blockName)
}

func (v *validationManager) UpdateBlock(ctx context.Context, instanceName string, block rpaas.ConfigurationBlock) error {
	validation, err := v.validationCRD(ctx, instanceName)
	if err != nil {
		return err
	}

	err = rpaas.NewMutation(&validation.Spec).UpdateBlock(block)
	if err != nil {
		return err
	}

	err = v.waitController(ctx, validation)
	if err != nil {
		return err
	}

	return v.RpaasManager.UpdateBlock(ctx, instanceName, block)
}

func (v *validationManager) CreateExtraFiles(ctx context.Context, instanceName string, files ...rpaas.File) error {
	validation, err := v.validationCRD(ctx, instanceName)
	if err != nil {
		return err
	}

	tempConfigMaps := []*corev1.ConfigMap{}
	defer func() {
		for _, configMap := range tempConfigMaps {
			deleteErr := v.cli.Delete(ctx, configMap)
			if deleteErr != nil {
				klog.Error("could not delete temporary configmap", deleteErr)
			}
		}

	}()

	for _, f := range files {
		_, found := findFileByName(validation.Spec.Files, f.Name)
		if found {
			return rpaas.ErrExtraFileAlreadyExists
		}
	}

	if validation.Spec.Files == nil {
		validation.Spec.Files = make([]v1alpha1.File, 0, len(files))
	}

	for _, f := range files {
		configMap, configMapErr := v.createTemporaryFileInConfigMap(ctx, validation, f)
		if configMapErr != nil {
			return configMapErr
		}

		tempConfigMaps = append(tempConfigMaps, configMap)

		validation.Spec.Files = append(validation.Spec.Files, v1alpha1.File{
			Name: f.Name,
			ConfigMap: &corev1.ConfigMapKeySelector{
				LocalObjectReference: corev1.LocalObjectReference{Name: configMap.Name},
				Key:                  f.Name,
			},
		})
	}

	err = v.waitController(ctx, validation)
	if err != nil {
		return err
	}

	return v.RpaasManager.CreateExtraFiles(ctx, instanceName, files...)
}

func (v *validationManager) DeleteExtraFiles(ctx context.Context, instanceName string, filenames ...string) error {
	validation, err := v.validationCRD(ctx, instanceName)
	if err != nil {
		return err
	}

	for _, name := range filenames {
		if index, found := findFileByName(validation.Spec.Files, name); found {
			validation.Spec.Files = append(validation.Spec.Files[:index], validation.Spec.Files[index+1:]...)
		}
	}

	err = v.waitController(ctx, validation)
	if err != nil {
		return err
	}

	return v.RpaasManager.DeleteExtraFiles(ctx, instanceName, filenames...)
}

func (v *validationManager) UpdateExtraFiles(ctx context.Context, instanceName string, files ...rpaas.File) error {
	validation, err := v.validationCRD(ctx, instanceName)
	if err != nil {
		return err
	}

	tempConfigMaps := []*corev1.ConfigMap{}
	defer func() {
		for _, configMap := range tempConfigMaps {
			deleteErr := v.cli.Delete(ctx, configMap)
			if deleteErr != nil {
				klog.Error("could not delete temporary configmap", deleteErr)
			}
		}
	}()

	for _, f := range files {
		position, found := findFileByName(validation.Spec.Files, f.Name)
		if !found {
			return rpaas.ErrNoSuchExtraFile
		}

		configMap, configMapErr := v.createTemporaryFileInConfigMap(ctx, validation, f)
		if configMapErr != nil {
			return configMapErr
		}

		tempConfigMaps = append(tempConfigMaps, configMap)

		validation.Spec.Files[position] = v1alpha1.File{
			Name: f.Name,
			ConfigMap: &corev1.ConfigMapKeySelector{
				LocalObjectReference: corev1.LocalObjectReference{Name: configMap.Name},
				Key:                  f.Name,
			},
		}
	}

	err = v.waitController(ctx, validation)
	if err != nil {
		return err
	}

	return v.RpaasManager.UpdateExtraFiles(ctx, instanceName, files...)
}

func (m *validationManager) createTemporaryFileInConfigMap(ctx context.Context, validation *v1alpha1.RpaasValidation, f rpaas.File) (*corev1.ConfigMap, error) {
	newConfigMap := newConfigMapForFile(validation, f)
	if err := m.cli.Create(ctx, newConfigMap); err != nil {
		return nil, err
	}

	return newConfigMap, nil

}

func newConfigMapForFile(validation *v1alpha1.RpaasValidation, f rpaas.File) *corev1.ConfigMap {
	return &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			GenerateName: fmt.Sprintf("%s-validation-file-", validation.Name),
			Namespace:    validation.Namespace,
			// we need TO discover UID of validation early
			/*OwnerReferences: []metav1.OwnerReference{
				*metav1.NewControllerRef(validation, schema.GroupVersionKind{
					Group:   v1alpha1.GroupVersion.Group,
					Version: v1alpha1.GroupVersion.Version,
					Kind:    "RpaasValidation",
				}),
			},*/
		},
		BinaryData: map[string][]byte{f.Name: f.Content},
	}
}

func findFileByName(files []v1alpha1.File, filename string) (int, bool) {
	for i := range files {
		if files[i].Name == filename {
			return i, true
		}
	}

	return -1, false
}

func (v *validationManager) DeleteRoute(ctx context.Context, instanceName, path string) error {
	validation, err := v.validationCRD(ctx, instanceName)
	if err != nil {
		return err
	}

	err = rpaas.NewMutation(&validation.Spec).DeleteRoute(path)
	if err != nil {
		return err
	}

	err = v.waitController(ctx, validation)
	if err != nil {
		return err
	}

	return v.RpaasManager.DeleteRoute(ctx, instanceName, path)
}

func (v *validationManager) UpdateRoute(ctx context.Context, instanceName string, route rpaas.Route) error {
	validation, err := v.validationCRD(ctx, instanceName)
	if err != nil {
		return err
	}

	err = rpaas.NewMutation(&validation.Spec).UpdateRoute(route)
	if err != nil {
		return err
	}

	err = v.waitController(ctx, validation)
	if err != nil {
		return err
	}

	return v.RpaasManager.UpdateRoute(ctx, instanceName, route)
}

func (v *validationManager) BindApp(ctx context.Context, instanceName string, args rpaas.BindAppArgs) error {
	validation, err := v.validationCRD(ctx, instanceName)
	if err != nil {
		return err
	}

	internalBind := validation.BelongsToCluster(args.AppClusterName)
	err = rpaas.NewMutation(&validation.Spec).BindApp(args, internalBind)
	if err != nil {
		return err
	}

	err = v.waitController(ctx, validation)
	if err != nil {
		return err
	}

	return v.RpaasManager.BindApp(ctx, instanceName, args)
}

func (v *validationManager) UnbindApp(ctx context.Context, instanceName, appName string) error {
	validation, err := v.validationCRD(ctx, instanceName)
	if err != nil {
		return err
	}

	err = rpaas.NewMutation(&validation.Spec).UnbindApp(appName)
	if err != nil {
		return err
	}

	err = v.waitController(ctx, validation)
	if err != nil {
		return err
	}

	return v.RpaasManager.UnbindApp(ctx, instanceName, appName)
}

func (v *validationManager) validationCRD(ctx context.Context, instanceName string) (*v1alpha1.RpaasValidation, error) {
	instance, err := v.GetInstance(ctx, instanceName)
	if err != nil {
		return nil, err
	}

	return &v1alpha1.RpaasValidation{
		ObjectMeta: v1.ObjectMeta{
			Name:        instance.Name,
			Namespace:   instance.Namespace,
			Labels:      instance.Labels,
			Annotations: instance.Annotations,
		},
		Spec: instance.Spec,
	}, nil
}

func (v *validationManager) waitController(ctx context.Context, validation *v1alpha1.RpaasValidation) error {
	existingValidation := v1alpha1.RpaasValidation{}
	err := v.cli.Get(ctx, client.ObjectKeyFromObject(validation), &existingValidation)
	if err != nil {
		if !k8sErrors.IsNotFound(err) {
			return err
		}
		err = v.cli.Create(ctx, validation, &client.CreateOptions{})
		if err != nil {
			return err
		}
	} else {
		validation.ObjectMeta.ResourceVersion = existingValidation.ResourceVersion
		err = v.cli.Update(ctx, validation, &client.UpdateOptions{})
		if err != nil {
			return err
		}
	}

	maxRetries := 30

	for retry := 0; retry < maxRetries; retry++ {
		existingValidation = v1alpha1.RpaasValidation{}
		err = v.cli.Get(ctx, client.ObjectKeyFromObject(validation), &existingValidation)
		if err != nil {
			return err
		}

		isLastStatus := existingValidation.Generation == existingValidation.Status.ObservedGeneration

		if isLastStatus && existingValidation.Status.Valid != nil {
			if *existingValidation.Status.Valid {
				return nil
			}

			return &rpaas.ValidationError{Msg: existingValidation.Status.Error}
		}

		time.Sleep(time.Second)
	}

	return &rpaas.ValidationError{Msg: "rpaas-operator timeout"}
}
