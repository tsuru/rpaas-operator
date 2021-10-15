// Copyright 2021 tsuru authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package rpaas

import (
	"context"
	"fmt"
	"strings"

	cmv1 "github.com/jetstack/cert-manager/pkg/apis/certmanager/v1"
	"github.com/tsuru/rpaas-operator/api/v1alpha1"
	"github.com/tsuru/rpaas-operator/internal/config"
	clientTypes "github.com/tsuru/rpaas-operator/pkg/rpaas/client/types"
	k8sErrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

func (m *k8sRpaasManager) UpdateCertManagerRequest(ctx context.Context, instanceName string, in clientTypes.CertManager) error {
	if !config.Get().EnableCertManager {
		return &ConflictError{Msg: "Cert Manager integration not enabled"}
	}

	instance, err := m.GetInstance(ctx, instanceName)
	if err != nil {
		return err
	}

	if instance.Spec.DynamicCertificates == nil {
		instance.Spec.DynamicCertificates = &v1alpha1.DynamicCertificates{}
	}

	issuer := issuerOrDefault(in.Issuer)
	if issuer == "" {
		return &ValidationError{Msg: "Cert Manager issuer cannot be empty"}
	}

	if len(in.DNSNames) == 0 && len(in.IPAddresses) == 0 {
		return &ValidationError{Msg: "you should provide a list of DNS names or IP addresses"}
	}

	issuerMeta, _, err := m.getIssuerMetadata(ctx, instance.Namespace, issuer)
	if err != nil {
		return err
	}

	allowed := strings.Split(issuerMeta.Annotations[allowedDNSZonesAnnotation], ",")
	if err = areDNSNamesAllowed(allowed, in.DNSNames); err != nil {
		return err
	}

	newRequest := v1alpha1.CertManager{
		Issuer:      issuer,
		DNSNames:    in.DNSNames,
		IPAddresses: in.IPAddresses,
	}

	if req := instance.Spec.DynamicCertificates.CertManager; req != nil && req.Issuer == issuer {
		instance.Spec.DynamicCertificates.CertManager = nil
	}

	if index, found := findCertManagerRequestByIssuer(instance, in.Issuer); found {
		instance.Spec.DynamicCertificates.CertManagerRequests[index] = newRequest
	} else {
		instance.Spec.DynamicCertificates.CertManagerRequests = append(instance.Spec.DynamicCertificates.CertManagerRequests, newRequest)
	}

	return m.cli.Update(ctx, instance)
}

func (m *k8sRpaasManager) DeleteCertManagerRequest(ctx context.Context, instanceName, issuer string) error {
	instance, err := m.GetInstance(ctx, instanceName)
	if err != nil {
		return err
	}

	issuer = issuerOrDefault(issuer)
	if issuer == "" {
		return &ValidationError{Msg: "cert-manager issuer cannot be empty"}
	}

	if instance.Spec.DynamicCertificates == nil {
		instance.Spec.DynamicCertificates = &v1alpha1.DynamicCertificates{}
	}

	if req := instance.Spec.DynamicCertificates.CertManager; req != nil {
		if req.Issuer == issuer {
			instance.Spec.DynamicCertificates.CertManager = nil
			return m.cli.Update(ctx, instance)
		}
	}

	index, found := findCertManagerRequestByIssuer(instance, issuer)
	if !found {
		return &NotFoundError{Msg: "cert-manager certificate has already been removed"}
	}

	// NOTE: removes the index-th element of slice.
	instance.Spec.DynamicCertificates.CertManagerRequests = append(instance.Spec.DynamicCertificates.CertManagerRequests[:index], instance.Spec.DynamicCertificates.CertManagerRequests[index+1:]...)

	return m.cli.Update(ctx, instance)
}

func (m *k8sRpaasManager) getIssuerMetadata(ctx context.Context, namespace, issuerName string) (*metav1.ObjectMeta, *cmv1.IssuerSpec, error) {
	var issuer cmv1.Issuer

	err := m.cli.Get(ctx, types.NamespacedName{
		Name:      issuerName,
		Namespace: namespace,
	}, &issuer)

	if err != nil && !k8sErrors.IsNotFound(err) {
		return nil, nil, err
	}

	if err == nil {
		return &issuer.ObjectMeta, &issuer.Spec, nil
	}

	var clusterIssuer cmv1.ClusterIssuer

	err = m.cli.Get(ctx, types.NamespacedName{
		Name: issuerName,
	}, &clusterIssuer)

	if err != nil && k8sErrors.IsNotFound(err) {
		return nil, nil, fmt.Errorf("there is no Issuer or ClusterIssuer with %q name", issuerName)
	}

	return &clusterIssuer.ObjectMeta, &clusterIssuer.Spec, nil
}

func findCertManagerRequestByIssuer(instance *v1alpha1.RpaasInstance, issuer string) (int, bool) {
	if instance.Spec.DynamicCertificates == nil {
		return -1, false
	}

	for i, req := range instance.Spec.DynamicCertificates.CertManagerRequests {
		if req.Issuer == issuer {
			return i, true
		}
	}

	return -1, false
}

func areDNSNamesAllowed(allowedSuffixes, dnsNames []string) error {
	var unmatched []string
	for _, want := range dnsNames {
		var found bool
		for _, suffix := range allowedSuffixes {
			if strings.HasSuffix(want, suffix) {
				found = true
			}
		}

		if !found {
			unmatched = append(unmatched, want)
		}
	}

	if len(unmatched) > 0 {
		return &ValidationError{Msg: fmt.Sprintf("there is some DNS name with forbidden suffix (invalid ones: %s - allowed DNS suffixes: %s)", strings.Join(unmatched, ", "), strings.Join(allowedSuffixes, ", "))}
	}

	return nil
}

func issuerOrDefault(issuer string) string {
	if issuer != "" {
		return issuer
	}

	return config.Get().DefaultCertManagerIssuer
}
