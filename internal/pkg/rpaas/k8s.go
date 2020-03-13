// Copyright 2019 tsuru authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package rpaas

import (
	"bytes"
	"context"
	"crypto"
	"crypto/ecdsa"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"reflect"
	"regexp"
	"sort"
	"strings"

	"github.com/pkg/errors"
	nginxv1alpha1 "github.com/tsuru/nginx-operator/pkg/apis/nginx/v1alpha1"
	"github.com/tsuru/rpaas-operator/config"
	nginxManager "github.com/tsuru/rpaas-operator/internal/pkg/rpaas/nginx"
	"github.com/tsuru/rpaas-operator/pkg/apis/extensions/v1alpha1"
	clientTypes "github.com/tsuru/rpaas-operator/pkg/rpaas/client/types"
	"github.com/tsuru/rpaas-operator/pkg/util"
	corev1 "k8s.io/api/core/v1"
	k8sErrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/manager"
)

const (
	defaultNamespace      = "rpaasv2"
	defaultKeyLabelPrefix = "rpaas.extensions.tsuru.io"
)

var _ RpaasManager = &k8sRpaasManager{}

type k8sRpaasManager struct {
	nonCachedCli client.Client
	cli          client.Client
	cacheManager CacheManager
}

func NewK8S(mgr manager.Manager) (RpaasManager, error) {
	nonCachedCli, err := client.New(mgr.GetConfig(), client.Options{
		Scheme: mgr.GetScheme(),
		Mapper: mgr.GetRESTMapper(),
	})
	if err != nil {
		return nil, err
	}
	return &k8sRpaasManager{
		nonCachedCli: nonCachedCli,
		cli:          mgr.GetClient(),
		cacheManager: nginxManager.NewNginxManager(),
	}, nil
}

func (m *k8sRpaasManager) DeleteInstance(ctx context.Context, name string) error {
	instance, err := m.GetInstance(ctx, name)
	if err != nil {
		return err
	}
	return m.cli.Delete(ctx, instance)
}

func (m *k8sRpaasManager) CreateInstance(ctx context.Context, args CreateArgs) error {
	if err := m.validateCreate(ctx, args); err != nil {
		return err
	}

	nsName, err := m.ensureNamespaceExists(ctx)
	if err != nil {
		return err
	}

	plan, err := m.getPlan(ctx, args.Plan)
	if err != nil {
		return err
	}

	instance := newRpaasInstance(args.Name)
	instance.Namespace = nsName
	instance.Spec.PlanName = plan.Name
	instance.Spec.Replicas = func(n int32) *int32 { return &n }(int32(1)) // one replica
	instance.Spec.Service = &nginxv1alpha1.NginxService{
		Type:        corev1.ServiceTypeLoadBalancer,
		Annotations: instance.Annotations,
		Labels:      instance.Labels,
	}
	instance.Spec.PodTemplate = nginxv1alpha1.NginxPodTemplateSpec{
		Affinity:    getAffinity(args.Team),
		Annotations: instance.Annotations,
		Labels:      instance.Labels,
	}

	setDescription(instance, args.Description)
	setTeamOwner(instance, args.Team)

	if err := m.setTags(ctx, instance, args.Tags); err != nil {
		return err
	}

	return m.cli.Create(ctx, instance)
}

func (m *k8sRpaasManager) UpdateInstance(ctx context.Context, instanceName string, args UpdateInstanceArgs) error {
	instance, err := m.GetInstance(ctx, instanceName)
	if err != nil {
		return err
	}

	plan, err := m.getPlan(ctx, args.Plan)
	if err != nil {
		return err
	}

	instance.Spec.PlanName = plan.Name
	setDescription(instance, args.Description)
	setTeamOwner(instance, args.Team)

	if err = m.setTags(ctx, instance, args.Tags); err != nil {
		return err
	}

	return m.cli.Update(ctx, instance)
}

func (m *k8sRpaasManager) ensureNamespaceExists(ctx context.Context) (string, error) {
	nsName := getServiceName()
	ns := newNamespace(nsName)
	if err := m.cli.Create(ctx, &ns); err != nil && !k8sErrors.IsAlreadyExists(err) {
		return "", err
	}

	return nsName, nil
}

func (m *k8sRpaasManager) GetAutoscale(ctx context.Context, instanceName string) (*Autoscale, error) {
	instance, err := m.GetInstance(ctx, instanceName)
	if err != nil {
		return nil, err
	}

	autoscale := instance.Spec.Autoscale
	if autoscale == nil {
		return nil, NotFoundError{Msg: fmt.Sprintf("autoscale not found")}
	}

	s := Autoscale{
		MinReplicas: autoscale.MinReplicas,
		MaxReplicas: &autoscale.MaxReplicas,
		CPU:         autoscale.TargetCPUUtilizationPercentage,
		Memory:      autoscale.TargetMemoryUtilizationPercentage,
	}

	return &s, nil
}

func (m *k8sRpaasManager) CreateAutoscale(ctx context.Context, instanceName string, autoscale *Autoscale) error {
	instance, err := m.GetInstance(ctx, instanceName)
	if err != nil {
		return err
	}

	err = validateAutoscale(ctx, autoscale)
	if err != nil {
		return err
	}

	s := instance.Spec.Autoscale
	if s != nil {
		return ValidationError{Msg: fmt.Sprintf("Autoscale already created")}
	}

	instance.Spec.Autoscale = &v1alpha1.RpaasInstanceAutoscaleSpec{
		MinReplicas:                       autoscale.MinReplicas,
		MaxReplicas:                       *autoscale.MaxReplicas,
		TargetCPUUtilizationPercentage:    autoscale.CPU,
		TargetMemoryUtilizationPercentage: autoscale.Memory,
	}

	return m.cli.Update(ctx, instance)
}

