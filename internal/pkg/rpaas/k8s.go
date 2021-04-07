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
	"net"
	"net/url"
	"reflect"
	"regexp"
	"sort"
	"strings"
	"text/template"
	"time"

	jsonpatch "github.com/evanphx/json-patch/v5"
	"github.com/hashicorp/go-multierror"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	nginxv1alpha1 "github.com/tsuru/nginx-operator/api/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/api/core/v1"
	k8sErrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/httpstream/spdy"
	"k8s.io/apimachinery/pkg/util/validation"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/remotecommand"
	"k8s.io/kubectl/pkg/scheme"
	metricsv1beta1 "k8s.io/metrics/pkg/apis/metrics/v1beta1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	osb "sigs.k8s.io/go-open-service-broker-client/v2"

	"github.com/tsuru/rpaas-operator/api/v1alpha1"
	"github.com/tsuru/rpaas-operator/internal/config"
	nginxManager "github.com/tsuru/rpaas-operator/internal/pkg/rpaas/nginx"
	clientTypes "github.com/tsuru/rpaas-operator/pkg/rpaas/client/types"
	"github.com/tsuru/rpaas-operator/pkg/util"
)

const (
	defaultNamespace      = "rpaasv2"
	defaultKeyLabelPrefix = "rpaas.extensions.tsuru.io"

	externalDNSHostnameLabel = "external-dns.alpha.kubernetes.io/hostname"

	nginxContainerName = "nginx"
)

var _ RpaasManager = &k8sRpaasManager{}

type k8sRpaasManager struct {
	cli          client.Client
	cacheManager CacheManager
	restConfig   *rest.Config
	kcs          *kubernetes.Clientset
	clusterName  string
}

func NewK8S(cfg *rest.Config, k8sClient client.Client, clusterName string) (RpaasManager, error) {
	m := &k8sRpaasManager{
		cli:          k8sClient,
		cacheManager: nginxManager.NewNginxManager(),
		restConfig:   cfg,
		clusterName:  clusterName,
	}

	if cfg == nil {
		return m, nil
	}

	kcs, err := kubernetes.NewForConfig(m.restConfig)
	if err != nil {
		return nil, err
	}
	m.kcs = kcs

	return m, nil
}

func keepAliveSpdyExecutor(config *rest.Config, method string, url *url.URL) (remotecommand.Executor, error) {
	tlsConfig, err := rest.TLSConfigFor(config)
	if err != nil {
		return nil, err
	}
	upgradeRoundTripper := spdy.NewRoundTripper(tlsConfig, true, false)
	upgradeRoundTripper.Dialer = &net.Dialer{
		Timeout:   30 * time.Second,
		KeepAlive: 10 * time.Second,
	}
	wrapper, err := rest.HTTPWrappersForConfig(config, upgradeRoundTripper)
	if err != nil {
		return nil, err
	}
	return remotecommand.NewSPDYExecutorForTransports(wrapper, upgradeRoundTripper, method, url)
}

type fixedSizeQueue struct {
	sz *remotecommand.TerminalSize
}

func (q *fixedSizeQueue) Next() *remotecommand.TerminalSize {
	defer func() { q.sz = nil }()
	return q.sz
}

func (m *k8sRpaasManager) Exec(ctx context.Context, instanceName string, args ExecArgs) error {
	instance, err := m.GetInstance(ctx, instanceName)
	if err != nil {
		return err
	}

	nginx, err := m.getNginx(ctx, instance)
	if err != nil {
		return err
	}

	podsInfo, err := m.getPodStatuses(ctx, nginx)
	if err != nil {
		return err
	}

	if args.Pod == "" {
		for _, ps := range podsInfo {
			if strings.EqualFold(ps.Status, "Running") {
				args.Pod = ps.Name
			}
		}
	} else {
		var podFound bool
		for _, ps := range podsInfo {
			if ps.Name == args.Pod {
				podFound = true
				break
			}
		}

		if !podFound {
			return fmt.Errorf("no such pod %s in instance %s", args.Pod, instanceName)
		}
	}

	if args.Container == "" {
		args.Container = "nginx"
	}

	req := m.kcs.
		CoreV1().
		RESTClient().
		Post().
		Resource("pods").
		Name(args.Pod).
		Namespace(instance.Namespace).
		SubResource("exec").
		VersionedParams(&corev1.PodExecOptions{
			Container: args.Container,
			Command:   args.Command,
			Stdin:     args.Stdin != nil,
			Stdout:    true,
			Stderr:    true,
			TTY:       args.TTY,
		}, scheme.ParameterCodec)

	executor, err := keepAliveSpdyExecutor(m.restConfig, "POST", req.URL())
	if err != nil {
		return err
	}

	var tsq remotecommand.TerminalSizeQueue
	if args.TerminalWidth != uint16(0) && args.TerminalHeight != uint16(0) {
		tsq = &fixedSizeQueue{
			sz: &remotecommand.TerminalSize{
				Width:  uint16(args.TerminalWidth),
				Height: uint16(args.TerminalHeight),
			},
		}
	}

	return executor.Stream(remotecommand.StreamOptions{
		Stdin:             args.Stdin,
		Stdout:            args.Stdout,
		Stderr:            args.Stderr,
		Tty:               args.TTY,
		TerminalSizeQueue: tsq,
	})
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
	instance.Spec = v1alpha1.RpaasInstanceSpec{
		Replicas: func(n int32) *int32 { return &n }(int32(1)),
		PlanName: plan.Name,
		Flavors:  args.Flavors(),
		Service: &nginxv1alpha1.NginxService{
			Type:        corev1.ServiceTypeLoadBalancer,
			Annotations: instance.Annotations,
			Labels:      instance.Labels,
		},
		PodTemplate: nginxv1alpha1.NginxPodTemplateSpec{
			Affinity:    getAffinity(args.Team),
			Annotations: instance.Annotations,
			Labels:      instance.Labels,
		},
		RolloutNginxOnce: true,
	}

	setDescription(instance, args.Description)
	instance.SetTeamOwner(args.Team)
	if m.clusterName != "" {
		instance.SetClusterName(m.clusterName)
	}
	setTags(instance, args.Tags)
	setIP(instance, args.IP())
	setLoadBalancerName(instance, args.LoadBalancerName())

	if err = setPlanTemplate(instance, args.PlanOverride()); err != nil {
		return err
	}

	return m.cli.Create(ctx, instance)
}

