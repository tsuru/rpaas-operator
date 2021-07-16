// Copyright 2021 tsuru authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package certificates

import (
	"context"
	"fmt"

	cmv1 "github.com/jetstack/cert-manager/pkg/apis/certmanager/v1"
	cmmeta "github.com/jetstack/cert-manager/pkg/apis/meta/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/tsuru/rpaas-operator/api/v1alpha1"
)

func reconcileCertManager(ctx context.Context, client client.Client, instance *v1alpha1.RpaasInstance) error {
	if instance.Spec.AutoCertificates == nil || instance.Spec.AutoCertificates.CertManager == nil {
		return deleteCertManager(ctx, client, instance)
	}

	issuer, err := getCertManagerIssuer(ctx, client, instance)
	if err != nil {
		return err
	}

	cert, err := getCertificate(ctx, client, instance)
	if err != nil && k8serrors.IsNotFound(err) {
		return client.Create(ctx, newCertificate(instance, issuer))
	}

	newCert := newCertificate(instance, issuer)
	newCert.ResourceVersion = cert.ResourceVersion

	return client.Update(ctx, newCert)
}

func deleteCertManager(ctx context.Context, client client.Client, instance *v1alpha1.RpaasInstance) error {
	cert, err := getCertificate(ctx, client, instance)
	if err != nil && k8serrors.IsNotFound(err) {
		return nil
	}

	if err != nil {
		return err
	}

	return client.Delete(ctx, cert)
}

func getCertificate(ctx context.Context, client client.Client, instance *v1alpha1.RpaasInstance) (*cmv1.Certificate, error) {
	var cert cmv1.Certificate

	err := client.Get(ctx, types.NamespacedName{
		Name:      instance.Name,
		Namespace: instance.Namespace,
	}, &cert)

	return &cert, err
}

func newCertificate(instance *v1alpha1.RpaasInstance, issuer *cmmeta.ObjectReference) *cmv1.Certificate {
	return &cmv1.Certificate{
		ObjectMeta: metav1.ObjectMeta{
			Name:      instance.Name,
			Namespace: instance.Namespace,
			OwnerReferences: []metav1.OwnerReference{
				*metav1.NewControllerRef(instance, schema.GroupVersionKind{
					Group:   v1alpha1.GroupVersion.Group,
					Version: v1alpha1.GroupVersion.Version,
					Kind:    "RpaasInstance",
				}),
			},
		},
		Spec: cmv1.CertificateSpec{
			IssuerRef:   *issuer,
			DNSNames:    instance.Spec.AutoCertificates.CertManager.DNSNames,
			IPAddresses: instance.Spec.AutoCertificates.CertManager.IPAddresses,
			SecretName:  fmt.Sprintf("%s-cert-manager", instance.Name),
		},
	}
}

func getCertManagerIssuer(ctx context.Context, client client.Client, instance *v1alpha1.RpaasInstance) (*cmmeta.ObjectReference, error) {
	var issuer cmv1.Issuer

	err := client.Get(ctx, types.NamespacedName{
		Name:      instance.Spec.AutoCertificates.CertManager.Issuer,
		Namespace: instance.Namespace,
	}, &issuer)

	if err == nil {
		return &cmmeta.ObjectReference{
			Group: cmv1.SchemeGroupVersion.Group,
			Kind:  issuer.Kind,
			Name:  issuer.Name,
		}, nil
	}

	if err != nil && !k8serrors.IsNotFound(err) {
		return nil, err
	}

	var clusterIssuer cmv1.ClusterIssuer

	err = client.Get(ctx, types.NamespacedName{
		Name: instance.Spec.AutoCertificates.CertManager.Issuer,
	}, &clusterIssuer)

	if err != nil && k8serrors.IsNotFound(err) {
		return nil, fmt.Errorf("there is no Issuer or ClusterIssuer with %q name", instance.Spec.AutoCertificates.CertManager.Issuer)
	}

	if err != nil {
		return nil, err
	}

	return &cmmeta.ObjectReference{
		Group: cmv1.SchemeGroupVersion.Group,
		Kind:  clusterIssuer.Kind,
		Name:  clusterIssuer.Name,
	}, nil
}
