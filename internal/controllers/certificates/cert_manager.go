// Copyright 2021 tsuru authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package certificates

import (
	"context"
	"fmt"
	"reflect"
	"strings"

	cmv1 "github.com/cert-manager/cert-manager/pkg/apis/certmanager/v1"
	cmmeta "github.com/cert-manager/cert-manager/pkg/apis/meta/v1"
	corev1 "k8s.io/api/core/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/tsuru/rpaas-operator/api/v1alpha1"
)

const CertManagerCertificateName string = "cert-manager"

func ReconcileCertManager(ctx context.Context, client client.Client, instance, instanceMergedWithFlavors *v1alpha1.RpaasInstance) ([]cmv1.Certificate, error) {
	err := removeOldCertificates(ctx, client, instance, instanceMergedWithFlavors)
	if err != nil {
		return nil, err
	}

	certManagerCerts := []cmv1.Certificate{}

	for _, req := range instanceMergedWithFlavors.Spec.CertManagerRequests(instance.Name) {
		issuer, err := getCertManagerIssuer(ctx, client, req, instanceMergedWithFlavors.Namespace)
		if err != nil {
			return nil, err
		}

		newCert, err := newCertificate(instanceMergedWithFlavors, issuer, req)
		if err != nil {
			return nil, err
		}

		var cert cmv1.Certificate
		err = client.Get(ctx, types.NamespacedName{Name: newCert.Name, Namespace: newCert.Namespace}, &cert)
		if err != nil && k8serrors.IsNotFound(err) {
			if err = takeOverPreviousSecret(ctx, client, instanceMergedWithFlavors, newCert); err != nil {
				return nil, err
			}

			if err = client.Create(ctx, newCert); err != nil {
				return nil, err
			}

			newCert.DeepCopyInto(&cert)
		}

		if !reflect.DeepEqual(cert.Spec, newCert.Spec) {
			newCert.ResourceVersion = cert.ResourceVersion

			if err = client.Update(ctx, newCert); err != nil {
				return nil, err
			}
		}

		if !isCertificateReady(&cert) {
			continue
		}

		certManagerCerts = append(certManagerCerts, cert)
	}

	return certManagerCerts, nil
}

func CertManagerCertificates(ctx context.Context, client client.Client, namespace, name string, spec *v1alpha1.RpaasInstanceSpec) ([]cmv1.Certificate, error) {
	certManagerCerts := []cmv1.Certificate{}

	for _, req := range spec.CertManagerRequests(name) {
		certName := CertManagerCertificateNameForInstance(name, req)

		var cert cmv1.Certificate
		err := client.Get(ctx, types.NamespacedName{Name: certName, Namespace: namespace}, &cert)
		if err != nil {
			if k8serrors.IsNotFound(err) {
				continue
			}
			return nil, err
		}

		if !isCertificateReady(&cert) {
			continue
		}

		certManagerCerts = append(certManagerCerts, cert)
	}

	return certManagerCerts, nil
}

func removeOldCertificates(ctx context.Context, c client.Client, instance, instanceMergedWithFlavors *v1alpha1.RpaasInstance) error {
	certs, err := getCertificates(ctx, c, instanceMergedWithFlavors)
	if err != nil {
		return err
	}

	toRemove := make(map[string]bool)
	for _, cert := range certs {
		toRemove[cert.Name] = true
	}

	for _, req := range instanceMergedWithFlavors.Spec.CertManagerRequests(instanceMergedWithFlavors.Name) {
		delete(toRemove, CertManagerCertificateNameForInstance(instance.Name, req))
	}

	for name := range toRemove {
		var cert cmv1.Certificate
		err = c.Get(ctx, types.NamespacedName{Name: name, Namespace: instance.Namespace}, &cert)
		if err != nil {
			return err
		}

		if err = c.Delete(ctx, &cert); err != nil {
			return err
		}

		var secret corev1.Secret
		err = c.Get(ctx, types.NamespacedName{Name: cert.Spec.SecretName, Namespace: instance.Namespace}, &secret)
		if err != nil && !k8serrors.IsNotFound(err) {
			return err
		} else if err == nil {
			if err = c.Delete(ctx, &secret); err != nil {
				return err
			}
		}
	}

	return nil
}

func takeOverPreviousSecret(ctx context.Context, c client.Client, instanceMergedWithFlavors *v1alpha1.RpaasInstance, cert *cmv1.Certificate) error {
	var secret corev1.Secret

	err := c.Get(ctx, types.NamespacedName{Name: cert.Spec.SecretName, Namespace: cert.Namespace}, &secret)

	if err == nil {
		return nil
	}

	if !k8serrors.IsNotFound(err) {
		return err
	}

	// find previous secret
	certificateName := cert.Labels[CertificateNameLabel]
	previousSecret, err := getTLSSecretByCertificateName(ctx, c, instanceMergedWithFlavors, certificateName)

	if err == ErrTLSSecretNotFound {
		return nil
	}

	if err != nil {
		return err
	}

	secret = corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:        cert.Spec.SecretName,
			Namespace:   cert.Namespace,
			Labels:      cert.Labels,
			Annotations: cert.Annotations,
		},
		Data:       previousSecret.Data,
		StringData: previousSecret.StringData,
		Type:       previousSecret.Type,
	}

	return c.Create(ctx, &secret)
}