func (m *k8sRpaasManager) UpdateInstance(ctx context.Context, instanceName string, args UpdateInstanceArgs) error {
	if err := m.validateUpdateInstanceArgs(ctx, args); err != nil {
		return err
	}

	instance, err := m.GetInstance(ctx, instanceName)
	if err != nil {
		return err
	}
	originalInstance := instance.DeepCopy()
	if args.Plan != "" && args.Plan != instance.Spec.PlanName {
		plan, err := m.getPlan(ctx, args.Plan)
		if err != nil {
			return err
		}

		instance.Spec.PlanName = plan.Name
	}

	instance.Spec.Flavors = args.Flavors()

	setDescription(instance, args.Description)
	instance.SetTeamOwner(args.Team)
	if m.clusterName != "" {
		instance.SetClusterName(m.clusterName)
	}
	setTags(instance, args.Tags)
	setIP(instance, args.IP())
	setLoadBalancerName(instance, args.LoadBalancerName())

	if err := setPlanTemplate(instance, args.PlanOverride()); err != nil {
		return err
	}

	return m.patchInstance(ctx, originalInstance, instance)
}

func (m *k8sRpaasManager) ensureNamespaceExists(ctx context.Context) (string, error) {
	nsName := getServiceName()
	ns := newNamespace(nsName)
	if err := m.cli.Create(ctx, &ns); err != nil && !k8sErrors.IsAlreadyExists(err) {
		return "", err
	}

	return nsName, nil
}

func (m *k8sRpaasManager) GetAutoscale(ctx context.Context, instanceName string) (*clientTypes.Autoscale, error) {
	instance, err := m.GetInstance(ctx, instanceName)
	if err != nil {
		return nil, err
	}

	autoscale := instance.Spec.Autoscale
	if autoscale == nil {
		return nil, NotFoundError{Msg: "autoscale not found"}
	}

	s := clientTypes.Autoscale{
		MinReplicas: autoscale.MinReplicas,
		MaxReplicas: &autoscale.MaxReplicas,
		CPU:         autoscale.TargetCPUUtilizationPercentage,
		Memory:      autoscale.TargetMemoryUtilizationPercentage,
	}

	return &s, nil
}

func (m *k8sRpaasManager) CreateAutoscale(ctx context.Context, instanceName string, autoscale *clientTypes.Autoscale) error {
	instance, err := m.GetInstance(ctx, instanceName)
	if err != nil {
		return err
	}
	originalInstance := instance.DeepCopy()

	err = validateAutoscale(ctx, autoscale)
	if err != nil {
		return err
	}

	s := instance.Spec.Autoscale
	if s != nil {
		return ValidationError{Msg: "Autoscale already created"}
	}

	instance.Spec.Autoscale = &v1alpha1.RpaasInstanceAutoscaleSpec{
		MinReplicas:                       autoscale.MinReplicas,
		MaxReplicas:                       *autoscale.MaxReplicas,
		TargetCPUUtilizationPercentage:    autoscale.CPU,
		TargetMemoryUtilizationPercentage: autoscale.Memory,
	}

	return m.patchInstance(ctx, originalInstance, instance)
}

func (m *k8sRpaasManager) UpdateAutoscale(ctx context.Context, instanceName string, autoscale *clientTypes.Autoscale) error {
	instance, err := m.GetInstance(ctx, instanceName)
	if err != nil {
		return err
	}
	originalInstance := instance.DeepCopy()

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

	return m.patchInstance(ctx, originalInstance, instance)
}

func (m *k8sRpaasManager) DeleteAutoscale(ctx context.Context, instanceName string) error {
	instance, err := m.GetInstance(ctx, instanceName)
	if err != nil {
		return err
	}
	originalInstance := instance.DeepCopy()

	instance.Spec.Autoscale = nil

	return m.patchInstance(ctx, originalInstance, instance)
}

func validateAutoscale(ctx context.Context, s *clientTypes.Autoscale) error {
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

	originalInstance := instance.DeepCopy()

	if instance.Spec.Blocks == nil {
		return NotFoundError{Msg: fmt.Sprintf("block %q not found", blockName)}
	}

	blockType := v1alpha1.BlockType(blockName)
	if _, ok := instance.Spec.Blocks[blockType]; !ok {
		return NotFoundError{Msg: fmt.Sprintf("block %q not found", blockName)}
	}

	delete(instance.Spec.Blocks, blockType)
	return m.patchInstance(ctx, originalInstance, instance)
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

	originalInstance := instance.DeepCopy()

	blockType := v1alpha1.BlockType(block.Name)
	if !isBlockTypeAllowed(blockType) {
		return ValidationError{Msg: fmt.Sprintf("block %q is not allowed", block.Name)}
	}

	if instance.Spec.Blocks == nil {
		instance.Spec.Blocks = make(map[v1alpha1.BlockType]v1alpha1.Value)
	}

	instance.Spec.Blocks[blockType] = v1alpha1.Value{Value: block.Content}

	return m.patchInstance(ctx, originalInstance, instance)
}

