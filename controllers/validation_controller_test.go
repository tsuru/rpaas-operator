// Copyright 2024 tsuru authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package controllers

import (
	"testing"

	"github.com/stretchr/testify/assert"
	nginxv1alpha1 "github.com/tsuru/nginx-operator/api/v1alpha1"
	"github.com/tsuru/rpaas-operator/api/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/ptr"
)

func TestNewValidationPod(t *testing.T) {
	pod := newValidationPod(&v1alpha1.RpaasValidation{
		ObjectMeta: metav1.ObjectMeta{
			Name: "valid",
			UID:  types.UID("blah"),
		},
		Spec: v1alpha1.RpaasInstanceSpec{},
	}, "hash",
		&v1alpha1.RpaasPlan{},
		&corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{
				Name: "valid-config",
			},
		},
	)

	assert.Equal(t, &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name: "valid-config",
			Annotations: map[string]string{
				"rpaas.extensions.tsuru.io/validation-hash": "hash",
			},
			OwnerReferences: []metav1.OwnerReference{
				{
					APIVersion:         "extensions.tsuru.io/v1alpha1",
					Kind:               "RpaasValidation",
					Name:               "valid",
					UID:                types.UID("blah"),
					Controller:         ptr.To(true),
					BlockOwnerDeletion: ptr.To(true),
				},
			},
		},
		Spec: corev1.PodSpec{
			Volumes: []corev1.Volume{
				{
					Name: "nginx-config",
					VolumeSource: corev1.VolumeSource{
						ConfigMap: &corev1.ConfigMapVolumeSource{
							LocalObjectReference: corev1.LocalObjectReference{
								Name: "valid-config",
							},
							Optional: ptr.To(false),
						},
					},
				},
			},
			Containers: []corev1.Container{
				{
					Name: "validation",
					Command: []string{
						"/bin/sh",
						"-c",
						"nginx -t 2> /dev/termination-log",
					},
					VolumeMounts: []corev1.VolumeMount{
						{
							Name:      "nginx-config",
							MountPath: "/etc/nginx/nginx.conf",
							SubPath:   "nginx.conf",
							ReadOnly:  true,
						},
					},
				},
			},

			RestartPolicy: "Never",
		},
	}, pod)
}

func TestNewValidationPodFullFeatured(t *testing.T) {
	pod := newValidationPod(
		&v1alpha1.RpaasValidation{
			ObjectMeta: metav1.ObjectMeta{
				Name: "valid",
				UID:  types.UID("blah"),
			},
			Spec: v1alpha1.RpaasInstanceSpec{
				Files: []v1alpha1.File{
					{
						Name: "myfile",
						ConfigMap: &corev1.ConfigMapKeySelector{
							Key: "myfile",
							LocalObjectReference: corev1.LocalObjectReference{
								Name: "myfile",
							},
						},
					},
				},
				TLS: []nginxv1alpha1.NginxTLS{
					{
						SecretName: "secret-tls",
						Hosts: []string{
							"host1",
							"host2",
						},
					},
				},

				TLSSessionResumption: &v1alpha1.TLSSessionResumption{
					SessionTicket: &v1alpha1.TLSSessionTicket{
						KeepLastKeys: 2,
					},
				},
			},
		},
		"hash",
		&v1alpha1.RpaasPlan{
			Spec: v1alpha1.RpaasPlanSpec{
				Config: v1alpha1.NginxConfig{
					CacheEnabled: ptr.To(true),
					CachePath:    "/var/cache",
				},
			},
		},
		&corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{
				Name: "valid-config",
			},
		},
	)

	assert.Equal(t, &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name: "valid-config",
			Annotations: map[string]string{
				"rpaas.extensions.tsuru.io/validation-hash": "hash",
			},
			OwnerReferences: []metav1.OwnerReference{
				{
					APIVersion:         "extensions.tsuru.io/v1alpha1",
					Kind:               "RpaasValidation",
					Name:               "valid",
					UID:                types.UID("blah"),
					Controller:         ptr.To(true),
					BlockOwnerDeletion: ptr.To(true),
				},
			},
		},
		Spec: corev1.PodSpec{
			Volumes: []corev1.Volume{
				{
					Name: "extra-files-0",
					VolumeSource: corev1.VolumeSource{
						ConfigMap: &corev1.ConfigMapVolumeSource{
							LocalObjectReference: corev1.LocalObjectReference{
								Name: "myfile",
							},
							Optional: ptr.To(false),
						},
					},
				},
				{
					Name: "nginx-config",
					VolumeSource: corev1.VolumeSource{
						ConfigMap: &corev1.ConfigMapVolumeSource{
							LocalObjectReference: corev1.LocalObjectReference{
								Name: "valid-config",
							},
							Optional: ptr.To(false),
						},
					},
				},
				{
					Name: "cache-vol",
					VolumeSource: corev1.VolumeSource{
						EmptyDir: &corev1.EmptyDirVolumeSource{
							Medium: "Memory",
						},
					},
				},
				{
					Name: "nginx-certs-0",
					VolumeSource: corev1.VolumeSource{
						Secret: &corev1.SecretVolumeSource{
							SecretName: "secret-tls",
							Optional:   ptr.To(false),
						},
					},
				},
				{
					Name: "tls-session-tickets",
					VolumeSource: corev1.VolumeSource{
						Secret: &corev1.SecretVolumeSource{
							SecretName: "valid-session-tickets",
							Optional:   ptr.To(false),
						},
					},
				},
			},
			Containers: []corev1.Container{
				{
					Name: "validation",
					Command: []string{
						"/bin/sh",
						"-c",
						"nginx -t 2> /dev/termination-log",
					},
					VolumeMounts: []corev1.VolumeMount{
						{
							Name:      "extra-files-0",
							MountPath: "/etc/nginx/extra_files/myfile",
							SubPath:   "myfile",
							ReadOnly:  true,
						},
						{
							Name:      "nginx-config",
							MountPath: "/etc/nginx/nginx.conf",
							SubPath:   "nginx.conf",
							ReadOnly:  true,
						},
						{
							Name:      "cache-vol",
							MountPath: "/var/cache",
							ReadOnly:  false,
						},
						{
							Name:      "nginx-certs-0",
							MountPath: "/etc/nginx/certs/secret-tls",
							ReadOnly:  true,
						},
						{
							Name:      "tls-session-tickets",
							MountPath: "/etc/nginx/tickets",
							ReadOnly:  true,
						},
					},
				},
			},

			RestartPolicy: "Never",
		},
	}, pod)
}