func (m *k8sRpaasManager) UpdateAutoscale(ctx context.Context, instanceName string, autoscale *Autoscale) error {
	instance, err := m.GetInstance(ctx, instanceName)
	if err != nil {
		return err
	}

	s := instance.Spec.Autoscale
	if s == nil {
		// Create if empty
		instance.Spec.Autoscale = &v1alpha1.RpaasInstanceAutoscaleSpec{}
		s = instance.Spec.Autoscale
	}

	if s.MinReplicas != autoscale.MinReplicas {
		s.MinReplicas = autoscale.MinReplicas
	}

	if &s.MaxReplicas != autoscale.MaxReplicas {
		s.MaxReplicas = *autoscale.MaxReplicas
	}

	if s.TargetCPUUtilizationPercentage != autoscale.CPU {
		s.TargetCPUUtilizationPercentage = autoscale.CPU
	}

	if s.TargetMemoryUtilizationPercentage != autoscale.Memory {
		s.TargetMemoryUtilizationPercentage = autoscale.Memory
	}

	err = validateAutoscaleSpec(ctx, s)
	if err != nil {
		return err
	}

	return m.cli.Update(ctx, instance)
}

func (m *k8sRpaasManager) DeleteAutoscale(ctx context.Context, instanceName string) error {
	instance, err := m.GetInstance(ctx, instanceName)
	if err != nil {
		return err
	}

	instance.Spec.Autoscale = nil

	return m.cli.Update(ctx, instance)
}

func validateAutoscale(ctx context.Context, s *Autoscale) error {
	if *s.MaxReplicas == 0 {
		return ValidationError{Msg: "max replicas is required"}
	}

	return nil
}

func validateAutoscaleSpec(ctx context.Context, s *v1alpha1.RpaasInstanceAutoscaleSpec) error {
	if s.MaxReplicas == 0 {
		return ValidationError{Msg: "max replicas is required"}
	}

	return nil
}

func (m *k8sRpaasManager) DeleteBlock(ctx context.Context, instanceName, blockName string) error {
	instance, err := m.GetInstance(ctx, instanceName)
	if err != nil {
		return err
	}

	if instance.Spec.Blocks == nil {
		return NotFoundError{Msg: fmt.Sprintf("block %q not found", blockName)}
	}

	blockType := v1alpha1.BlockType(blockName)
	if _, ok := instance.Spec.Blocks[blockType]; !ok {
		return NotFoundError{Msg: fmt.Sprintf("block %q not found", blockName)}
	}

	delete(instance.Spec.Blocks, blockType)
	return m.cli.Update(ctx, instance)
}

func (m *k8sRpaasManager) ListBlocks(ctx context.Context, instanceName string) ([]ConfigurationBlock, error) {
	instance, err := m.GetInstance(ctx, instanceName)
	if err != nil {
		return nil, err
	}

	var blocks []ConfigurationBlock
	for blockType, blockValue := range instance.Spec.Blocks {
		content, err := util.GetValue(ctx, m.cli, instance.Namespace, &blockValue)
		if err != nil {
			return nil, err
		}

		blocks = append(blocks, ConfigurationBlock{Name: string(blockType), Content: content})
	}

	sort.SliceStable(blocks, func(i, j int) bool {
		return blocks[i].Name < blocks[j].Name
	})

	return blocks, nil
}

func (m *k8sRpaasManager) UpdateBlock(ctx context.Context, instanceName string, block ConfigurationBlock) error {
	instance, err := m.GetInstance(ctx, instanceName)
	if err != nil {
		return err
	}

	blockType := v1alpha1.BlockType(block.Name)
	if !isBlockTypeAllowed(blockType) {
		return ValidationError{Msg: fmt.Sprintf("block %q is not allowed", block.Name)}
	}

	if instance.Spec.Blocks == nil {
		instance.Spec.Blocks = make(map[v1alpha1.BlockType]v1alpha1.Value)
	}

	instance.Spec.Blocks[blockType] = v1alpha1.Value{Value: block.Content}

	return m.cli.Update(ctx, instance)
}

func (m *k8sRpaasManager) Scale(ctx context.Context, instanceName string, replicas int32) error {
	instance, err := m.GetInstance(ctx, instanceName)
	if err != nil {
		return err
	}
	if replicas < 0 {
		return ValidationError{Msg: fmt.Sprintf("invalid replicas number: %d", replicas)}
	}
	instance.Spec.Replicas = &replicas
	return m.cli.Update(ctx, instance)
}

func (m *k8sRpaasManager) GetCertificates(ctx context.Context, instanceName string) ([]CertificateData, error) {
	instance, err := m.GetInstance(ctx, instanceName)
	if err != nil {
		return nil, err
	}

	if instance.Spec.Certificates == nil {
		return nil, nil
	}

	var secret corev1.Secret
	err = m.cli.Get(ctx, types.NamespacedName{
		Name:      instance.Spec.Certificates.SecretName,
		Namespace: instance.Namespace,
	}, &secret)
	if err != nil {
		return nil, err
	}

	var certList []CertificateData
	for _, item := range instance.Spec.Certificates.Items {
		if _, ok := secret.Data[item.CertificateField]; !ok {
			return nil, fmt.Errorf("certificate data not found")
		}
		if _, ok := secret.Data[item.KeyField]; !ok {
			return nil, fmt.Errorf("key data not found")
		}
		certItem := CertificateData{
			Name:        strings.TrimSuffix(item.CertificateField, ".crt"),
			Certificate: string(secret.Data[item.CertificateField]),
			Key:         string(secret.Data[item.KeyField]),
		}

		certList = append(certList, certItem)
	}

	return certList, nil
}

