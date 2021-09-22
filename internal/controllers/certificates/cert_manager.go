// Copyright 2021 tsuru authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package certificates

import (
	"context"
	"errors"
	"fmt"
	"strings"

	cmv1 "github.com/jetstack/cert-manager/pkg/apis/certmanager/v1"
	cmmeta "github.com/jetstack/cert-manager/pkg/apis/meta/v1"
	corev1 "k8s.io/api/core/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/tsuru/rpaas-operator/api/v1alpha1"
)

const CertManagerCertificateName string = "cert-manager"

func reconcileCertManager(ctx context.Context, client client.Client, instance, instanceMergedWithFlavors *v1alpha1.RpaasInstance) error {
	if instanceMergedWithFlavors.Spec.DynamicCertificates == nil || instanceMergedWithFlavors.Spec.DynamicCertificates.CertManager == nil {
		return deleteCertManager(ctx, client, instance)
	}

	issuer, err := getCertManagerIssuer(ctx, client, instanceMergedWithFlavors)
	if err != nil {
		return err
	}

	cert, err := getCertificate(ctx, client, instance)
	if err != nil && k8serrors.IsNotFound(err) {
		cert, err = newCertificate(instanceMergedWithFlavors, issuer)
		if err != nil {
			return err
		}

		return client.Create(ctx, cert)
	}

	newCert, err := newCertificate(instanceMergedWithFlavors, issuer)
	if err != nil {
		return err
	}
	newCert.ResourceVersion = cert.ResourceVersion

	if err = client.Update(ctx, newCert); err != nil {
		return err
	}

	if !isCertificateReady(cert) {
		return nil
	}

	return UpdateCertificateFromSecret(ctx, client, instance, CertManagerCertificateName, newCert.Spec.SecretName)
}

func deleteCertManager(ctx context.Context, client client.Client, instance *v1alpha1.RpaasInstance) error {
	cert, err := getCertificate(ctx, client, instance)
	if k8serrors.IsNotFound(err) {
		return nil
	}

	if err != nil {
		return err
	}

	if err = client.Delete(ctx, cert); err != nil {
		return err
	}

	var s corev1.Secret
	err = client.Get(ctx, types.NamespacedName{
		Name:      cert.Spec.SecretName,
		Namespace: cert.Namespace,
	}, &s)

	if err != nil && !k8serrors.IsNotFound(err) {
		return err
	}

	if err == nil {
		if err = client.Delete(ctx, &s); err != nil {
			return err
		}
	}

	return DeleteCertificate(ctx, client, instance, CertManagerCertificateName)
}

func getCertificate(ctx context.Context, client client.Client, instance *v1alpha1.RpaasInstance) (*cmv1.Certificate, error) {
	var cert cmv1.Certificate
	return &cert, client.Get(ctx, types.NamespacedName{
		Name:      instance.Name,
		Namespace: instance.Namespace,
	}, &cert)
}

func newCertificate(instanceMergedWithFlavors *v1alpha1.RpaasInstance, issuer *cmmeta.ObjectReference) (*cmv1.Certificate, error) {
	dnsNames := instanceMergedWithFlavors.Spec.DynamicCertificates.CertManager.DNSNames

	if len(dnsNames) == 0 && instanceMergedWithFlavors.Spec.DynamicCertificates.CertManager.DNSNamesDefault {
		if instanceMergedWithFlavors.Spec.DNS == nil {
			return nil, errors.New("DNS Spec is not specified")
		}

		dnsNames = []string{
			fmt.Sprintf("%s.%s", instanceMergedWithFlavors.Name, instanceMergedWithFlavors.Spec.DNS.Zone),
		}
	}

	return &cmv1.Certificate{
		ObjectMeta: metav1.ObjectMeta{
			Name:      instanceMergedWithFlavors.Name,
			Namespace: instanceMergedWithFlavors.Namespace,
			OwnerReferences: []metav1.OwnerReference{
				*metav1.NewControllerRef(instanceMergedWithFlavors, schema.GroupVersionKind{
					Group:   v1alpha1.GroupVersion.Group,
					Version: v1alpha1.GroupVersion.Version,
					Kind:    "RpaasInstance",
				}),
			},
		},
		Spec: cmv1.CertificateSpec{
			IssuerRef:   *issuer,
			DNSNames:    dnsNames,
			IPAddresses: instanceMergedWithFlavors.Spec.DynamicCertificates.CertManager.IPAddresses,
			SecretName:  fmt.Sprintf("%s-cert-manager", instanceMergedWithFlavors.Name),
		},
	}, nil
}

