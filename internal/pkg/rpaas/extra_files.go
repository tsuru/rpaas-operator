// Copyright 2022 tsuru authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package rpaas

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"regexp"
	"sort"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/tsuru/rpaas-operator/api/v1alpha1"
)

const maxFileSize = int(1 << 20) // 1MiB

var (
	ErrNoSuchExtraFile        = &NotFoundError{Msg: "extra file not found"}
	ErrExtraFileAlreadyExists = &ConflictError{Msg: "file already exists"}
)

func (m *k8sRpaasManager) GetExtraFiles(ctx context.Context, instanceName string) ([]File, error) {
	i, err := m.GetInstance(ctx, instanceName)
	if err != nil {
		return nil, err
	}

	var files []File
	for filename := range i.Spec.Files {
		cm, err := m.getConfigMapByFileName(ctx, i, filename)
		if err != nil {
			return nil, err
		}

		files = append(files, File{Name: filename, Content: cm.BinaryData[filename]})
	}

	sort.Slice(files, func(i, j int) bool { return files[i].Name < files[j].Name })

	return files, nil
}

func (m *k8sRpaasManager) CreateExtraFiles(ctx context.Context, instanceName string, files ...File) error {
	return m.addOrUpdateExtraFiles(ctx, instanceName, files, true)
}

func (m *k8sRpaasManager) UpdateExtraFiles(ctx context.Context, instanceName string, files ...File) error {
	return m.addOrUpdateExtraFiles(ctx, instanceName, files, false)
}

func (m *k8sRpaasManager) addOrUpdateExtraFiles(ctx context.Context, instanceName string, files []File, creating bool) error {
	if err := validateFiles(files); err != nil {
		return err
	}

	i, err := m.GetInstance(ctx, instanceName)
	if err != nil {
		return err
	}

	original := i.DeepCopy()

	for _, f := range files {
		_, found := i.Spec.Files[f.Name]
		if creating && found {
			return ErrExtraFileAlreadyExists
		}

		if !creating && !found {
			return ErrNoSuchExtraFile
		}

		// NOTE(nettoclaudio): Since the data stored in a ConfigMap cannot exceed 1MiB
		// we should limit a file for ConfigMap to support greater file contents.
		//
		// See: https://kubernetes.io/docs/concepts/configuration/configmap/#motivation
		cm, err := m.updateFileInConfigMap(ctx, i, f)
		if err != nil {
			return err
		}

		if i.Spec.Files == nil {
			i.Spec.Files = make(map[string]v1alpha1.Value)
		}

		i.Spec.Files[f.Name] = v1alpha1.Value{ValueFrom: &v1alpha1.ValueSource{
			ConfigMapKeyRef: &corev1.ConfigMapKeySelector{
				LocalObjectReference: corev1.LocalObjectReference{Name: cm.Name},
				Key:                  f.Name,
			},
		}}
	}

	if i.Spec.PodTemplate.Annotations == nil {
		i.Spec.PodTemplate.Annotations = make(map[string]string)
	}

	// we should ensure rollout of pods even for file updates
	i.Spec.PodTemplate.Annotations[fmt.Sprintf("%s/extra-files-last-update", defaultKeyLabelPrefix)] = time.Now().UTC().String()

	return m.patchInstance(ctx, original, i)
}

func (m *k8sRpaasManager) updateFileInConfigMap(ctx context.Context, i *v1alpha1.RpaasInstance, f File) (*corev1.ConfigMap, error) {
	newConfigMap := newConfigMapForFile(i, f)

	existingConfigMap, err := m.getConfigMapByFileName(ctx, i, f.Name)
	if errors.Is(err, ErrNoSuchExtraFile) {
		if err = m.cli.Create(ctx, newConfigMap); err != nil {
			return nil, err
		}

		return newConfigMap, nil
	}

	if err != nil {
		return nil, err
	}

	if reflect.DeepEqual(existingConfigMap.BinaryData, newConfigMap.BinaryData) {
		return nil, &NotModifiedError{Msg: fmt.Sprintf("no changes found in %q file", f.Name)}
	}

	existingConfigMap.BinaryData = newConfigMap.BinaryData

	if err = m.cli.Update(ctx, existingConfigMap); err != nil {
		return nil, err
	}

	return existingConfigMap, nil
}

func (m *k8sRpaasManager) getConfigMapByFileName(ctx context.Context, i *v1alpha1.RpaasInstance, filename string) (*corev1.ConfigMap, error) {
	var cms corev1.ConfigMapList
	if err := m.cli.List(ctx, &cms, &client.ListOptions{
		Namespace:     i.Namespace,
		LabelSelector: labels.SelectorFromSet(labels.Set(labelsSelectorForFile(filename))),
	}); err != nil {
		return nil, err
	}

	switch len(cms.Items) {
	case 0:
		return nil, ErrNoSuchExtraFile

	case 1:
		return cms.Items[0].DeepCopy(), nil

	default:
		return nil, &ConflictError{Msg: fmt.Sprintf("too many config maps for %q file", filename)}
	}
}

func newConfigMapForFile(i *v1alpha1.RpaasInstance, f File) *corev1.ConfigMap {
	return &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			GenerateName: fmt.Sprintf("%s-extra-files-", i.Name),
			Namespace:    i.Namespace,
			Labels:       labelsSelectorForFile(f.Name),
			OwnerReferences: []metav1.OwnerReference{
				*metav1.NewControllerRef(i, schema.GroupVersionKind{
					Group:   v1alpha1.GroupVersion.Group,
					Version: v1alpha1.GroupVersion.Version,
					Kind:    "RpaasInstance",
				}),
			},
		},
		BinaryData: map[string][]byte{f.Name: f.Content},
	}
}

func labelsSelectorForFile(filename string) map[string]string {
	return map[string]string{
		fmt.Sprintf("%s/is-file", defaultKeyLabelPrefix):   "true",
		fmt.Sprintf("%s/file-name", defaultKeyLabelPrefix): filename,
	}
}

func validateFiles(fs []File) error {
	if len(fs) == 0 {
		return &ValidationError{Msg: "you must provide a file"}
	}

	for _, f := range fs {
		if err := validateFile(f); err != nil {
			return err
		}
	}

	return nil
}

func validateFile(f File) error {
	if !isFileNameValid(f.Name) {
		return &ValidationError{Msg: fmt.Sprintf("file name %q is not valid (regular expression applied: %s)", f.Name, basePathRegexp)}
	}

	if len(f.Content) == 0 {
		return &ValidationError{Msg: fmt.Sprintf("file %q cannot be empty", f.Name)}
	}

	if len(f.Content) > maxFileSize {
		return &ValidationError{Msg: fmt.Sprintf("file %q exceeds the max size of %v bytes", f.Name, maxFileSize)}
	}

	return nil
}

var basePathRegexp = regexp.MustCompile("^[a-zA-Z0-9][^/ ]+$")

func isFileNameValid(filename string) bool { return basePathRegexp.MatchString(filename) }