func searchCertificate(certificates []nginxv1alpha1.TLSSecretItem, target string) (int, bool) {
	for i, c := range certificates {
		if strings.TrimSuffix(c.CertificateField, ".crt") == target {
			return i, true
		}
	}

	return -1, false
}

func (m *k8sRpaasManager) DeleteCertificate(ctx context.Context, instanceName, name string) error {
	instance, err := m.GetInstance(ctx, instanceName)
	if err != nil {
		return err
	}
	if name == "" {
		name = v1alpha1.CertificateNameDefault
	}

	if instance.Spec.Certificates == nil {
		return &NotFoundError{Msg: fmt.Sprintf("no certificate bound to instance %q", instanceName)}
	}

	var oldSecret corev1.Secret
	err = m.cli.Get(ctx, types.NamespacedName{
		Name:      instance.Spec.Certificates.SecretName,
		Namespace: instance.Namespace,
	}, &oldSecret)
	if err != nil {
		return err
	}

	if index, found := searchCertificate(instance.Spec.Certificates.Items, name); found {
		items := instance.Spec.Certificates.Items
		item := items[index]
		instance.Spec.Certificates.Items = append(items[:index], items[index+1:]...)

		// deleting secret data
		delete(oldSecret.Data, item.CertificateField)
		delete(oldSecret.Data, item.KeyField)
		err = m.cli.Update(ctx, &oldSecret)
		if err != nil {
			return err
		}

	} else {
		return &NotFoundError{Msg: fmt.Sprintf("certificate not found")}
	}

	if len(oldSecret.Data) == 0 {
		err = m.cli.Delete(ctx, &oldSecret)
		if err != nil {
			return err
		}
	}

	if len(instance.Spec.Certificates.Items) == 0 {
		instance.Spec.Certificates = nil
	}

	return m.cli.Update(ctx, instance)
}

func (m *k8sRpaasManager) UpdateCertificate(ctx context.Context, instanceName, name string, c tls.Certificate) error {
	instance, err := m.GetInstance(ctx, instanceName)
	if err != nil {
		return err
	}

	if name == "" {
		name = v1alpha1.CertificateNameDefault
	}

	var oldSecret corev1.Secret
	if instance.Spec.Certificates != nil && instance.Spec.Certificates.SecretName != "" {
		err = m.cli.Get(ctx, types.NamespacedName{
			Name:      instance.Spec.Certificates.SecretName,
			Namespace: instance.Namespace,
		}, &oldSecret)

		if err != nil {
			return err
		}
	}

	newSecretData := map[string][]byte{}
	// copying the whole old secret's data to newSecretData to safely compare them after
	for key, value := range oldSecret.Data {
		newSecretData[key] = value
	}

	rawCertificate, rawKey, err := getRawCertificateAndKey(c)
	if err != nil {
		return err
	}

	newCertificateField := fmt.Sprintf("%s.crt", name)
	newKeyField := fmt.Sprintf("%s.key", name)

	newSecretData[newCertificateField] = rawCertificate
	newSecretData[newKeyField] = rawKey

	if reflect.DeepEqual(newSecretData, oldSecret.Data) {
		return &ConflictError{Msg: fmt.Sprintf("certificate %q already is deployed", name)}
	}

	newSecret := newSecretForCertificates(*instance, newSecretData)
	if err = m.cli.Create(ctx, newSecret); err != nil {
		return err
	}

	if instance.Spec.Certificates == nil {
		instance.Spec.Certificates = &nginxv1alpha1.TLSSecret{}
	}

	instance.Spec.Certificates.SecretName = newSecret.Name

	isNewCertificate := true
	for _, item := range instance.Spec.Certificates.Items {
		if item.CertificateField == newCertificateField && item.KeyField == newKeyField {
			isNewCertificate = false
			break
		}
	}

	if isNewCertificate {
		instance.Spec.Certificates.Items = append(instance.Spec.Certificates.Items, nginxv1alpha1.TLSSecretItem{
			CertificateField: newCertificateField,
			KeyField:         newKeyField,
		})
	}

	return m.cli.Update(ctx, instance)
}

func (m *k8sRpaasManager) GetInstanceAddress(ctx context.Context, name string) (string, error) {
	instance, err := m.GetInstance(ctx, name)
	if err != nil {
		return "", err
	}

	var nginx nginxv1alpha1.Nginx
	err = m.cli.Get(ctx, types.NamespacedName{Name: instance.Name, Namespace: instance.Namespace}, &nginx)
	if err != nil && IsNotFoundError(err) {
		return "", nil
	}

	if err != nil {
		return "", err
	}

	if len(nginx.Status.Services) == 0 {
		return "", nil
	}

	svcName := nginx.Status.Services[0].Name
	var svc corev1.Service
	err = m.cli.Get(ctx, types.NamespacedName{Name: svcName, Namespace: instance.Namespace}, &svc)
	if err != nil {
		return "", err
	}

	switch svc.Spec.Type {
	case corev1.ServiceTypeLoadBalancer:
		if len(svc.Status.LoadBalancer.Ingress) == 0 {
			return "", nil
		}

		return svc.Status.LoadBalancer.Ingress[0].IP, nil
	case corev1.ServiceTypeClusterIP, corev1.ServiceTypeNodePort:
		return svc.Spec.ClusterIP, nil
	}

	return "", nil
}

func (m *k8sRpaasManager) GetInstance(ctx context.Context, name string) (*v1alpha1.RpaasInstance, error) {
	var instance v1alpha1.RpaasInstance
	err := m.cli.Get(ctx, types.NamespacedName{Name: name, Namespace: namespaceName()}, &instance)
	if err != nil && k8sErrors.IsNotFound(err) {
		return nil, NotFoundError{Msg: fmt.Sprintf("rpaas instance %q not found", name)}
	}

	if err != nil {
		return nil, err
	}

	return &instance, nil
}

