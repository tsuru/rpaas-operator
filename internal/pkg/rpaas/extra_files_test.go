// Copyright 2022 tsuru authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package rpaas

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tsuru/rpaas-operator/api/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func Test_k8sRpaasManager_UpdateExtraFiles(t *testing.T) {
	tests := map[string]struct {
		resources     []runtime.Object
		instance      func(i *v1alpha1.RpaasInstance) *v1alpha1.RpaasInstance
		files         []File
		assert        func(t *testing.T, c client.Client, i *v1alpha1.RpaasInstance)
		expectedError string
	}{
		"no files presented": {
			expectedError: "you must provide a file",
		},

		"file name empty": {
			files:         []File{{}},
			expectedError: `file name "" is not valid (regular expression applied: ^[a-zA-Z0-9][^/ ]+$)`,
		},

		"file name w/ path separator": {
			files:         []File{{Name: "www/index.html"}},
			expectedError: `file name "www/index.html" is not valid (regular expression applied: ^[a-zA-Z0-9][^/ ]+$)`,
		},

		"file name w/ white spaces": {
			files:         []File{{Name: "My File.pdf"}},
			expectedError: `file name "My File.pdf" is not valid (regular expression applied: ^[a-zA-Z0-9][^/ ]+$)`,
		},

		"file content exceeds 1MiB": {
			files:         []File{{Name: "huge-file.txt", Content: []byte(strings.Repeat("A", 1048576+1))}},
			expectedError: `file "huge-file.txt" exceeds the max size of 1048576 bytes`,
		},

		"when file is ok, should create configmap and points to it in rpaasinstance.spec.files field": {
			files: []File{{Name: "index.html", Content: []byte("<h1>Hello world!</h1>")}},
			assert: func(t *testing.T, c client.Client, i *v1alpha1.RpaasInstance) {
				var cmList corev1.ConfigMapList
				err := c.List(context.TODO(), &cmList, &client.ListOptions{
					Namespace: i.Namespace,
					LabelSelector: labels.Set{
						"rpaas.extensions.tsuru.io/is-file":   "true",
						"rpaas.extensions.tsuru.io/file-name": "index.html",
					}.AsSelector(),
				})
				require.NoError(t, err)
				require.Len(t, cmList.Items, 1)

				cm := cmList.Items[0]

				assert.Equal(t, fmt.Sprintf("%s-extra-files-", i.Name), cm.GenerateName)
				assert.Equal(t, map[string]string{
					"rpaas.extensions.tsuru.io/is-file":   "true",
					"rpaas.extensions.tsuru.io/file-name": "index.html",
				}, cm.Labels)
				assert.Equal(t, map[string][]byte{"index.html": []byte("<h1>Hello world!</h1>")}, cm.BinaryData)

				err = c.Get(context.TODO(), types.NamespacedName{Name: i.Name, Namespace: i.Namespace}, i)
				require.NoError(t, err)
				assert.Equal(t, map[string]v1alpha1.Value{
					"index.html": {ValueFrom: &v1alpha1.ValueSource{
						ConfigMapKeyRef: &corev1.ConfigMapKeySelector{
							LocalObjectReference: corev1.LocalObjectReference{Name: cm.Name},
							Key:                  "index.html",
						},
					}},
				}, i.Spec.Files)
			},
		},

		"no changes found": {
			resources: []runtime.Object{
				&corev1.ConfigMap{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "my-instance-extra-files-abcde",
						Namespace: "rpaasv2",
						Labels: map[string]string{
							"rpaas.extensions.tsuru.io/is-file":   "true",
							"rpaas.extensions.tsuru.io/file-name": "index.html",
						},
					},
					BinaryData: map[string][]byte{"index.html": []byte("<h1>Hello world!</h1>")},
				},
			},
			instance: func(i *v1alpha1.RpaasInstance) *v1alpha1.RpaasInstance {
				i.Spec.Files = map[string]v1alpha1.Value{
					"index.html": {ValueFrom: &v1alpha1.ValueSource{
						ConfigMapKeyRef: &corev1.ConfigMapKeySelector{
							LocalObjectReference: corev1.LocalObjectReference{Name: "my-instance-extra-files-abcde"},
							Key:                  "index.html",
						},
					}},
				}
				return i
			},
			files:         []File{{Name: "index.html", Content: []byte("<h1>Hello world!</h1>")}},
			expectedError: `no changes found in "index.html" file`,
		},

		"updating the file content": {
			resources: []runtime.Object{
				&corev1.ConfigMap{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "my-instance-extra-files-abcde",
						Namespace: "rpaasv2",
						Labels: map[string]string{
							"rpaas.extensions.tsuru.io/is-file":   "true",
							"rpaas.extensions.tsuru.io/file-name": "index.html",
						},
					},
					BinaryData: map[string][]byte{"index.html": []byte("<h1>Hello world!</h1>")},
				},
			},
			instance: func(i *v1alpha1.RpaasInstance) *v1alpha1.RpaasInstance {
				i.Spec.Files = map[string]v1alpha1.Value{
					"index.html": {ValueFrom: &v1alpha1.ValueSource{
						ConfigMapKeyRef: &corev1.ConfigMapKeySelector{
							LocalObjectReference: corev1.LocalObjectReference{Name: "my-instance-extra-files-abcde"},
							Key:                  "index.html",
						},
					}},
				}
				return i
			},
			files: []File{{Name: "index.html", Content: []byte("<h1>Hello there!</h1>")}},
			assert: func(t *testing.T, c client.Client, i *v1alpha1.RpaasInstance) {
				var cm corev1.ConfigMap
				err := c.Get(context.TODO(), types.NamespacedName{Name: "my-instance-extra-files-abcde", Namespace: "rpaasv2"}, &cm)
				require.NoError(t, err)
				assert.Equal(t, map[string][]byte{"index.html": []byte(`<h1>Hello there!</h1>`)}, cm.BinaryData)
			},
		},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			instance := &v1alpha1.RpaasInstance{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "my-instance",
					Namespace: "rpaasv2",
				},
			}

			if tt.instance != nil {
				instance = tt.instance(instance)
			}

			resources := append(tt.resources, instance)

			manager := &k8sRpaasManager{
				cli: fake.NewClientBuilder().
					WithScheme(newScheme()).
					WithRuntimeObjects(resources...).
					Build(),
			}
			err := manager.UpdateExtraFiles(context.Background(), instance.Name, tt.files...)
			if tt.expectedError != "" {
				assert.EqualError(t, err, tt.expectedError)
				return
			}

			require.NoError(t, err)
			require.NotNil(t, tt.assert, "test case must provide an assert function")
			tt.assert(t, manager.cli, instance)
		})
	}
}
