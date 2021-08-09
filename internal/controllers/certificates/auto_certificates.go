// Copyright 2021 tsuru authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package certificates

import (
	"context"
	"fmt"

	nginxv1alpha1 "github.com/tsuru/nginx-operator/api/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/tsuru/rpaas-operator/api/v1alpha1"
	"github.com/tsuru/rpaas-operator/pkg/util"
)

func RencocileAutoCertificates(ctx context.Context, client client.Client, instance *v1alpha1.RpaasInstance) error {
	if ctx == nil {
		return fmt.Errorf("context cannot be nil")
	}

	if client == nil {
		return fmt.Errorf("kubernetes cliente cannot be nil")
	}

	if instance == nil {
		return fmt.Errorf("rpaasinstance cannot be nil")
	}

	return reconcileAutoCertificates(ctx, client, instance)
}

func reconcileAutoCertificates(ctx context.Context, client client.Client, instance *v1alpha1.RpaasInstance) error {
	// NOTE: for now, we've only a way to obtain automatic certificates but it can
	// be useful if we had more options in the future.
	return reconcileCertManager(ctx, client, instance)
}

func UpdateCertificates(ctx context.Context, c client.Client, instance *v1alpha1.RpaasInstance, certificateName string, certData, keyData []byte) error {
	if c == nil {
		return fmt.Errorf("kubernetes client cannot be nil")
	}

	if instance == nil {
		return fmt.Errorf("rpaasinstance cannot be nil")
	}

	var s corev1.Secret
	err := c.Get(ctx, types.NamespacedName{
		Name:      secretNameForCertificates(instance),
		Namespace: instance.Namespace,
	}, &s)

	if err != nil && k8serrors.IsNotFound(err) {
		s = *new(corev1.Secret)
		newSecretForCertificates(instance).DeepCopyInto(&s)

		if err = c.Create(ctx, &s); err != nil {
			return err
		}
	}

	if err != nil {
		return err
	}

	certLabel, keyLabel := fmt.Sprintf("%s.crt", certificateName), fmt.Sprintf("%s.key", certificateName)

	if s.Data == nil {
		s.Data = make(map[string][]byte)
	}

	s.Data[certLabel] = certData
	s.Data[keyLabel] = keyData

	if err = c.Update(ctx, &s); err != nil {
		return err
	}

	if instance.Spec.Certificates == nil {
		instance.Spec.Certificates = &nginxv1alpha1.TLSSecret{}
	}

	if instance.Spec.Certificates.SecretName == "" {
		instance.Spec.Certificates.SecretName = fmt.Sprintf("%s-certificates", instance.Name)
	}

	if hasCertificate(instance.Spec.Certificates.Items, certLabel) {
		return nil
	}

	instance.Spec.Certificates.Items = append(instance.Spec.Certificates.Items, nginxv1alpha1.TLSSecretItem{
		CertificateField: certLabel,
		KeyField:         keyLabel,
	})

	if instance.Spec.PodTemplate.Annotations == nil {
		instance.Spec.PodTemplate.Annotations = make(map[string]string)
	}

	instance.Spec.PodTemplate.Annotations["rpaas.extensions.tsuru.io/certificates-sha256-hash"] = util.SHA256(s.Data)

	return c.Update(ctx, instance)
}

func secretNameForCertificates(instance *v1alpha1.RpaasInstance) string {
	if instance.Spec.Certificates != nil {
		return instance.Spec.Certificates.SecretName
	}

	return fmt.Sprintf("%s-certificates", instance.Name)
}

func newSecretForCertificates(instance *v1alpha1.RpaasInstance) *corev1.Secret {
	return &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      secretNameForCertificates(instance),
			Namespace: instance.Namespace,
			OwnerReferences: []metav1.OwnerReference{
				*metav1.NewControllerRef(instance, schema.GroupVersionKind{
					Group:   v1alpha1.GroupVersion.Group,
					Version: v1alpha1.GroupVersion.Version,
					Kind:    "RpaasInstance",
				}),
			},
		},
	}
}

func hasCertificate(items []nginxv1alpha1.TLSSecretItem, certLabel string) bool {
	for _, i := range items {
		if certLabel == i.CertificateField {
			return true
		}
	}

	return false
}