func (m *k8sRpaasManager) GetPlans(ctx context.Context) ([]v1alpha1.RpaasPlan, error) {
	var planList v1alpha1.RpaasPlanList
	if err := m.cli.List(ctx, &planList, client.InNamespace(namespaceName())); err != nil {
		return nil, err
	}

	return planList.Items, nil
}

func (m *k8sRpaasManager) GetFlavors(ctx context.Context) ([]Flavor, error) {
	flavors, err := m.getFlavors(ctx)
	if err != nil {
		return nil, err
	}

	var result []Flavor
	for _, flavor := range flavors {
		if flavor.Spec.Default {
			continue
		}
		result = append(result, Flavor{
			Name:        flavor.Name,
			Description: flavor.Spec.Description,
		})
	}

	sort.SliceStable(result, func(i, j int) bool { return result[i].Name < result[j].Name })

	return result, nil
}

func (m *k8sRpaasManager) getFlavors(ctx context.Context) ([]v1alpha1.RpaasFlavor, error) {
	flavorList := &v1alpha1.RpaasFlavorList{}
	if err := m.cli.List(ctx, flavorList, client.InNamespace(namespaceName())); err != nil {
		return nil, err
	}

	return flavorList.Items, nil
}

func (m *k8sRpaasManager) isFlavorAvailable(ctx context.Context, flavorName string) bool {
	flavors, err := m.getFlavors(ctx)
	if err != nil {
		return false
	}

	for _, flavor := range flavors {
		if flavor.Name == flavorName {
			return true
		}
	}

	return false
}

func (m *k8sRpaasManager) CreateExtraFiles(ctx context.Context, instanceName string, files ...File) error {
	instance, err := m.GetInstance(ctx, instanceName)
	if err != nil {
		return err
	}
	newData := map[string][]byte{}
	oldExtraFiles, err := m.getExtraFiles(ctx, *instance)
	if err != nil && !IsNotFoundError(err) {
		return err
	}
	if oldExtraFiles != nil && oldExtraFiles.BinaryData != nil {
		newData = oldExtraFiles.BinaryData
	}
	for _, file := range files {
		if !isPathValid(file.Name) {
			return &ValidationError{Msg: fmt.Sprintf("filename %q is not valid", file.Name)}
		}
		key := convertPathToConfigMapKey(file.Name)
		if _, ok := newData[key]; ok {
			return &ConflictError{Msg: fmt.Sprintf("file %q already exists", file.Name)}
		}
		newData[key] = file.Content
	}
	newExtraFiles, err := m.createExtraFiles(ctx, *instance, newData)
	if err != nil {
		return err
	}
	if instance.Spec.ExtraFiles == nil {
		instance.Spec.ExtraFiles = &nginxv1alpha1.FilesRef{
			Files: map[string]string{},
		}
	}
	for _, file := range files {
		key := convertPathToConfigMapKey(file.Name)
		instance.Spec.ExtraFiles.Files[key] = file.Name
	}
	instance.Spec.ExtraFiles.Name = newExtraFiles.Name
	return m.cli.Update(ctx, instance)
}

func (m *k8sRpaasManager) DeleteExtraFiles(ctx context.Context, instanceName string, filenames ...string) error {
	instance, err := m.GetInstance(ctx, instanceName)
	if err != nil {
		return err
	}
	extraFiles, err := m.getExtraFiles(ctx, *instance)
	if err != nil {
		return err
	}
	newData := map[string][]byte{}
	if extraFiles.BinaryData != nil {
		newData = extraFiles.BinaryData
	}
	for _, filename := range filenames {
		key := convertPathToConfigMapKey(filename)
		if _, ok := newData[key]; !ok {
			return &NotFoundError{Msg: fmt.Sprintf("file %q does not exist", filename)}
		}
		delete(newData, key)
	}
	if len(newData) == 0 {
		instance.Spec.ExtraFiles = nil
		return m.cli.Update(ctx, instance)
	}
	extraFiles, err = m.createExtraFiles(ctx, *instance, newData)
	if err != nil && k8sErrors.IsAlreadyExists(err) {
		return ConflictError{Msg: "extra files already is defined"}
	}
	if err != nil {
		return err
	}
	for _, filename := range filenames {
		key := convertPathToConfigMapKey(filename)
		delete(instance.Spec.ExtraFiles.Files, key)
	}
	instance.Spec.ExtraFiles.Name = extraFiles.Name
	return m.cli.Update(ctx, instance)
}

func (m *k8sRpaasManager) GetExtraFiles(ctx context.Context, instanceName string) ([]File, error) {
	instance, err := m.GetInstance(ctx, instanceName)
	if err != nil {
		return nil, err
	}
	extraFiles, err := m.getExtraFiles(ctx, *instance)
	if err != nil && IsNotFoundError(err) {
		return []File{}, nil
	}
	if err != nil {
		return nil, err
	}
	files := []File{}
	for key, path := range instance.Spec.ExtraFiles.Files {
		files = append(files, File{
			Name:    path,
			Content: extraFiles.BinaryData[key],
		})
	}
	return files, nil
}

