// Copyright 2021 tsuru authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package certificates

import (
	"context"
	"fmt"
	"reflect"

	"github.com/jetstack/cert-manager/pkg/util/pki"
	nginxv1alpha1 "github.com/tsuru/nginx-operator/api/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/tsuru/rpaas-operator/api/v1alpha1"
	"github.com/tsuru/rpaas-operator/pkg/util"
)

const CertificateNameLabel = "rpaas.extensions.tsuru.io/certificate-name"

var (
	ErrTLSSecretNotFound      = fmt.Errorf("TLS secret not found")
	ErrTooManyTLSSecretsFound = fmt.Errorf("too many TLS secrets found")
)

func ReconcileDynamicCertificates(ctx context.Context, client client.Client, instance, instanceMergedWithFlavors *v1alpha1.RpaasInstance) error {
	// NOTE: for now, we've only a way to obtain automatic certificates but it can
	// be useful if we had more options in the future.
	return reconcileCertManager(ctx, client, instance, instanceMergedWithFlavors)
}

func UpdateCertificateFromSecret(ctx context.Context, c client.Client, instance *v1alpha1.RpaasInstance, certificateName, secretName string) error {
	if c == nil {
		return fmt.Errorf("kubernetes client cannot be nil")
	}

	if instance == nil {
		return fmt.Errorf("rpaasinstance cannot be nil")
	}

	var s corev1.Secret
	err := c.Get(ctx, types.NamespacedName{Name: secretName, Namespace: instance.Namespace}, &s)
	if err != nil {
		return err
	}

	originalSecret := s.DeepCopy()

	if s.Labels == nil {
		s.Labels = make(map[string]string)
	}

	s.Labels[CertificateNameLabel] = certificateName
	s.Labels["rpaas.extensions.tsuru.io/instance-name"] = instance.Name

	if !reflect.DeepEqual(originalSecret.Labels, s.Labels) {
		if err = c.Update(ctx, &s); err != nil {
			return err
		}
	}

	return updateInstanceWithCertificateInfos(ctx, c, instance, &s)
}

func UpdateCertificate(ctx context.Context, c client.Client, instance *v1alpha1.RpaasInstance, certificateName string, certData, keyData []byte) error {
	if c == nil {
		return fmt.Errorf("kubernetes client cannot be nil")
	}

	if instance == nil {
		return fmt.Errorf("rpaasinstance cannot be nil")
	}

	s, err := getTLSSecretByCertificateName(ctx, c, instance, certificateName)
	switch err {
	case ErrTLSSecretNotFound:
		s = newSecretForCertificates(instance, certificateName, certData, keyData)
		if err = c.Create(ctx, s); err != nil {
			return err
		}

	case nil:
		originalSecret := s.DeepCopy()

		if s.Labels == nil {
			s.Labels = make(map[string]string)
		}

		s.Labels[CertificateNameLabel] = certificateName
		s.Labels["rpaas.extensions.tsuru.io/instance-name"] = instance.Name

		s.Data = map[string][]byte{
			corev1.TLSCertKey:       certData,
			corev1.TLSPrivateKeyKey: keyData,
		}

		if reflect.DeepEqual(s.Labels, originalSecret.Labels) && reflect.DeepEqual(s.Data, originalSecret.Data) {
			break
		}

		if err = c.Update(ctx, s); err != nil {
			return err
		}

	default:
		return err
	}

	return updateInstanceWithCertificateInfos(ctx, c, instance, s)

}

func DeleteCertificate(ctx context.Context, c client.Client, instance *v1alpha1.RpaasInstance, certificateName string) error {
	if c == nil {
		return fmt.Errorf("kubernetes client cannot be nil")
	}

	if instance == nil {
		return fmt.Errorf("rpaasinstance cannot be nil")
	}

	if certificateName == "" {
		return fmt.Errorf("certificate name cannot be empty")
	}

	s, err := getTLSSecretByCertificateName(ctx, c, instance, certificateName)
	if err != nil && err == ErrTLSSecretNotFound {
		return fmt.Errorf("certificate %q does not exist", certificateName)
	}

	if err != nil {
		return err
	}

	if index, found := findCertificate(instance, s.Name); found {
		instance.Spec.TLS = append(instance.Spec.TLS[:index], instance.Spec.TLS[index+1:]...) // removes the i-th element
	}

	delete(instance.Spec.PodTemplate.Annotations, fmt.Sprintf("rpaas.extensions.tsuru.io/%s-certificate-sha256", certificateName))
	delete(instance.Spec.PodTemplate.Annotations, fmt.Sprintf("rpaas.extensions.tsuru.io/%s-key-sha256", certificateName))

	if err = c.Update(ctx, instance); err != nil {
		return err
	}

	return c.Delete(ctx, s)
}