func (m *k8sRpaasManager) Scale(ctx context.Context, instanceName string, replicas int32) error {
	instance, err := m.GetInstance(ctx, instanceName)
	if err != nil {
		return err
	}
	originalInstance := instance.DeepCopy()
	if replicas < 0 {
		return ValidationError{Msg: fmt.Sprintf("invalid replicas number: %d", replicas)}
	}
	instance.Spec.Replicas = &replicas
	return m.patchInstance(ctx, originalInstance, instance)
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

	originalInstance := instance.DeepCopy()

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

	name = certificateName(name)
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
		return &NotFoundError{Msg: "certificate not found"}
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

	return m.patchInstance(ctx, originalInstance, instance)
}

func (m *k8sRpaasManager) UpdateCertificate(ctx context.Context, instanceName, name string, c tls.Certificate) error {
	instance, err := m.GetInstance(ctx, instanceName)
	if err != nil {
		return err
	}

	originalInstance := instance.DeepCopy()

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

	name = certificateName(name)
	if errs := validation.IsConfigMapKey(name); len(errs) > 0 {
		return ValidationError{Msg: fmt.Sprintf("certificate name is not valid: %s", strings.Join(errs, ": "))}
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

	return m.patchInstance(ctx, originalInstance, instance)
}

func (m *k8sRpaasManager) GetInstanceAddress(ctx context.Context, name string) (string, error) {
	instance, err := m.GetInstance(ctx, name)
	if err != nil {
		return "", err
	}

	nginx, err := m.getNginx(ctx, instance)
	if err != nil && k8sErrors.IsNotFound(err) {
		return "", nil
	}

	if err != nil {
		return "", err
	}

	addresses, err := m.getInstanceAddresses(ctx, nginx)
	if err != nil {
		return "", err
	}

	if len(addresses) == 0 {
		return "", nil
	}

	if addresses[0].IP == "" {
		return addresses[0].Hostname, nil
	}

	return addresses[0].IP, nil
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

func (m *k8sRpaasManager) GetPlans(ctx context.Context) ([]Plan, error) {
	var planList v1alpha1.RpaasPlanList
	if err := m.cli.List(ctx, &planList, client.InNamespace(namespaceName())); err != nil {
		return nil, err
	}

	flavors, err := m.GetFlavors(ctx)
	if err != nil {
		return nil, err
	}

	var schemas *osb.Schemas
	if p := buildServiceInstanceParametersForPlan(flavors); p != nil {
		schemas = &osb.Schemas{
			ServiceInstance: &osb.ServiceInstanceSchema{
				Create: &osb.InputParametersSchema{Parameters: p},
				Update: &osb.InputParametersSchema{Parameters: p},
			},
		}
	}

	var plans []Plan
	for _, p := range planList.Items {
		plans = append(plans, Plan{
			Name:        p.Name,
			Description: p.Spec.Description,
			Schemas:     schemas,
		})
	}

	sort.SliceStable(plans, func(i, j int) bool { return plans[i].Name < plans[j].Name })

	return plans, nil
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

func (m *k8sRpaasManager) isFlavorAvailable(ctx context.Context, name string) bool {
	flavors, err := m.getFlavors(ctx)
	if err != nil {
		return false
	}

	for _, f := range flavors {
		if f.Name == name {
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
	originalInstance := instance.DeepCopy()
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
	return m.patchInstance(ctx, originalInstance, instance)
}

func (m *k8sRpaasManager) DeleteExtraFiles(ctx context.Context, instanceName string, filenames ...string) error {
	instance, err := m.GetInstance(ctx, instanceName)
	if err != nil {
		return err
	}
	originalInstance := instance.DeepCopy()
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
		return m.patchInstance(ctx, originalInstance, instance)
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
	return m.patchInstance(ctx, originalInstance, instance)
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
	originalInstance := instance.DeepCopy()
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
	return m.patchInstance(ctx, originalInstance, instance)
}

func (m *k8sRpaasManager) BindApp(ctx context.Context, instanceName string, args BindAppArgs) error {
	instance, err := m.GetInstance(ctx, instanceName)
	if err != nil {
		return err
	}

	originalInstance := instance.DeepCopy()

	var host string
	if args.AppClusterName != "" && instance.BelongsToCluster(args.AppClusterName) {
		if len(args.AppInternalHosts) == 0 || args.AppInternalHosts[0] == "" {
			return &ValidationError{Msg: "application internal hosts cannot be empty"}
		}

		host = args.AppInternalHosts[0]
	} else {
		if len(args.AppHosts) == 0 || args.AppHosts[0] == "" {
			return &ValidationError{Msg: "application hosts cannot be empty"}
		}

		host = args.AppHosts[0]
	}

	u, err := url.Parse(host)
	if err != nil {
		return err
	}
	if u.Scheme == "tcp" {
		host = u.Host
	}

	if u.Scheme == "udp" {
		return &ValidationError{Msg: fmt.Sprintf("Unsupported host: %q", host)}
	}

	if len(instance.Spec.Binds) > 0 {
		for _, value := range instance.Spec.Binds {
			if value.Host == host {
				return &ConflictError{Msg: "instance already bound with this application"}
			}
		}
	}
	if instance.Spec.Binds == nil {
		instance.Spec.Binds = make([]v1alpha1.Bind, 0)
	}

	instance.Spec.Binds = append(instance.Spec.Binds, v1alpha1.Bind{Host: host, Name: args.AppName})

	return m.patchInstance(ctx, originalInstance, instance)
}

func (m *k8sRpaasManager) UnbindApp(ctx context.Context, instanceName, appName string) error {
	instance, err := m.GetInstance(ctx, instanceName)
	if err != nil {
		return err
	}

	originalInstance := instance.DeepCopy()

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

	return m.patchInstance(ctx, originalInstance, instance)
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
	var purgeErrors error
	purgeCount := 0
	for _, podStatus := range podMap {
		if !podStatus.Running {
			continue
		}
		if err = m.cacheManager.PurgeCache(podStatus.Address, args.Path, port, args.PreservePath); err != nil {
			purgeErrors = multierror.Append(purgeErrors, errors.Wrapf(err, "pod %s failed", podStatus.Address))
			continue
		}
		purgeCount++
	}
	return purgeCount, purgeErrors
}

func (m *k8sRpaasManager) DeleteRoute(ctx context.Context, instanceName, path string) error {
	instance, err := m.GetInstance(ctx, instanceName)
	if err != nil {
		return err
	}

	originalInstance := instance.DeepCopy()

	index, found := hasPath(*instance, path)
	if !found {
		return &NotFoundError{Msg: "path does not exist"}
	}

	instance.Spec.Locations = append(instance.Spec.Locations[:index], instance.Spec.Locations[index+1:]...)
	return m.patchInstance(ctx, originalInstance, instance)
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
	originalInstance := instance.DeepCopy()

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

	return m.patchInstance(ctx, originalInstance, instance)
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
					Group:   v1alpha1.GroupVersion.Group,
					Version: v1alpha1.GroupVersion.Version,
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
		pp, err := m.getPlan(ctx, p.Name)
		if err != nil {
			return nil, err
		}

		if pp.Spec.Default {
			defaultPlans = append(defaultPlans, *pp)
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

	if len(args.Name) > 30 {
		return ValidationError{Msg: "instance name cannot length up than 30 chars"}
	}

	if errs := validation.IsDNS1123Label(args.Name); len(errs) > 0 {
		return ValidationError{Msg: fmt.Sprintf("instance name is not valid: %s", strings.Join(errs, ": "))}
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

	if err = m.validateFlavors(ctx, args.Flavors()); err != nil {
		return err
	}

	return nil
}

func (m *k8sRpaasManager) validateUpdateInstanceArgs(ctx context.Context, args UpdateInstanceArgs) error {
	if err := m.validateFlavors(ctx, args.Flavors()); err != nil {
		return err
	}

	return nil
}

func (m *k8sRpaasManager) validateFlavors(ctx context.Context, flavors []string) error {
	encountered := map[string]bool{}
	for _, f := range flavors {
		if !m.isFlavorAvailable(ctx, f) {
			return ValidationError{Msg: fmt.Sprintf("flavor %q not found", f)}
		}

		if _, duplicated := encountered[f]; duplicated {
			return ValidationError{Msg: fmt.Sprintf("flavor %q only can be set once", f)}
		}

		encountered[f] = true
	}

	return nil
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
	instance, err := m.GetInstance(ctx, name)
	if err != nil {
		return nil, nil, err
	}

	nginx, err := m.getNginx(ctx, instance)
	if err != nil {
		return nil, nil, err
	}

	pods, err := m.getPods(ctx, nginx)
	if err != nil {
		return nil, nil, err
	}

	podMap := PodStatusMap{}
	for _, pod := range pods {
		st, err := m.podStatus(ctx, &pod)
		if err != nil {
			st = PodStatus{
				Running: false,
				Status:  fmt.Sprintf("%+v", err),
			}
		}
		podMap[pod.Name] = st
	}

	return nginx, podMap, nil
}

func (m *k8sRpaasManager) podStatus(ctx context.Context, pod *corev1.Pod) (PodStatus, error) {
	evts, err := m.eventsForPod(ctx, pod)
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

func (m *k8sRpaasManager) eventsForPod(ctx context.Context, pod *corev1.Pod) ([]corev1.Event, error) {
	const podKind = "Pod"
	return m.eventsForObject(ctx, pod.ObjectMeta.Namespace, podKind, pod.ObjectMeta.UID)
}

func (m *k8sRpaasManager) eventsForObject(ctx context.Context, namespace, kind string, uid types.UID) ([]corev1.Event, error) {
	listOpts := &client.ListOptions{
		FieldSelector: fields.Set{
			"involvedObject.kind": kind,
			"involvedObject.uid":  string(uid),
		}.AsSelector(),
		Namespace: namespace,
	}
	var eventList corev1.EventList
	if err := m.cli.List(ctx, &eventList, listOpts); err != nil {
		return nil, err
	}

	// NOTE: re-applying the above filter to work on unit tests as well.
	events := eventList.Items
	for i := 0; i < len(events); i++ {
		if events[i].InvolvedObject.Kind != kind || events[i].InvolvedObject.UID != uid {
			events[i] = events[len(events)-1]
			events = eventList.Items[:len(events)-1]
			i--
		}
	}

	sort.Slice(events, func(i, j int) bool {
		return events[i].CreationTimestamp.After(events[j].CreationTimestamp.Time)
	})

	return events, nil
}

func newSecretForCertificates(instance v1alpha1.RpaasInstance, data map[string][]byte) *corev1.Secret {
	hash := util.SHA256(data)
	return &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      fmt.Sprintf("%s-certificates-%s", instance.Name, hash[:10]),
			Namespace: instance.Namespace,
			OwnerReferences: []metav1.OwnerReference{
				*metav1.NewControllerRef(&instance, schema.GroupVersionKind{
					Group:   v1alpha1.GroupVersion.Group,
					Version: v1alpha1.GroupVersion.Version,
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

func setIP(instance *v1alpha1.RpaasInstance, ip string) {
	if instance == nil {
		return
	}

	if instance.Spec.Service == nil {
		instance.Spec.Service = &nginxv1alpha1.NginxService{}
	}

	instance.Spec.Service.LoadBalancerIP = ip
}

func setPlanTemplate(instance *v1alpha1.RpaasInstance, override string) error {
	if instance == nil {
		return nil
	}

	instance.Spec.PlanTemplate = nil
	if override == "" {
		return nil
	}

	var planTemplate v1alpha1.RpaasPlanSpec
	if err := json.Unmarshal([]byte(override), &planTemplate); err != nil {
		return fmt.Errorf("unable to unmarshal plan-override on plan spec: %w", err)
	}

	instance.Spec.PlanTemplate = &planTemplate
	return nil
}

func setTags(instance *v1alpha1.RpaasInstance, tags []string) {
	if instance == nil {
		return
	}

	sort.Strings(tags)

	instance.Annotations = mergeMap(instance.Annotations, map[string]string{
		labelKey("tags"): strings.Join(tags, ","),
	})
}

func setLoadBalancerName(instance *v1alpha1.RpaasInstance, lbName string) {
	if lbName == "" {
		return
	}
	lbNameLabelKey := config.Get().LoadBalancerNameLabelKey
	if lbNameLabelKey == "" {
		return
	}
	if instance.Spec.Service == nil {
		instance.Spec.Service = &nginxv1alpha1.NginxService{}
	}
	if instance.Spec.Service.Annotations == nil {
		instance.Spec.Service.Annotations = make(map[string]string)
	}
	instance.Spec.Service.Annotations[lbNameLabelKey] = lbName
}

func (m *k8sRpaasManager) GetInstanceInfo(ctx context.Context, instanceName string) (*clientTypes.InstanceInfo, error) {
	instance, err := m.GetInstance(ctx, instanceName)
	if err != nil {
		return nil, err
	}

	info := &clientTypes.InstanceInfo{
		Name:        instance.Name,
		Service:     instance.Labels[labelKey("service-name")],
		Cluster:     m.clusterName,
		Description: instance.Annotations[labelKey("description")],
		Team:        instance.Annotations[labelKey("team-owner")],
		Tags:        strings.Split(instance.Annotations[labelKey("tags")], ","),
		Replicas:    instance.Spec.Replicas,
		Plan:        instance.Spec.PlanName,
		Binds:       instance.Spec.Binds,
		Flavors:     instance.Spec.Flavors,
	}

	autoscale := instance.Spec.Autoscale
	if autoscale != nil {
		info.Autoscale = &clientTypes.Autoscale{
			MinReplicas: autoscale.MinReplicas,
			MaxReplicas: &autoscale.MaxReplicas,
			CPU:         autoscale.TargetCPUUtilizationPercentage,
			Memory:      autoscale.TargetMemoryUtilizationPercentage,
		}
	}

	routes, err := m.GetRoutes(ctx, instanceName)
	if err != nil {
		return nil, err
	}

	for _, r := range routes {
		info.Routes = append(info.Routes, clientTypes.Route{
			Path:        r.Path,
			Destination: r.Destination,
			Content:     r.Content,
			HTTPSOnly:   r.HTTPSOnly,
		})
	}

	blocks, err := m.ListBlocks(ctx, instanceName)
	if err != nil {
		return nil, err
	}

	for _, block := range blocks {
		info.Blocks = append(info.Blocks, clientTypes.Block{
			Name:    block.Name,
			Content: block.Content,
		})
	}

	info.Certificates, err = m.getCertificatesInfo(ctx, instance)
	if err != nil {
		return nil, err
	}

	dashboardTemplate := config.Get().DashboardTemplate
	if dashboardTemplate != "" {
		tpl, tplErr := template.New("dashboard").Parse(dashboardTemplate)
		if tplErr != nil {
			return nil, errors.Wrap(tplErr, "could not parse dashboard template")
		}

		var buf bytes.Buffer
		tplErr = tpl.Execute(&buf, info)

		if tplErr != nil {
			return nil, errors.Wrap(tplErr, "could not execute dashboard template")
		}
		info.Dashboard = strings.TrimSpace(buf.String())
	}

	nginx, err := m.getNginx(ctx, instance)
	if err != nil && k8sErrors.IsNotFound(err) {
		return info, nil
	}

	if err != nil {
		return nil, err
	}

	info.Addresses, err = m.getInstanceAddresses(ctx, nginx)
	if err != nil {
		return nil, err
	}

	info.Pods, err = m.getPodStatuses(ctx, nginx)
	if err != nil {
		return nil, err
	}

	return info, nil
}

func (m *k8sRpaasManager) getNginx(ctx context.Context, instance *v1alpha1.RpaasInstance) (*nginxv1alpha1.Nginx, error) {
	if instance == nil {
		return nil, fmt.Errorf("rpaasinstance cannot be nil")
	}

	var nginx nginxv1alpha1.Nginx
	err := m.cli.Get(ctx, types.NamespacedName{Name: instance.Name, Namespace: instance.Namespace}, &nginx)
	if err != nil {
		return nil, err
	}

	return &nginx, nil
}

func (m *k8sRpaasManager) getPods(ctx context.Context, nginx *nginxv1alpha1.Nginx) ([]corev1.Pod, error) {
	if nginx == nil {
		return nil, fmt.Errorf("nginx resource cannot be nil")
	}

	if nginx.Status.PodSelector == "" {
		return nil, fmt.Errorf("pod selector on nginx status cannot be empty")
	}

	labelSet, err := labels.ConvertSelectorToLabelsMap(nginx.Status.PodSelector)
	if err != nil {
		return nil, err
	}

	var podList corev1.PodList
	err = m.cli.List(ctx, &podList, &client.ListOptions{LabelSelector: labelSet.AsSelector(), Namespace: nginx.Namespace})
	if err != nil {
		return nil, err
	}

	return podList.Items, nil
}

func (m *k8sRpaasManager) getPodMetrics(ctx context.Context, nginx *nginxv1alpha1.Nginx) (map[string]*clientTypes.PodMetrics, error) {
	if nginx == nil {
		return nil, fmt.Errorf("nginx resource cannot be nil")
	}

	if nginx.Status.PodSelector == "" {
		return nil, fmt.Errorf("pod selector on nginx status cannot be empty")
	}

	labelSet, err := labels.ConvertSelectorToLabelsMap(nginx.Status.PodSelector)
	if err != nil {
		return nil, err
	}

	var metricsList metricsv1beta1.PodMetricsList
	err = m.cli.List(ctx, &metricsList, &client.ListOptions{LabelSelector: labelSet.AsSelector(), Namespace: nginx.Namespace})
	if err != nil {
		return nil, err
	}

	result := map[string]*clientTypes.PodMetrics{}
	for _, podMetric := range metricsList.Items {
		totalCPUUsage := resource.NewQuantity(0, resource.DecimalSI)
		totalMemoryUsage := resource.NewQuantity(0, resource.BinarySI)

		for _, container := range podMetric.Containers {
			cpuUsage, ok := container.Usage["cpu"]
			if !ok {
				continue
			}
			totalCPUUsage.Add(cpuUsage)
		}

		for _, container := range podMetric.Containers {
			memoryUsage, ok := container.Usage["memory"]
			if !ok {
				continue
			}
			totalMemoryUsage.Add(memoryUsage)
		}

		result[podMetric.ObjectMeta.Name] = &clientTypes.PodMetrics{
			CPU:    totalCPUUsage.String(),
			Memory: totalMemoryUsage.String(),
		}
	}

	return result, nil
}

func (m *k8sRpaasManager) getServices(ctx context.Context, nginx *nginxv1alpha1.Nginx) ([]corev1.Service, error) {
	if nginx == nil {
		return nil, fmt.Errorf("nginx cannot be nil")
	}

	var services []corev1.Service
	for _, svcStatus := range nginx.Status.Services {
		var svc corev1.Service
		err := m.cli.Get(ctx, types.NamespacedName{Name: svcStatus.Name, Namespace: nginx.Namespace}, &svc)
		if err != nil {
			return nil, err
		}

		services = append(services, svc)
	}

	return services, nil
}

func (m *k8sRpaasManager) getInstanceAddresses(ctx context.Context, nginx *nginxv1alpha1.Nginx) ([]clientTypes.InstanceAddress, error) {
	if nginx == nil {
		return nil, fmt.Errorf("nginx cannot be nil")
	}

	services, err := m.getServices(ctx, nginx)
	if err != nil {
		return nil, err
	}

	var addresses []clientTypes.InstanceAddress
	for _, svc := range services {
		switch svc.Spec.Type {
		case corev1.ServiceTypeLoadBalancer:
			lbAddresses, err := m.loadBalancerInstanceAddresses(ctx, &svc)
			if err != nil {
				return nil, err
			}
			addresses = append(addresses, lbAddresses...)
		default:
			addresses = append(addresses, clientTypes.InstanceAddress{
				ServiceName: svc.ObjectMeta.Name,
				IP:          svc.Spec.ClusterIP,
			})
		}
	}

	sort.SliceStable(addresses, func(i, j int) bool {
		if addresses[i].IP != addresses[j].IP {
			return addresses[i].IP < addresses[j].IP
		}

		return addresses[i].Hostname < addresses[j].Hostname
	})

	return addresses, nil
}

func (m *k8sRpaasManager) loadBalancerInstanceAddresses(ctx context.Context, svc *v1.Service) ([]clientTypes.InstanceAddress, error) {
	var addresses []clientTypes.InstanceAddress

	if isLoadBalancerReady(svc) {
		status := "ready"
		for _, lbIngress := range svc.Status.LoadBalancer.Ingress {
			hostname := lbIngress.Hostname
			if vhost, ok := svc.Annotations[externalDNSHostnameLabel]; ok {
				hostname = vhost
			}
			addresses = append(addresses, clientTypes.InstanceAddress{
				ServiceName: svc.ObjectMeta.Name,
				IP:          lbIngress.IP,
				Hostname:    hostname,
				Status:      status,
			})
		}
	} else {
		serviceKind := "Service"
		events, err := m.eventsForObject(ctx, svc.ObjectMeta.Namespace, serviceKind, svc.ObjectMeta.UID)
		if err != nil {
			return nil, err
		}
		addresses = append(addresses, clientTypes.InstanceAddress{
			ServiceName: svc.ObjectMeta.Name,
			Status:      "pending: " + formatEventsToString(events),
		})
	}

	return addresses, nil
}

func formatEventsToString(events []v1.Event) string {
	var buf bytes.Buffer
	reasonMap := map[string]bool{}

	for _, event := range events {
		if reasonMap[event.Reason] {
			continue
		}
		reasonMap[event.Reason] = true

		fmt.Fprintf(&buf, "%s - %s - %s\n", event.LastTimestamp.UTC().Format(time.RFC3339), event.Type, event.Message)
	}

	return buf.String()
}

func isLoadBalancerReady(service *v1.Service) bool {
	if len(service.Status.LoadBalancer.Ingress) == 0 {
		return false
	}
	// NOTE: aws load-balancers does not have IP
	return service.Status.LoadBalancer.Ingress[0].IP != "" || service.Status.LoadBalancer.Ingress[0].Hostname != ""
}

func (m *k8sRpaasManager) getPodStatuses(ctx context.Context, nginx *nginxv1alpha1.Nginx) ([]clientTypes.Pod, error) {
	pods, err := m.getPods(ctx, nginx)
	if err != nil {
		return nil, err
	}

	podMetrics, err := m.getPodMetrics(ctx, nginx)
	if err != nil {
		podMetrics = map[string]*clientTypes.PodMetrics{}
		if m.clusterName == "" {
			logrus.Errorf("[local cluster] Failed to fetch pod metrics: %s", err.Error())
		} else {
			logrus.Errorf("[cluster %s] Failed to fetch pod metrics: %s", m.clusterName, err.Error())
		}
	}

	var podStatuses []clientTypes.Pod
	for _, pod := range pods {
		ps, err := m.newPodStatus(ctx, &pod)
		if err != nil {
			return nil, err
		}
		ps.Metrics = podMetrics[pod.ObjectMeta.Name]
		podStatuses = append(podStatuses, ps)
	}

	sort.Slice(podStatuses, func(i, j int) bool {
		return podStatuses[i].Name < podStatuses[j].Name
	})

	return podStatuses, nil
}

func (m *k8sRpaasManager) newPodStatus(ctx context.Context, pod *corev1.Pod) (clientTypes.Pod, error) {
	phase := pod.Status.Phase
	if phase == "" {
		phase = corev1.PodUnknown
	}

	errors, err := m.getErrorsForPod(ctx, pod)
	if err != nil {
		return clientTypes.Pod{}, err
	}

	if len(errors) > 0 {
		phase = "Errored"
	}

	var restarts int32
	var ready bool
	for _, cs := range pod.Status.ContainerStatuses {
		if cs.Name != nginxContainerName {
			continue
		}
		restarts, ready = cs.RestartCount, cs.Ready
		break
	}

	return clientTypes.Pod{
		CreatedAt: pod.CreationTimestamp.Time.In(time.UTC),
		Name:      pod.Name,
		IP:        pod.Status.PodIP,
		HostIP:    pod.Status.HostIP,
		Status:    string(phase),
		Ports:     getPortsForPod(pod),
		Errors:    errors,
		Restarts:  restarts,
		Ready:     ready,
	}, nil
}

func (m *k8sRpaasManager) getCertificatesInfo(ctx context.Context, instance *v1alpha1.RpaasInstance) ([]clientTypes.CertificateInfo, error) {
	certs, err := m.GetCertificates(ctx, instance.Name)
	if err != nil {
		return nil, err
	}

	var certsInfo []clientTypes.CertificateInfo
	for _, cert := range certs {
		c, err := tls.X509KeyPair([]byte(cert.Certificate), []byte(cert.Key))
		if err != nil {
			return nil, err
		}

		leaf, err := x509.ParseCertificate(c.Certificate[0])
		if err != nil {
			return nil, err
		}

		certsInfo = append(certsInfo, clientTypes.CertificateInfo{
			Name:               cert.Name,
			DNSNames:           leaf.DNSNames,
			ValidFrom:          leaf.NotBefore,
			ValidUntil:         leaf.NotAfter,
			PublicKeyAlgorithm: leaf.PublicKeyAlgorithm.String(),
			PublicKeyBitSize:   publicKeySize(leaf.PublicKey),
		})
	}

	return certsInfo, nil
}

func getPortsForPod(pod *corev1.Pod) []clientTypes.PodPort {
	var ports []clientTypes.PodPort
	for _, container := range pod.Spec.Containers {
		if container.Name != nginxContainerName {
			continue
		}

		for _, port := range container.Ports {
			ports = append(ports, clientTypes.PodPort(port))
		}

		sort.Slice(ports, func(i, j int) bool {
			return ports[i].Name < ports[j].Name
		})

		break
	}
	return ports
}

func (m *k8sRpaasManager) getErrorsForPod(ctx context.Context, pod *corev1.Pod) ([]clientTypes.PodError, error) {
	for _, cs := range pod.Status.ContainerStatuses {
		if cs.Name == nginxContainerName && cs.State.Running != nil {
			return nil, nil
		}
	}

	events, err := m.eventsForPod(ctx, pod)
	if err != nil {
		return nil, err
	}

	var errors []clientTypes.PodError
	for _, event := range events {
		if event.Type == corev1.EventTypeNormal {
			continue
		}

		errors = append(errors, clientTypes.PodError{
			First:   event.FirstTimestamp.Time.In(time.UTC),
			Last:    event.LastTimestamp.Time.In(time.UTC),
			Count:   event.Count,
			Message: event.Message,
		})
	}

	sort.SliceStable(errors, func(i, j int) bool {
		// NOTE: descending order by date.
		return errors[i].Last.After(errors[j].Last)
	})

	return errors, nil
}

func (m *k8sRpaasManager) patchInstance(ctx context.Context, originalInstance *v1alpha1.RpaasInstance, updatedInstance *v1alpha1.RpaasInstance) error {
	updatedInstance.Spec.RolloutNginxOnce = true

	originalData, err := json.Marshal(originalInstance)
	if err != nil {
		return err
	}
	updatedData, err := json.Marshal(updatedInstance)
	if err != nil {
		return err
	}
	data, err := jsonpatch.CreateMergePatch(originalData, updatedData)

	if err != nil {
		return err
	}

	return m.cli.Patch(ctx, originalInstance, client.RawPatch(types.MergePatchType, data))
}

func buildServiceInstanceParametersForPlan(flavors []Flavor) interface{} {
	planParameters := map[string]interface{}{
		"flavors": map[string]interface{}{
			"type": "array",
			"items": map[string]interface{}{
				"$ref": "#/definitions/flavor",
			},
			"description": formatFlavorsDescription(flavors),
			"enum":        flavorNames(flavors),
		},
		"ip": map[string]interface{}{
			"type":        "string",
			"description": "IP address that will be assigned to load balancer. Example: ip=192.168.15.10.\n",
		},
		"plan-override": map[string]interface{}{
			"type":        "object",
			"description": "Allows an instance to change its plan parameters to specific ones. Examples: plan-override={\"config\": {\"cacheEnabled\": false}}; plan-override={\"image\": \"tsuru/nginx:latest\"}.\n",
		},
	}

	if config.Get().LoadBalancerNameLabelKey != "" {
		planParameters["lb-name"] = map[string]interface{}{
			"type":        "string",
			"description": "Custom domain address (e.g. following RFC 1035) assigned to instance's load balancer. Example: lb-name=my-instance.internal.subdomain.example.\n",
		}
	}

	return map[string]interface{}{
		"$id":        "https://example.com/schema.json",
		"$schema":    "https://json-schema.org/draft-07/schema#",
		"type":       "object",
		"properties": planParameters,
		"definitions": map[string]interface{}{
			"flavor": map[string]interface{}{
				"type": "string",
			},
		},
	}
}

func formatFlavorsDescription(flavors []Flavor) string {
	var sb strings.Builder
	sb.WriteString("Provides a self-contained set of features that can be enabled on this plan. Example: flavors=flavor-a,flavor-b.\n")

	if len(flavors) == 0 {
		return sb.String()
	}

	sb.WriteString("  supported flavors:")
	for _, f := range flavors {
		sb.WriteString("\n")
		sb.WriteString(fmt.Sprintf("    - name: %s\n", f.Name))
		sb.WriteString(fmt.Sprintf("      description: %s", f.Description))
	}

	sb.WriteString("\n")
	return sb.String()
}

func flavorNames(flavors []Flavor) (names []string) {
	for _, f := range flavors {
		names = append(names, f.Name)
	}

	return
}

func certificateName(name string) string {
	if name == "" {
		return v1alpha1.CertificateNameDefault
	}

	return strings.ToLower(strings.TrimLeft(name, `*.`))
}

func publicKeySize(publicKey interface{}) (keySize int) {
	switch pk := publicKey.(type) {
	case *rsa.PublicKey:
		keySize = pk.Size() * 8 // convert bytes to bits
	case *ecdsa.PublicKey:
		keySize = pk.Params().BitSize
	}
	return
}

func (m *k8sRpaasManager) AddAllowedUpstream(ctx context.Context, instanceName string, upstream v1alpha1.RpaasAllowedUpstream) error {
	isCreation := false
	instance, err := m.getAllowedUpstreams(ctx, instanceName)
	if err != nil {
		if k8sErrors.IsNotFound(err) {
			instance = &v1alpha1.RpaasAllowedUpstreams{
				ObjectMeta: metav1.ObjectMeta{
					Name:      instanceName,
					Namespace: namespaceName(),
				},
			}
			isCreation = true
		} else {
			return err
		}
	}

	instance.Spec.Upstreams = append(instance.Spec.Upstreams, upstream)
	if isCreation {
		return m.cli.Create(ctx, instance)
	}

	return m.cli.Update(ctx, instance)
}

func (m *k8sRpaasManager) getAllowedUpstreams(ctx context.Context, name string) (*v1alpha1.RpaasAllowedUpstreams, error) {
	var instance v1alpha1.RpaasAllowedUpstreams
	err := m.cli.Get(ctx, types.NamespacedName{Name: name, Namespace: namespaceName()}, &instance)
	if err != nil {
		return nil, err
	}

	return &instance, nil
}