func (m *k8sRpaasManager) UpdateExtraFiles(ctx context.Context, instanceName string, files ...File) error {
	instance, err := m.GetInstance(ctx, instanceName)
	if err != nil {
		return err
	}
	extraFiles, err := m.getExtraFiles(ctx, *instance)
	if err != nil {
		return err
	}
	newData := map[string][]byte{}
	if extraFiles.BinaryData != nil {
		newData = extraFiles.BinaryData
	}
	for _, file := range files {
		key := convertPathToConfigMapKey(file.Name)
		if _, ok := newData[key]; !ok {
			return &NotFoundError{Msg: fmt.Sprintf("file %q does not exist", file.Name)}
		}
		newData[key] = file.Content
	}
	extraFiles, err = m.createExtraFiles(ctx, *instance, newData)
	if err != nil && k8sErrors.IsAlreadyExists(err) {
		return ConflictError{Msg: "extra files already is defined"}
	}
	if err != nil {
		return err
	}
	instance.Spec.ExtraFiles.Name = extraFiles.Name
	return m.cli.Update(ctx, instance)
}

func (m *k8sRpaasManager) BindApp(ctx context.Context, instanceName string, args BindAppArgs) error {
	instance, err := m.GetInstance(ctx, instanceName)
	if err != nil {
		return err
	}

	if args.AppHost == "" {
		return &ValidationError{Msg: "application host cannot be empty"}
	}

	if len(instance.Spec.Binds) > 0 {
		for _, value := range instance.Spec.Binds {
			if value.Host == args.AppHost {
				return &ConflictError{Msg: "instance already bound with this application"}
			}
		}
	}
	if instance.Spec.Binds == nil {
		instance.Spec.Binds = make([]v1alpha1.Bind, 0)
	}

	instance.Spec.Binds = append(instance.Spec.Binds, v1alpha1.Bind{Host: args.AppHost, Name: args.AppName})

	return m.cli.Update(ctx, instance)
}

func (m *k8sRpaasManager) UnbindApp(ctx context.Context, instanceName, appName string) error {
	instance, err := m.GetInstance(ctx, instanceName)
	if err != nil {
		return err
	}

	if appName == "" {
		return &ValidationError{Msg: "must specify an app name"}
	}

	var found bool
	for i, bind := range instance.Spec.Binds {
		if bind.Name == appName {
			found = true
			binds := instance.Spec.Binds
			// Remove the element at index i from instance.Spec.Binds *maintaining it's order! -> O(n)*.
			instance.Spec.Binds = append(binds[:i], binds[i+1:]...)
			break
		}
	}

	if !found {
		return &NotFoundError{Msg: "app not found in instance bind list"}
	}

	return m.cli.Update(ctx, instance)
}

func (m *k8sRpaasManager) PurgeCache(ctx context.Context, instanceName string, args PurgeCacheArgs) (int, error) {
	nginx, podMap, err := m.GetInstanceStatus(ctx, instanceName)
	if err != nil {
		return 0, err
	}
	if args.Path == "" {
		return 0, ValidationError{Msg: "path is required"}
	}
	port := util.PortByName(nginx.Spec.PodTemplate.Ports, nginxManager.PortNameManagement)
	purgeCount := 0
	for _, podStatus := range podMap {
		if !podStatus.Running {
			continue
		}
		if err = m.cacheManager.PurgeCache(podStatus.Address, args.Path, port, args.PreservePath); err != nil {
			continue
		}
		purgeCount += 1
	}
	return purgeCount, nil
}

func (m *k8sRpaasManager) DeleteRoute(ctx context.Context, instanceName, path string) error {
	instance, err := m.GetInstance(ctx, instanceName)
	if err != nil {
		return err
	}

	index, found := hasPath(*instance, path)
	if !found {
		return &NotFoundError{Msg: "path does not exist"}
	}

	instance.Spec.Locations = append(instance.Spec.Locations[:index], instance.Spec.Locations[index+1:]...)
	return m.cli.Update(ctx, instance)
}

func (m *k8sRpaasManager) GetRoutes(ctx context.Context, instanceName string) ([]Route, error) {
	instance, err := m.GetInstance(ctx, instanceName)
	if err != nil {
		return nil, err
	}

	var routes []Route
	for _, location := range instance.Spec.Locations {
		var content string

		if location.Content != nil {
			content, err = util.GetValue(ctx, m.cli, instance.Namespace, location.Content)
			if err != nil {
				return nil, err
			}
		}

		if location.Destination == "" && content == "" {
			continue
		}

		routes = append(routes, Route{
			Path:        location.Path,
			Destination: location.Destination,
			HTTPSOnly:   location.ForceHTTPS,
			Content:     content,
		})
	}

	return routes, nil
}

func (m *k8sRpaasManager) UpdateRoute(ctx context.Context, instanceName string, route Route) error {
	instance, err := m.GetInstance(ctx, instanceName)
	if err != nil {
		return err
	}

	if err = validateRoute(route); err != nil {
		return err
	}

	var content *v1alpha1.Value
	if route.Content != "" {
		content = &v1alpha1.Value{Value: route.Content}
	}

	newLocation := v1alpha1.Location{
		Path:        route.Path,
		Destination: route.Destination,
		ForceHTTPS:  route.HTTPSOnly,
		Content:     content,
	}

	if index, found := hasPath(*instance, route.Path); found {
		instance.Spec.Locations[index] = newLocation
	} else {
		instance.Spec.Locations = append(instance.Spec.Locations, newLocation)
	}

	return m.cli.Update(ctx, instance)
}

func hasPath(instance v1alpha1.RpaasInstance, path string) (index int, found bool) {
	for i, location := range instance.Spec.Locations {
		if location.Path == path {
			return i, true
		}
	}

	return
}

