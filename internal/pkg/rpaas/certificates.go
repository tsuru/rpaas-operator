// Copyright 2021 tsuru authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package rpaas

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	cmv1 "github.com/cert-manager/cert-manager/pkg/apis/certmanager/v1"
	k8sErrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"

	"github.com/tsuru/rpaas-operator/api/v1alpha1"
	"github.com/tsuru/rpaas-operator/internal/config"
	clientTypes "github.com/tsuru/rpaas-operator/pkg/rpaas/client/types"
)

func (m *k8sRpaasManager) GetCertManagerRequests(ctx context.Context, instanceName string) ([]clientTypes.CertManager, error) {
	instance, err := m.GetInstance(ctx, instanceName)
	if err != nil {
		return nil, err
	}

	var requests []clientTypes.CertManager
	for _, r := range instance.CertManagerRequests() {
		requests = append(requests, clientTypes.CertManager{
			Name:        r.Name,
			Issuer:      r.Issuer,
			DNSNames:    r.DNSNames,
			IPAddresses: r.IPAddresses,
		})
	}

	return requests, nil
}

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

	issuerAnnotations, err := m.getIssuerMetadata(ctx, instance.Namespace, issuer)
	if err != nil {
		return err
	}

	allowed := strings.Split(issuerAnnotations[allowedDNSZonesAnnotation], ",")
	if err = areDNSNamesAllowed(allowed, in.DNSNames); err != nil {
		return err
	}

	if issuerAnnotations[maxDNSNamesAnnotation] != "" {
		maxDNSName, _ := strconv.Atoi(issuerAnnotations[maxDNSNamesAnnotation])
		if len(in.DNSNames) > maxDNSName {
			return &ValidationError{Msg: fmt.Sprintf("maximum number of DNS names exceeded (maximum allowed: %d)", maxDNSName)}
		}
	}

	if issuerAnnotations[maxIPsAnnotation] != "" {
		maxIPs, _ := strconv.Atoi(issuerAnnotations[maxIPsAnnotation])
		if len(in.IPAddresses) > maxIPs {
			return &ValidationError{Msg: fmt.Sprintf("maximum number of IP Addresses exceeded (maximum allowed: %d)", maxIPs)}
		}
	}

	if issuerAnnotations[strictNamesAnnotation] == "true" && len(in.DNSNames) > 0 {
		expectedName := strings.TrimPrefix(in.DNSNames[0], "*.")

		if expectedName != in.Name {
			return &ValidationError{Msg: fmt.Sprintf("the name of this certificate must be: %q", expectedName)}
		}
	}

	if issuerAnnotations[allowWildcardAnnotation] == "false" {
		for _, dnsName := range in.DNSNames {
			if strings.HasPrefix(dnsName, "*") {
				return &ValidationError{Msg: "wildcard DNS names are not allowed on this issuer"}
			}
		}
	}

	newRequest := v1alpha1.CertManager{
		Name:        in.Name,
		Issuer:      issuer,
		DNSNames:    in.DNSNames,
		IPAddresses: in.IPAddresses,
	}

	if req := instance.Spec.DynamicCertificates.CertManager; req != nil && req.Issuer == issuer {
		instance.Spec.DynamicCertificates.CertManager = nil
	}

	if index, found := findCertManagerRequestByName(instance, in.Name); found && in.Name != "" {
		instance.Spec.DynamicCertificates.CertManagerRequests[index] = newRequest
	} else if index, found := findCertManagerRequestByIssuer(instance, in.Issuer); found && in.Name == "" {
		instance.Spec.DynamicCertificates.CertManagerRequests[index] = newRequest
	} else {
		instance.Spec.DynamicCertificates.CertManagerRequests = append(instance.Spec.DynamicCertificates.CertManagerRequests, newRequest)
	}

	return m.cli.Update(ctx, instance)
}

func (m *k8sRpaasManager) DeleteCertManagerRequestByIssuer(ctx context.Context, instanceName, issuer string) error {
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

func (m *k8sRpaasManager) DeleteCertManagerRequestByName(ctx context.Context, instanceName, name string) error {
	instance, err := m.GetInstance(ctx, instanceName)
	if err != nil {
		return err
	}

	if name == "" {
		return &ValidationError{Msg: "cert-manager name cannot be empty"}
	}

	if instance.Spec.DynamicCertificates == nil {
		instance.Spec.DynamicCertificates = &v1alpha1.DynamicCertificates{}
	}

	if req := instance.Spec.DynamicCertificates.CertManager; req != nil {
		if req.Name == name {
			instance.Spec.DynamicCertificates.CertManager = nil
			return m.cli.Update(ctx, instance)
		}
	}

	index, found := findCertManagerRequestByName(instance, name)
	if !found {
		return &NotFoundError{Msg: "cert-manager certificate has already been removed"}
	}

	// NOTE: removes the index-th element of slice.
	instance.Spec.DynamicCertificates.CertManagerRequests = append(instance.Spec.DynamicCertificates.CertManagerRequests[:index], instance.Spec.DynamicCertificates.CertManagerRequests[index+1:]...)

	return m.cli.Update(ctx, instance)
}

func (m *k8sRpaasManager) getIssuerMetadata(ctx context.Context, namespace, issuerName string) (map[string]string, error) {
	if strings.Contains(issuerName, ".") {
		return m.getCustomIssuerMetadata(ctx, namespace, issuerName)
	}

	var issuer cmv1.Issuer
	err := m.cli.Get(ctx, types.NamespacedName{
		Name:      issuerName,
		Namespace: namespace,
	}, &issuer)

	if err != nil && !k8sErrors.IsNotFound(err) {
		return nil, err
	}

	if err == nil {
		return issuer.Annotations, nil
	}

	var clusterIssuer cmv1.ClusterIssuer
	err = m.cli.Get(ctx, types.NamespacedName{
		Name: issuerName,
	}, &clusterIssuer)

	if err != nil && k8sErrors.IsNotFound(err) {
		return nil, fmt.Errorf("there is no Issuer or ClusterIssuer with %q name", issuerName)
	}

	return clusterIssuer.Annotations, nil
}

func (m *k8sRpaasManager) getCustomIssuerMetadata(ctx context.Context, namespace, issuer string) (map[string]string, error) {
	parts := strings.SplitN(issuer, ".", 3)
	if len(parts) < 3 {
		return nil, fmt.Errorf("missing information to retrieve custom Cert Manager issuer: (requires <resource name>.<resource kind>.<resource group>, got %s)", issuer)
	}

	name, kind, group := parts[0], parts[1], parts[2]

	restMapper := m.cli.RESTMapper()
	if restMapper == nil {
		return map[string]string{}, nil
	}
	mapping, err := restMapper.RESTMapping(schema.GroupKind{Group: group, Kind: kind})
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

	err = m.cli.Get(ctx, types.NamespacedName{Name: name, Namespace: namespace}, u)
	if err != nil {
		return nil, err
	}

	return u.GetAnnotations(), nil
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

func findCertManagerRequestByName(instance *v1alpha1.RpaasInstance, name string) (int, bool) {
	if instance.Spec.DynamicCertificates == nil {
		return -1, false
	}

	if name == "" {
		return -1, false
	}

	for i, req := range instance.Spec.DynamicCertificates.CertManagerRequests {
		if req.Name == name {
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