func getCertificates(ctx context.Context, c client.Client, i *v1alpha1.RpaasInstance) ([]cmv1.Certificate, error) {
	var certList cmv1.CertificateList
	err := c.List(ctx, &certList, &client.ListOptions{
		LabelSelector: labels.SelectorFromSet(map[string]string{CertificateNameLabel: i.Name}),
		Namespace:     i.Namespace},
	)
	if err != nil {
		return nil, err
	}

	var certs []cmv1.Certificate
	certs = append(certs, certList.Items...)

	err = c.List(ctx, &certList, &client.ListOptions{Namespace: i.Namespace})
	if err != nil {
		return nil, err
	}

	for _, cert := range certList.Items {
		for _, ownerRef := range cert.OwnerReferences {
			if ownerRef.APIVersion == v1alpha1.GroupVersion.String() &&
				ownerRef.Kind == "RpaasInstance" &&
				ownerRef.Name == i.Name {
				certs = append(certs, cert)
			}
		}
	}

	return certs, nil
}

func newCertificate(instance *v1alpha1.RpaasInstance, issuer *cmmeta.ObjectReference, req v1alpha1.CertManager) (*cmv1.Certificate, error) {
	labels := map[string]string{}

	for k, v := range instance.Labels {
		labels[k] = v
	}

	labels["rpaas.extensions.tsuru.io/certificate-name"] = req.RequiredName()
	labels["rpaas.extensions.tsuru.io/instance-name"] = instance.Name

	var commonName string
	if len(req.DNSNames) > 0 {
		commonName = req.DNSNames[0]
	}

	var revisionHistoryLimit *int32
	var subject *cmv1.X509Subject
	var privateKey *cmv1.CertificatePrivateKey
	if instance.Spec.CertificateSpec != nil && !req.IgnoreCertificateSpec {
		subject = instance.Spec.CertificateSpec.Subject
		privateKey = instance.Spec.CertificateSpec.PrivateKey
		revisionHistoryLimit = instance.Spec.CertificateSpec.RevisionHistoryLimit
	}

	return &cmv1.Certificate{
		ObjectMeta: metav1.ObjectMeta{
			Name:      CertManagerCertificateNameForInstance(instance.Name, req),
			Namespace: instance.Namespace,
			Labels:    labels,
			OwnerReferences: []metav1.OwnerReference{
				*metav1.NewControllerRef(instance, schema.GroupVersionKind{
					Group:   v1alpha1.GroupVersion.Group,
					Version: v1alpha1.GroupVersion.Version,
					Kind:    "RpaasInstance",
				}),
			},
		},
		Spec: cmv1.CertificateSpec{
			IssuerRef:            *issuer,
			DNSNames:             req.DNSNames,
			IPAddresses:          req.IPAddresses,
			CommonName:           commonName,
			Subject:              subject,
			PrivateKey:           privateKey,
			RevisionHistoryLimit: revisionHistoryLimit,
			SecretName:           CertManagerCertificateNameForInstance(instance.Name, req),
			SecretTemplate: &cmv1.CertificateSecretTemplate{
				Labels: map[string]string{
					"rpaas.extensions.tsuru.io/certificate-name": req.RequiredName(),
					"rpaas.extensions.tsuru.io/instance-name":    instance.Name,
				},
			},
		},
	}, nil
}

func getCertManagerIssuer(ctx context.Context, client client.Client, req v1alpha1.CertManager, namespace string) (*cmmeta.ObjectReference, error) {
	if strings.Contains(req.Issuer, ".") {
		return getCustomCertManagerIssuer(ctx, client, req, namespace)
	}

	var issuer cmv1.Issuer
	err := client.Get(ctx, types.NamespacedName{
		Name:      req.Issuer,
		Namespace: namespace,
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
		Name: req.Issuer,
	}, &clusterIssuer)

	if k8serrors.IsNotFound(err) {
		return nil, fmt.Errorf("there is no %q certificate issuer", req.Issuer)
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

func getCustomCertManagerIssuer(ctx context.Context, client client.Client, req v1alpha1.CertManager, namespace string) (*cmmeta.ObjectReference, error) {
	parts := strings.SplitN(req.Issuer, ".", 3)
	if len(parts) < 3 {
		return nil, fmt.Errorf("missing information to retrieve custom Cert Manager issuer: (requires <resource name>.<resource kind>.<resource group>, got %s)", req.Issuer)
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

	err = client.Get(ctx, types.NamespacedName{Name: name, Namespace: namespace}, u)
	if err != nil {
		return nil, err
	}

	return &cmmeta.ObjectReference{
		Group: mapping.GroupVersionKind.Group,
		Kind:  mapping.GroupVersionKind.Kind,
		Name:  name,
	}, nil
}

func isCertificateReady(cert *cmv1.Certificate) bool {
	if cert == nil {
		return false
	}

	for _, c := range cert.Status.Conditions {
		if c.Type == cmv1.CertificateConditionReady && c.Status == cmmeta.ConditionTrue {
			return true
		}
	}

	return false
}

func CertManagerCertificateNameForInstance(instanceName string, r v1alpha1.CertManager) string {
	return fmt.Sprintf("%s-%s", instanceName, r.RequiredName())
}