func getCertManagerIssuer(ctx context.Context, client client.Client, instance *v1alpha1.RpaasInstance) (*cmmeta.ObjectReference, error) {
	if strings.Contains(instance.Spec.DynamicCertificates.CertManager.Issuer, ".") {
		return getCustomCertManagerIssuer(ctx, client, instance)
	}

	var issuer cmv1.Issuer
	err := client.Get(ctx, types.NamespacedName{
		Name:      instance.Spec.DynamicCertificates.CertManager.Issuer,
		Namespace: instance.Namespace,
	}, &issuer)

	if err != nil && !k8serrors.IsNotFound(err) {
		return nil, err
	}

	if err == nil {
		return &cmmeta.ObjectReference{
			Group: cmv1.SchemeGroupVersion.Group,
			Kind:  issuer.Kind,
			Name:  issuer.Name,
		}, nil
	}

	var clusterIssuer cmv1.ClusterIssuer
	err = client.Get(ctx, types.NamespacedName{
		Name: instance.Spec.DynamicCertificates.CertManager.Issuer,
	}, &clusterIssuer)

	if k8serrors.IsNotFound(err) {
		return nil, fmt.Errorf("there is no %q certificate issuer", instance.Spec.DynamicCertificates.CertManager.Issuer)
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

func getCustomCertManagerIssuer(ctx context.Context, client client.Client, instance *v1alpha1.RpaasInstance) (*cmmeta.ObjectReference, error) {
	parts := strings.SplitN(instance.Spec.DynamicCertificates.CertManager.Issuer, ".", 3)
	if len(parts) < 3 {
		return nil, fmt.Errorf("missing information to retrieve custom Cert Manager issuer: (requires <resource name>.<resource kind>.<resource group>, got %s)", instance.Spec.DynamicCertificates.CertManager.Issuer)
	}

	name, kind, group := parts[0], parts[1], parts[2]

	mapping, err := client.RESTMapper().RESTMapping(schema.GroupKind{Group: group, Kind: kind})
	if err != nil {
		return nil, err
	}

	u := &unstructured.Unstructured{}
	u.Object = map[string]interface{}{}
	u.SetGroupVersionKind(schema.GroupVersionKind{
		Group:   mapping.GroupVersionKind.Group,
		Kind:    mapping.GroupVersionKind.Kind,
		Version: mapping.GroupVersionKind.Version,
	})

	err = client.Get(ctx, types.NamespacedName{Name: name, Namespace: instance.Namespace}, u)
	if err != nil && !k8serrors.IsNotFound(err) {
		return nil, err
	}

	if err == nil {
		return &cmmeta.ObjectReference{
			Group: mapping.GroupVersionKind.Group,
			Kind:  mapping.GroupVersionKind.Kind,
			Name:  name,
		}, nil
	}

	err = client.Get(ctx, types.NamespacedName{Name: name}, u)
	if err != nil && !k8serrors.IsNotFound(err) {
		return nil, err
	}

	if k8serrors.IsNotFound(err) {
		return nil, fmt.Errorf("there is no %q certificate issuer", instance.Spec.DynamicCertificates.CertManager.Issuer)
	}

	return &cmmeta.ObjectReference{
		Group: mapping.GroupVersionKind.Group,
		Kind:  mapping.GroupVersionKind.Kind,
		Name:  name,
	}, nil
}

func isCertificateReady(cert *cmv1.Certificate) bool {
	for _, c := range cert.Status.Conditions {
		if c.Type == cmv1.CertificateConditionReady && c.Status == cmmeta.ConditionTrue {
			return true
		}
	}

	return false
}
