package util

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	rpaasv1alpha1 "github.com/tsuru/rpaas-operator/pkg/apis/extensions/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	k8sErrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func TestGetValue(t *testing.T) {
	scheme := runtime.NewScheme()
	corev1.AddToScheme(scheme)

	boolPtr := func(b bool) *bool {
		return &b
	}

	tests := []struct {
		name      string
		resources []runtime.Object
		namespace string
		value     *rpaasv1alpha1.Value
		assertion func(t *testing.T, v string, err error)
	}{
		{
			name: "when value is nil, should return an error",
			assertion: func(t *testing.T, v string, err error) {
				assert.Error(t, err)
				assert.Equal(t, fmt.Errorf("value cannot be nil"), err)
			},
		},
		{
			name: "when value has inline field",
			value: &rpaasv1alpha1.Value{
				Value: "# My expected string value",
			},
			assertion: func(t *testing.T, v string, err error) {
				assert.NoError(t, err)
				assert.Equal(t, "# My expected string value", v)
			},
		},
		{
			name: "when value comes from configmap",
			value: &rpaasv1alpha1.Value{
				ValueFrom: &rpaasv1alpha1.ValueSource{
					Namespace: "default",
					ConfigMapKeyRef: &corev1.ConfigMapKeySelector{
						LocalObjectReference: corev1.LocalObjectReference{
							Name: "my-configmap",
						},
						Key: "some-key",
					},
				},
			},
			resources: []runtime.Object{
				&corev1.ConfigMap{
					TypeMeta: metav1.TypeMeta{
						APIVersion: "v1",
						Kind:       "ConfigMap",
					},
					ObjectMeta: metav1.ObjectMeta{
						Name:      "my-configmap",
						Namespace: "default",
					},
					Data: map[string]string{
						"some-key": "# My expected string value from ConfigMap",
					},
				},
			},
			assertion: func(t *testing.T, v string, err error) {
				assert.NoError(t, err)
				assert.Equal(t, "# My expected string value from ConfigMap", v)
			},
		},
		{
			name: "when configmap not found and value is not optional",
			value: &rpaasv1alpha1.Value{
				ValueFrom: &rpaasv1alpha1.ValueSource{
					Namespace: "default",
					ConfigMapKeyRef: &corev1.ConfigMapKeySelector{
						LocalObjectReference: corev1.LocalObjectReference{
							Name: "unknown-configmap",
						},
						Key:      "some-key",
						Optional: boolPtr(false),
					},
				},
			},
			assertion: func(t *testing.T, v string, err error) {
				assert.Error(t, err)
				assert.True(t, k8sErrors.IsNotFound(err))
			},
		},
		{
			name: "when key is not found into configmap and value is not optional",
			resources: []runtime.Object{
				&corev1.ConfigMap{
					TypeMeta: metav1.TypeMeta{
						APIVersion: "v1",
						Kind:       "ConfigMap",
					},
					ObjectMeta: metav1.ObjectMeta{
						Name:      "my-configmap",
						Namespace: "default",
					},
					Data: map[string]string{
						"some-key": "# My expected string value from ConfigMap",
					},
				},
			},
			value: &rpaasv1alpha1.Value{
				ValueFrom: &rpaasv1alpha1.ValueSource{
					Namespace: "default",
					ConfigMapKeyRef: &corev1.ConfigMapKeySelector{
						LocalObjectReference: corev1.LocalObjectReference{
							Name: "my-configmap",
						},
						Key:      "unknown-key",
						Optional: boolPtr(false),
					},
				},
			},
			assertion: func(t *testing.T, v string, err error) {
				assert.Error(t, err)
				assert.Equal(t, fmt.Errorf("key \"unknown-key\" cannot be found in configmap default/my-configmap"), err)
			},
		},
		{
			name: "when default namespace is set, should try get ConfigMap from it",
			resources: []runtime.Object{
				&corev1.Namespace{
					TypeMeta: metav1.TypeMeta{
						APIVersion: "v1",
						Kind:       "Namespace",
					},
					ObjectMeta: metav1.ObjectMeta{
						Name: "another-namespace",
					},
				},
				&corev1.ConfigMap{
					TypeMeta: metav1.TypeMeta{
						APIVersion: "v1",
						Kind:       "ConfigMap",
					},
					ObjectMeta: metav1.ObjectMeta{
						Name:      "my-configmap",
						Namespace: "another-namespace",
					},
					Data: map[string]string{
						"some-key": "# My expected string value from ConfigMap",
					},
				},
			},
			namespace: "another-namespace",
			value: &rpaasv1alpha1.Value{
				ValueFrom: &rpaasv1alpha1.ValueSource{
					ConfigMapKeyRef: &corev1.ConfigMapKeySelector{
						LocalObjectReference: corev1.LocalObjectReference{
							Name: "my-configmap",
						},
						Key: "some-key",
					},
				},
			},
			assertion: func(t *testing.T, v string, err error) {
				assert.NoError(t, err)
				assert.Equal(t, "# My expected string value from ConfigMap", v)
			},
		},
		{
			name: "when both default namespace and ValueSource namespace are set, should try getting ConfigMap from the last one",
			resources: []runtime.Object{
				&corev1.ConfigMap{
					TypeMeta: metav1.TypeMeta{
						APIVersion: "v1",
						Kind:       "ConfigMap",
					},
					ObjectMeta: metav1.ObjectMeta{
						Name:      "my-configmap",
						Namespace: "default",
					},
					Data: map[string]string{
						"some-key": "# My expected string value from ConfigMap",
					},
				},
			},
			namespace: "another-namespace",
			value: &rpaasv1alpha1.Value{
				ValueFrom: &rpaasv1alpha1.ValueSource{
					Namespace: "default",
					ConfigMapKeyRef: &corev1.ConfigMapKeySelector{
						LocalObjectReference: corev1.LocalObjectReference{
							Name: "my-configmap",
						},
						Key: "some-key",
					},
				},
			},
			assertion: func(t *testing.T, v string, err error) {
				assert.NoError(t, err)
				assert.Equal(t, "# My expected string value from ConfigMap", v)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			k8sClient := fake.NewFakeClientWithScheme(scheme, tt.resources...)
			value, err := GetValue(nil, k8sClient, tt.namespace, tt.value)
			tt.assertion(t, value, err)
		})
	}
}