func validateRoute(r Route) error {
	if r.Path == "" {
		return &ValidationError{Msg: "path is required"}
	}

	if !regexp.MustCompile(`^/[^ ]*`).MatchString(r.Path) {
		return &ValidationError{Msg: "invalid path format"}
	}

	if r.Content == "" && r.Destination == "" {
		return &ValidationError{Msg: "either content or destination are required"}
	}

	if r.Content != "" && r.Destination != "" {
		return &ValidationError{Msg: "cannot set both content and destination"}
	}

	if r.Content != "" && r.HTTPSOnly {
		return &ValidationError{Msg: "cannot set both content and httpsonly"}
	}

	return nil
}

func (m *k8sRpaasManager) createExtraFiles(ctx context.Context, instance v1alpha1.RpaasInstance, data map[string][]byte) (*corev1.ConfigMap, error) {
	hash := util.SHA256(data)
	cm := corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      fmt.Sprintf("%s-extra-files-%s", instance.Name, hash[:10]),
			Namespace: instance.Namespace,
			OwnerReferences: []metav1.OwnerReference{
				*metav1.NewControllerRef(&instance, schema.GroupVersionKind{
					Group:   v1alpha1.SchemeGroupVersion.Group,
					Version: v1alpha1.SchemeGroupVersion.Version,
					Kind:    "RpaasInstance",
				}),
			},
			Annotations: map[string]string{
				"rpaas.extensions.tsuru.io/sha256-hash": hash,
			},
		},
		BinaryData: data,
	}
	if err := m.cli.Create(ctx, &cm); err != nil && !k8sErrors.IsAlreadyExists(err) {
		return nil, err
	}
	return &cm, nil
}

func (m *k8sRpaasManager) getExtraFiles(ctx context.Context, instance v1alpha1.RpaasInstance) (*corev1.ConfigMap, error) {
	if instance.Spec.ExtraFiles == nil {
		return nil, &NotFoundError{Msg: "there are no extra files"}
	}
	configMapName := types.NamespacedName{
		Name:      instance.Spec.ExtraFiles.Name,
		Namespace: instance.Namespace,
	}
	configMap := corev1.ConfigMap{}
	if err := m.cli.Get(ctx, configMapName, &configMap); err != nil {
		return nil, err
	}
	return &configMap, nil
}

func (m *k8sRpaasManager) getPlan(ctx context.Context, name string) (*v1alpha1.RpaasPlan, error) {
	if name == "" {
		return m.getDefaultPlan(ctx)
	}

	planName := types.NamespacedName{
		Name:      name,
		Namespace: namespaceName(),
	}
	var plan v1alpha1.RpaasPlan
	if err := m.cli.Get(ctx, planName, &plan); err != nil {
		if !k8sErrors.IsNotFound(err) {
			return nil, err
		}

		return nil, NotFoundError{Msg: fmt.Sprintf("plan %q not found", name)}
	}

	return &plan, nil
}

func (m *k8sRpaasManager) getDefaultPlan(ctx context.Context) (*v1alpha1.RpaasPlan, error) {
	plans, err := m.GetPlans(ctx)
	if err != nil {
		return nil, err
	}

	var defaultPlans []v1alpha1.RpaasPlan
	for _, p := range plans {
		if p.Spec.Default {
			defaultPlans = append(defaultPlans, p)
		}
	}

	switch len(defaultPlans) {
	case 0:
		return nil, NotFoundError{Msg: "no default plan found"}
	case 1:
		return &defaultPlans[0], nil
	default:
		var names []string
		for _, p := range defaultPlans {
			names = append(names, p.Name)
		}
		return nil, ConflictError{Msg: fmt.Sprintf("several default plans found: %v", strings.Join(names, ","))}
	}
}

func getRawCertificateAndKey(c tls.Certificate) ([]byte, []byte, error) {
	certificatePem, err := convertCertificateToPem(c.Certificate)
	if err != nil {
		return []byte{}, []byte{}, err
	}
	keyPem, err := convertPrivateKeyToPem(c.PrivateKey)
	if err != nil {
		return []byte{}, []byte{}, err
	}
	return certificatePem, keyPem, err
}

func convertCertificateToPem(certificate [][]byte) ([]byte, error) {
	buffer := &bytes.Buffer{}
	for _, derBytes := range certificate {
		pemBlock := &pem.Block{
			Type:  "CERTIFICATE",
			Bytes: derBytes,
		}
		if err := pem.Encode(buffer, pemBlock); err != nil {
			return []byte{}, err
		}
	}
	return buffer.Bytes(), nil
}

func convertPrivateKeyToPem(key crypto.PrivateKey) ([]byte, error) {
	switch k := key.(type) {
	case *rsa.PrivateKey:
		return pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(k)}), nil
	case *ecdsa.PrivateKey:
		bytes, err := x509.MarshalECPrivateKey(k)
		if err != nil {
			return nil, err
		}
		return pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: bytes}), nil
	default:
		return nil, errors.New("manager: unsupported private key")
	}
}

func (m *k8sRpaasManager) validateCreate(ctx context.Context, args CreateArgs) error {
	if args.Name == "" {
		return ValidationError{Msg: "name is required"}
	}

	if args.Team == "" {
		return ValidationError{Msg: "team name is required"}
	}

	if _, err := m.getPlan(ctx, args.Plan); err != nil && IsNotFoundError(err) {
		return ValidationError{Msg: "invalid plan"}
	}

	_, err := m.GetInstance(ctx, args.Name)
	if err != nil && !IsNotFoundError(err) {
		return err
	}

	if err == nil {
		return ConflictError{Msg: fmt.Sprintf("rpaas instance named %q already exists", args.Name)}
	}

	return nil
}

func parseTagArg(tags []string, name string, destination *string) {
	for _, tag := range tags {
		parts := strings.SplitN(tag, "=", 2)
		if parts[0] == name {
			*destination = parts[1]
			break
		}
	}
}