func updateInstanceWithCertificateInfos(ctx context.Context, c client.Client, i *v1alpha1.RpaasInstance, s *corev1.Secret) error {
	hosts, err := extractDNSNames(s.Data[corev1.TLSCertKey])
	if err != nil {
		return err
	}

	original := i.DeepCopy()

	if index, found := findCertificate(i, s.Name); found {
		i.Spec.TLS[index].Hosts = hosts
	} else {
		i.Spec.TLS = append(i.Spec.TLS, nginxv1alpha1.NginxTLS{
			SecretName: s.Name,
			Hosts:      hosts,
		})
	}

	if i.Spec.PodTemplate.Annotations == nil {
		i.Spec.PodTemplate.Annotations = make(map[string]string)
	}

	certName := s.Labels[CertificateNameLabel]

	i.Spec.PodTemplate.Annotations[fmt.Sprintf("rpaas.extensions.tsuru.io/%s-certificate-sha256", certName)] = util.SHA256(s.Data[corev1.TLSCertKey])
	i.Spec.PodTemplate.Annotations[fmt.Sprintf("rpaas.extensions.tsuru.io/%s-key-sha256", certName)] = util.SHA256(s.Data[corev1.TLSPrivateKeyKey])

	if reflect.DeepEqual(i.Spec.PodTemplate.Annotations, original.Spec.PodTemplate.Annotations) && reflect.DeepEqual(i.Spec.TLS, original.Spec.TLS) {
		return nil
	}

	return c.Update(ctx, i)
}

func getTLSSecretByCertificateName(ctx context.Context, c client.Client, instance *v1alpha1.RpaasInstance, certName string) (*corev1.Secret, error) {
	var sl corev1.SecretList
	err := c.List(ctx, &sl, &client.ListOptions{
		LabelSelector: labels.Set{
			CertificateNameLabel:                      certName,
			"rpaas.extensions.tsuru.io/instance-name": instance.Name,
		}.AsSelector(),
		Namespace: instance.Namespace,
	})

	if err != nil {
		return nil, err
	}

	switch len(sl.Items) {
	case 0:
		return nil, ErrTLSSecretNotFound

	case 1:
		return &sl.Items[0], nil

	default:
		return nil, ErrTooManyTLSSecretsFound
	}
}

func newSecretForCertificates(instance *v1alpha1.RpaasInstance, certName string, certData, keyData []byte) *corev1.Secret {
	return &corev1.Secret{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "v1",
			Kind:       "Secret",
		},
		ObjectMeta: metav1.ObjectMeta{
			GenerateName: fmt.Sprintf("%s-certs-", instance.Name),
			Namespace:    instance.Namespace,
			OwnerReferences: []metav1.OwnerReference{
				*metav1.NewControllerRef(instance, schema.GroupVersionKind{
					Group:   v1alpha1.GroupVersion.Group,
					Version: v1alpha1.GroupVersion.Version,
					Kind:    "RpaasInstance",
				}),
			},
			Labels: map[string]string{
				CertificateNameLabel:                      certName,
				"rpaas.extensions.tsuru.io/instance-name": instance.Name,
			},
		},
		Type: corev1.SecretTypeTLS,
		Data: map[string][]byte{
			corev1.TLSCertKey:       certData,
			corev1.TLSPrivateKeyKey: keyData,
		},
	}
}

func findCertificate(instance *v1alpha1.RpaasInstance, secretName string) (int, bool) {
	for i, t := range instance.Spec.TLS {
		if t.SecretName == secretName {
			return i, true
		}
	}

	return -1, false
}

func extractDNSNames(rawCert []byte) ([]string, error) {
	certs, err := pki.DecodeX509CertificateChainBytes(rawCert)
	if err != nil {
		return nil, err
	}

	leaf := certs[len(certs)-1]

	return leaf.DNSNames, nil
}