func isBlockTypeAllowed(bt v1alpha1.BlockType) bool {
	allowedBlockTypes := map[v1alpha1.BlockType]bool{
		v1alpha1.BlockTypeRoot:      true,
		v1alpha1.BlockTypeServer:    true,
		v1alpha1.BlockTypeHTTP:      true,
		v1alpha1.BlockTypeLuaServer: true,
		v1alpha1.BlockTypeLuaWorker: true,
	}

	_, ok := allowedBlockTypes[bt]
	return ok
}

func (m *k8sRpaasManager) GetInstanceStatus(ctx context.Context, name string) (*nginxv1alpha1.Nginx, PodStatusMap, error) {
	rpaasInstance, err := m.GetInstance(ctx, name)
	if err != nil {
		return nil, nil, err
	}
	var nginx nginxv1alpha1.Nginx
	err = m.cli.Get(ctx, types.NamespacedName{Name: rpaasInstance.Name, Namespace: rpaasInstance.Namespace}, &nginx)
	if err != nil {
		return nil, nil, err
	}
	podMap := PodStatusMap{}
	for _, podInfo := range nginx.Status.Pods {
		st, err := m.podStatus(ctx, podInfo.Name, rpaasInstance.Namespace)
		if err != nil {
			st = PodStatus{
				Running: false,
				Status:  fmt.Sprintf("%+v", err),
			}
		}
		podMap[podInfo.Name] = st
	}
	return &nginx, podMap, nil
}

func (m *k8sRpaasManager) podStatus(ctx context.Context, podName, ns string) (PodStatus, error) {
	var pod corev1.Pod
	err := m.cli.Get(ctx, types.NamespacedName{
		Name:      podName,
		Namespace: ns,
	}, &pod)
	if err != nil {
		return PodStatus{}, err
	}
	evts, err := m.eventsForPod(ctx, pod.Name, pod.Namespace)
	if err != nil {
		return PodStatus{}, err
	}
	allRunning := true
	for _, cs := range pod.Status.ContainerStatuses {
		allRunning = allRunning && cs.Ready
	}
	return PodStatus{
		Address: pod.Status.PodIP,
		Running: allRunning,
		Status:  formatPodEvents(evts),
	}, nil
}

func (m *k8sRpaasManager) eventsForPod(ctx context.Context, podName, ns string) ([]corev1.Event, error) {
	const podKind = "Pod"
	listOpts := &client.ListOptions{Namespace: ns}
	client.MatchingFields(fields.Set{
		"involvedObject.kind": podKind,
		"involvedObject.name": podName,
	}).ApplyToList(listOpts)
	var eventList corev1.EventList
	err := m.nonCachedCli.List(ctx, &eventList, listOpts)
	if err != nil {
		return nil, err
	}
	for i := 0; i < len(eventList.Items); i++ {
		if eventList.Items[i].InvolvedObject.Kind != podKind ||
			eventList.Items[i].InvolvedObject.Name != podName {
			eventList.Items[i] = eventList.Items[len(eventList.Items)-1]
			eventList.Items = eventList.Items[:len(eventList.Items)-1]
			i--
		}
	}
	return eventList.Items, nil
}

func newSecretForCertificates(instance v1alpha1.RpaasInstance, data map[string][]byte) *corev1.Secret {
	hash := util.SHA256(data)
	return &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      fmt.Sprintf("%s-certificates-%s", instance.Name, hash[:10]),
			Namespace: instance.Namespace,
			OwnerReferences: []metav1.OwnerReference{
				*metav1.NewControllerRef(&instance, schema.GroupVersionKind{
					Group:   v1alpha1.SchemeGroupVersion.Group,
					Version: v1alpha1.SchemeGroupVersion.Version,
					Kind:    "RpaasInstance",
				}),
			},
			Annotations: map[string]string{
				"rpaas.extensions.tsuru.io/sha256-hash": hash,
			},
		},
		Data: data,
	}
}

func formatPodEvents(events []corev1.Event) string {
	var statuses []string
	for _, evt := range events {
		component := []string{evt.Source.Component}
		if evt.Source.Host != "" {
			component = append(component, evt.Source.Host)
		}
		statuses = append(statuses, fmt.Sprintf("%s [%s]",
			evt.Message,
			strings.Join(component, ", "),
		))
	}
	return strings.Join(statuses, "\n")
}

func isPathValid(p string) bool {
	return !regexp.MustCompile(`(^/|[.]{2})`).MatchString(p)
}

func convertPathToConfigMapKey(s string) string {
	return regexp.MustCompile("[^a-zA-Z0-9._-]+").ReplaceAllString(s, "_")
}

func labelsForRpaasInstance(name string) map[string]string {
	return map[string]string{
		labelKey("service-name"):  getServiceName(),
		labelKey("instance-name"): name,
		"rpaas_service":           getServiceName(),
		"rpaas_instance":          name,
	}
}

func labelKey(name string) string {
	return fmt.Sprintf("%s/%s", defaultKeyLabelPrefix, name)
}

func getServiceName() string {
	serviceName := config.Get().ServiceName
	if serviceName == "" {
		return defaultNamespace
	}

	return serviceName
}

func namespaceName() string {
	return getServiceName()
}

func newNamespace(name string) corev1.Namespace {
	return corev1.Namespace{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "v1",
			Kind:       "Namespace",
		},
		ObjectMeta: metav1.ObjectMeta{Name: name},
	}
}

func newRpaasInstance(name string) *v1alpha1.RpaasInstance {
	return &v1alpha1.RpaasInstance{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "extensions.tsuru.io/v1alpha1",
			Kind:       "RpaasInstance",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespaceName(),
			Labels:    labelsForRpaasInstance(name),
		},
		Spec: v1alpha1.RpaasInstanceSpec{},
	}
}

func getAffinity(team string) *corev1.Affinity {
	conf := config.Get()
	if conf.TeamAffinity != nil {
		if teamAffinity, ok := conf.TeamAffinity[team]; ok {
			return &teamAffinity
		}
	}

	return conf.DefaultAffinity
}

func mergeMap(a, b map[string]string) map[string]string {
	if a == nil {
		return b
	}
	for k, v := range b {
		a[k] = v
	}
	return a
}

func setDescription(instance *v1alpha1.RpaasInstance, description string) {
	if instance == nil {
		return
	}

	instance.Annotations = mergeMap(instance.Annotations, map[string]string{
		labelKey("description"): description,
	})
}

func (m *k8sRpaasManager) setTags(ctx context.Context, instance *v1alpha1.RpaasInstance, tags []string) error {
	if instance == nil {
		return nil
	}

	sort.Strings(tags)

	instance.Annotations = mergeMap(instance.Annotations, map[string]string{
		labelKey("tags"): strings.Join(tags, ","),
	})

	var ip string
	parseTagArg(tags, "ip", &ip)
	if instance.Spec.Service == nil {
		instance.Spec.Service = &nginxv1alpha1.NginxService{}
	}
	instance.Spec.Service.LoadBalancerIP = ip

	var flavor string
	parseTagArg(tags, "flavor", &flavor)
	if err := m.setFlavors(ctx, instance, strings.Split(flavor, ",")); err != nil {
		return err
	}

	var planOverride string
	parseTagArg(tags, "plan-override", &planOverride)
	instance.Spec.PlanTemplate = nil
	if planOverride == "" {
		return nil
	}

	var planTemplate v1alpha1.RpaasPlanSpec
	if err := json.Unmarshal([]byte(planOverride), &planTemplate); err != nil {
		return errors.Wrapf(err, "unable to parse plan-override from data %q", planOverride)
	}
	instance.Spec.PlanTemplate = &planTemplate

	return nil
}

func (m *k8sRpaasManager) setFlavors(ctx context.Context, instance *v1alpha1.RpaasInstance, flavorNames []string) error {
	var flavors []string
	for _, flavor := range flavorNames {
		if flavor == "" {
			break
		}

		if !m.isFlavorAvailable(ctx, flavor) {
			return NotFoundError{Msg: fmt.Sprintf("flavor %q not found", flavor)}
		}

		flavors = append(flavors, flavor)
	}

	instance.Spec.Flavors = flavors
	return nil
}

func setTeamOwner(instance *v1alpha1.RpaasInstance, team string) {
	if instance == nil {
		return
	}

	newLabels := map[string]string{labelKey("team-owner"): team}

	instance.Annotations = mergeMap(instance.Annotations, newLabels)
	instance.Labels = mergeMap(instance.Labels, newLabels)
	instance.Spec.PodTemplate.Labels = mergeMap(instance.Spec.PodTemplate.Labels, newLabels)
}

func newInstanceInfo(instance *v1alpha1.RpaasInstance, ingresses []corev1.LoadBalancerIngress) *clientTypes.InstanceInfo {
	info := &clientTypes.InstanceInfo{
		Replicas:  instance.Spec.Replicas,
		Plan:      instance.Spec.PlanName,
		Locations: instance.Spec.Locations,
		Autoscale: instance.Spec.Autoscale,
		Binds:     instance.Spec.Binds,
		Name:      instance.ObjectMeta.Name,
	}

	if desc, ok := instance.ObjectMeta.Annotations["description"]; ok {
		info.Description = desc
	}

	info.Tags = strings.Split(instance.ObjectMeta.Annotations["tags"], ",")

	setInfoTeam(instance, info)

	for _, ingress := range ingresses {
		info.Address = append(info.Address, clientTypes.InstanceAddress{
			Hostname: ingress.Hostname,
			IP:       ingress.IP,
		})
	}

	return info
}

func (m *k8sRpaasManager) getLoadBalancerIngress(ctx context.Context, instance *v1alpha1.RpaasInstance) ([]corev1.LoadBalancerIngress, error) {
	var nginx nginxv1alpha1.Nginx
	err := m.cli.Get(ctx, types.NamespacedName{Name: instance.Name, Namespace: instance.Namespace}, &nginx)
	if err != nil {
		return nil, err
	}

	if len(nginx.Status.Services) == 0 {
		return nil, nil
	}

	svcName := nginx.Status.Services[0].Name
	var svc corev1.Service
	err = m.cli.Get(ctx, types.NamespacedName{Name: svcName, Namespace: instance.Namespace}, &svc)
	if err != nil {
		return nil, err
	}

	return svc.Status.LoadBalancer.Ingress, nil
}

func setInfoTeam(instance *v1alpha1.RpaasInstance, infoPayload *clientTypes.InstanceInfo) {
	teamLabelKey := labelKey("team-owner")
	team, ok := instance.ObjectMeta.Annotations[teamLabelKey]
	if ok {
		infoPayload.Team = team
		return
	}

	team, ok = instance.Labels[teamLabelKey]
	if ok {
		infoPayload.Team = team
		return
	}

	team, ok = instance.Spec.PodTemplate.Labels[teamLabelKey]
	if ok {
		infoPayload.Team = team
		return
	}
}

func (m *k8sRpaasManager) GetInstanceInfo(ctx context.Context, instanceName string) (*clientTypes.InstanceInfo, error) {
	instance, err := m.GetInstance(ctx, instanceName)
	if err != nil {
		return nil, err
	}

	ingresses, err := m.getLoadBalancerIngress(ctx, instance)
	if err != nil {
		return nil, err
	}

	return newInstanceInfo(instance, ingresses), nil
}
