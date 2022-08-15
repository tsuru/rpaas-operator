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
	"errors"
	"fmt"
	"net"
	"net/url"
	"regexp"
	"sort"
	"strings"
	"text/template"
	"time"

	"github.com/cert-manager/cert-manager/pkg/util/pki"
	jsonpatch "github.com/evanphx/json-patch/v5"
	"github.com/hashicorp/go-multierror"
	"github.com/sirupsen/logrus"
	nginxv1alpha1 "github.com/tsuru/nginx-operator/api/v1alpha1"
	nginxk8s "github.com/tsuru/nginx-operator/pkg/k8s"
	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	k8sErrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/labels"
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
	"github.com/tsuru/rpaas-operator/internal/controllers/certificates"
	nginxManager "github.com/tsuru/rpaas-operator/internal/pkg/rpaas/nginx"
	clientTypes "github.com/tsuru/rpaas-operator/pkg/rpaas/client/types"
	"github.com/tsuru/rpaas-operator/pkg/util"
)

const (
	defaultNamespace      = "rpaasv2"
	defaultKeyLabelPrefix = "rpaas.extensions.tsuru.io"

	externalDNSHostnameLabel  = "external-dns.alpha.kubernetes.io/hostname"
	allowedDNSZonesAnnotation = "rpaas.extensions.tsuru.io/allowed-dns-zones"

	nginxContainerName = "nginx"
)

var _ RpaasManager = &k8sRpaasManager{}

var podAllowedReasonsToFail = map[string]bool{
	"shutdown":     true,
	"evicted":      true,
	"nodeaffinity": true,
	"terminated":   true,
}

type k8sRpaasManager struct {
	cli          client.Client
	cacheManager CacheManager
	restConfig   *rest.Config
	kcs          kubernetes.Interface
	clusterName  string
	poolName     string
}

func NewK8S(cfg *rest.Config, k8sClient client.Client, clusterName string, poolName string) (RpaasManager, error) {
	m := &k8sRpaasManager{
		cli:          k8sClient,
		cacheManager: nginxManager.NewNginxManager(),
		restConfig:   cfg,
		clusterName:  clusterName,
		poolName:     poolName,
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
	upgradeRoundTripper := spdy.NewRoundTripper(tlsConfig)
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

	replicas := func(n int32) *int32 { return &n }(1)
	if r := config.Get().NewInstanceReplicas; r > 0 {
		replicas = func(n int32) *int32 { return &n }(int32(r))
	}

	instance := newRpaasInstance(args.Name)
	instance.Namespace = nsName
	instance.Spec = v1alpha1.RpaasInstanceSpec{
		Replicas: replicas,
		PlanName: plan.Name,
		Flavors:  args.Flavors(),
		Service: &nginxv1alpha1.NginxService{
			Annotations: instance.Annotations,
			Labels:      instance.Labels,
		},
		PodTemplate: nginxv1alpha1.NginxPodTemplateSpec{
			Affinity:    getAffinity(args.Team),
			Annotations: instance.Annotations,
			Labels:      instance.Labels,
		},
	}

	if config.Get().NamespacedInstances {
		instance.Spec.PlanNamespace = getServiceName()
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
	instance, err := m.GetInstance(ctx, instanceName)
	if err != nil {
		return err
	}

	if err := m.validateUpdateInstanceArgs(ctx, instance, args); err != nil {
		return err
	}

	originalInstance := instance.DeepCopy()

	if args.Plan != "" {
		instance.Spec.PlanName = args.Plan
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
	var nsName string
	poolNamespace, err := m.poolNamespace()
	if err != nil {
		return "", err
	}
	if poolNamespace != "" {
		nsName = poolNamespace
	} else {
		nsName = getServiceName()
	}

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

	err = validateBlock(block)
	if err != nil {
		return err
	}

	if instance.Spec.Blocks == nil {
		instance.Spec.Blocks = make(map[v1alpha1.BlockType]v1alpha1.Value)
	}

	blockType := v1alpha1.BlockType(block.Name)
	instance.Spec.Blocks[blockType] = v1alpha1.Value{Value: block.Content}

	return m.patchInstance(ctx, originalInstance, instance)
}

func (m *k8sRpaasManager) Scale(ctx context.Context, instanceName string, replicas int32) error {
	instance, err := m.GetInstance(ctx, instanceName)
	if err != nil {
		return err
	}

	if instance.Spec.Autoscale != nil {
		return ValidationError{Msg: "cannot scale manual with autoscaler configured, please update autoscale settings"}
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

	var certList []CertificateData
	for _, tls := range instance.Spec.TLS {
		var s corev1.Secret
		if err = m.cli.Get(ctx, types.NamespacedName{Name: tls.SecretName, Namespace: instance.Namespace}, &s); err != nil {
			return nil, err
		}

		certList = append(certList, CertificateData{
			Name:        s.Labels[certificates.CertificateNameLabel],
			Certificate: string(s.Data[corev1.TLSCertKey]),
			Key:         string(s.Data[corev1.TLSPrivateKeyKey]),
		})
	}

	return certList, nil
}

func (m *k8sRpaasManager) DeleteCertificate(ctx context.Context, instanceName, name string) error {
	instance, err := m.GetInstance(ctx, instanceName)
	if err != nil {
		return err
	}

	if len(instance.Spec.TLS) == 0 {
		return &NotFoundError{Msg: fmt.Sprintf("no certificate bound to instance %q", instanceName)}
	}

	name = certificateName(name)

	err = certificates.DeleteCertificate(ctx, m.cli, instance, name)
	if err != nil && err == fmt.Errorf("certificate %q does not exist", name) {
		return &NotFoundError{Msg: err.Error()}
	}

	return err
}

func (m *k8sRpaasManager) UpdateCertificate(ctx context.Context, instanceName, name string, c tls.Certificate) error {
	instance, err := m.GetInstance(ctx, instanceName)
	if err != nil {
		return err
	}

	name = certificateName(name)
	if errs := validation.IsConfigMapKey(name); len(errs) > 0 {
		return ValidationError{Msg: fmt.Sprintf("certificate name is not valid: %s", strings.Join(errs, ": "))}
	}

	if strings.HasPrefix(name, certificates.CertManagerCertificateName) {
		return &ValidationError{Msg: fmt.Sprintf("certificate name is forbidden: name should not begin with %q", certificates.CertManagerCertificateName)}
	}

	rawCertificate, rawKey, err := getRawCertificateAndKey(c)
	if err != nil {
		return err
	}

	certsInfo, err := m.getCertificatesInfo(ctx, instance)
	if err != nil {
		return err
	}

	leaf, err := x509.ParseCertificate(c.Certificate[0])
	if err != nil {
		return err
	}

	for _, ci := range certsInfo {
		if ci.Name == name {
			continue
		}

		if hasIntersection(ci.DNSNames, leaf.DNSNames) {
			return &ValidationError{Msg: fmt.Sprintf("certificate DNS name is forbidden: you cannot use a already used dns name, currently in use use in %q certificate", ci.Name)}
		}
	}

	return certificates.UpdateCertificate(ctx, m.cli, instance, name, rawCertificate, rawKey)
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

	poolNamespace, err := m.poolNamespace()
	if err != nil {
		return nil, err
	}
	if poolNamespace != "" {
		err = m.cli.Get(ctx, types.NamespacedName{Name: name, Namespace: poolNamespace}, &instance)
		if err != nil && !k8sErrors.IsNotFound(err) {
			return nil, err
		}

		if err == nil {
			return &instance, nil
		}
	}
	err = m.cli.Get(ctx, types.NamespacedName{Name: name, Namespace: getServiceName()}, &instance)
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
	if err := m.cli.List(ctx, &planList, client.InNamespace(getServiceName())); err != nil {
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

func (m *k8sRpaasManager) poolNamespace() (string, error) {
	if config.Get().NamespacedInstances {
		if m.poolName == "" {
			return "", ErrNoPoolDefined
		}
		return fmt.Sprintf("%s-%s", getServiceName(), m.poolName), nil
	}

	return "", nil
}

func (m *k8sRpaasManager) getFlavors(ctx context.Context) ([]v1alpha1.RpaasFlavor, error) {
	flavorList := &v1alpha1.RpaasFlavorList{}
	if err := m.cli.List(ctx, flavorList, client.InNamespace(getServiceName())); err != nil {
		return nil, err
	}

	return flavorList.Items, nil
}

func (m *k8sRpaasManager) selectFlavor(ctx context.Context, flavors []v1alpha1.RpaasFlavor, name string) *v1alpha1.RpaasFlavor {
	for i := range flavors {
		if flavors[i].Name == name {
			return &flavors[i]
		}
	}

	return nil
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
	status := false
	for _, podStatus := range podMap {
		if !podStatus.Running {
			continue
		}
		status, err = m.cacheManager.PurgeCache(podStatus.Address, args.Path, port, args.PreservePath, args.ExtraHeaders)
		if err != nil {
			purgeErrors = multierror.Append(purgeErrors, fmt.Errorf("pod %s failed: %w", podStatus.Address, err))
			continue
		}
		if status {
			purgeCount++
		}
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

func validateContent(content string) error {
	denyPatterns := config.Get().ConfigDenyPatterns
	for _, re := range denyPatterns {
		if re.MatchString(content) {
			return &ValidationError{Msg: fmt.Sprintf("content contains the forbidden pattern %q", re.String())}
		}
	}
	return nil
}

func validateBlock(block ConfigurationBlock) error {
	blockType := v1alpha1.BlockType(block.Name)
	if !isBlockTypeAllowed(blockType) {
		return ValidationError{Msg: fmt.Sprintf("block %q is not allowed", block.Name)}
	}
	if block.Content == "" {
		return &ValidationError{Msg: "content is required"}
	}
	err := validateContent(block.Content)
	if err != nil {
		return err
	}
	return nil
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

	if r.Content != "" {
		err := validateContent(r.Content)
		if err != nil {
			return err
		}
	}

	return nil
}

func (m *k8sRpaasManager) getPlan(ctx context.Context, name string) (*v1alpha1.RpaasPlan, error) {
	if name == "" {
		return m.getDefaultPlan(ctx)
	}

	planName := types.NamespacedName{
		Name:      name,
		Namespace: getServiceName(),
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

	if err = m.validateFlavors(ctx, nil, args.Flavors()); err != nil {
		return err
	}

	return nil
}

func (m *k8sRpaasManager) validateUpdateInstanceArgs(ctx context.Context, instance *v1alpha1.RpaasInstance, args UpdateInstanceArgs) error {
	if err := m.validatePlan(ctx, args.Plan); err != nil {
		return err
	}

	if err := m.validateFlavors(ctx, instance, args.Flavors()); err != nil {
		return err
	}

	return nil
}

func (m *k8sRpaasManager) validatePlan(ctx context.Context, updatedPlan string) error {
	_, err := m.getPlan(ctx, updatedPlan)
	if err != nil && IsNotFoundError(err) {
		return &ValidationError{Msg: "invalid plan", Internal: err}
	}

	return err
}

func (m *k8sRpaasManager) validateFlavors(ctx context.Context, instance *v1alpha1.RpaasInstance, flavors []string) error {
	encountered := map[string]struct{}{}
	for _, f := range flavors {
		if _, duplicated := encountered[f]; duplicated {
			return &ValidationError{Msg: fmt.Sprintf("flavor %q only can be set once", f)}
		}
		encountered[f] = struct{}{}
	}

	var existingFlavors []string
	if instance != nil {
		existingFlavors = instance.Spec.Flavors
	}

	allFlavors, err := m.getFlavors(ctx)
	if err != nil {
		return err
	}

	added, removed := diffFlavors(existingFlavors, flavors)

	for _, f := range added {
		flavorObj := m.selectFlavor(ctx, allFlavors, f)
		if flavorObj == nil {
			return &ValidationError{Msg: fmt.Sprintf("flavor %q not found", f)}
		}

		if flavorObj.Spec.CreationOnly {
			return &ValidationError{Msg: fmt.Sprintf("flavor %q can used only in the creation of instance", f)}
		}
	}

	for _, f := range removed {
		flavorObj := m.selectFlavor(ctx, allFlavors, f)
		if flavorObj == nil {
			continue
		}

		if flavorObj.Spec.CreationOnly {
			return &ValidationError{Msg: fmt.Sprintf("cannot unset flavor %q as it is a creation only flavor", f)}
		}
	}

	return nil
}

func diffFlavors(existing, updated []string) (added, removed []string) {
	for _, f := range updated {
		if !contains(existing, f) {
			added = append(added, f)
		}
	}

	for _, f := range existing {
		if !contains(updated, f) {
			removed = append(removed, f)
		}
	}

	return
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
			Namespace: getServiceName(),
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
		Name:         instance.Name,
		Service:      instance.Labels[labelKey("service-name")],
		Cluster:      m.clusterName,
		Pool:         m.poolName,
		Description:  instance.Annotations[labelKey("description")],
		Team:         instance.Annotations[labelKey("team-owner")],
		Tags:         strings.Split(instance.Annotations[labelKey("tags")], ","),
		Replicas:     instance.Spec.Replicas,
		Plan:         instance.Spec.PlanName,
		Binds:        instance.Spec.Binds,
		Flavors:      instance.Spec.Flavors,
		PlanOverride: instance.Spec.PlanTemplate,
	}

	var acls []clientTypes.AllowedUpstream
	for _, u := range instance.Spec.AllowedUpstreams {
		acls = append(acls, clientTypes.AllowedUpstream{Host: u.Host, Port: u.Port})
	}
	info.ACLs = acls

	autoscale := instance.Spec.Autoscale
	if autoscale != nil {
		info.Autoscale = &clientTypes.Autoscale{
			MinReplicas: autoscale.MinReplicas,
			MaxReplicas: &autoscale.MaxReplicas,
			CPU:         autoscale.TargetCPUUtilizationPercentage,
			Memory:      autoscale.TargetMemoryUtilizationPercentage,
		}
	}

	extraFiles, err := m.GetExtraFiles(ctx, instanceName)
	if err != nil {
		return nil, err
	}

	for _, f := range extraFiles {
		info.ExtraFiles = append(info.ExtraFiles, clientTypes.RpaasFile{
			Name:    f.Name,
			Content: f.Content,
		})
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
			return nil, fmt.Errorf("could not parse dashboard template: %w", tplErr)
		}

		var buf bytes.Buffer
		tplErr = tpl.Execute(&buf, info)
		if tplErr != nil {
			return nil, fmt.Errorf("could not execute dashboard template: %w", tplErr)
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

	info.Events, err = m.getEvents(ctx, nginx)
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

	ps := nginx.Status.PodSelector
	if ps == "" {
		ps = labels.Set(nginxk8s.LabelsForNginx(nginx.Name)).String()
	}

	labelSet, err := labels.ConvertSelectorToLabelsMap(ps)
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

	ps := nginx.Status.PodSelector
	if ps == "" {
		ps = labels.Set(nginxk8s.LabelsForNginx(nginx.Name)).String()
	}

	labelSet, err := labels.ConvertSelectorToLabelsMap(ps)
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
			if container.Name != nginxContainerName {
				continue
			}

			if cpuUsage, ok := container.Usage["cpu"]; ok {
				totalCPUUsage.Add(cpuUsage)
			}

			if memoryUsage, ok := container.Usage["memory"]; ok {
				totalMemoryUsage.Add(memoryUsage)
			}
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

func (m *k8sRpaasManager) getIngresses(ctx context.Context, nginx *nginxv1alpha1.Nginx) ([]networkingv1.Ingress, error) {
	if nginx == nil {
		return nil, fmt.Errorf("nginx cannot be nil")
	}

	var ingresses []networkingv1.Ingress
	for _, si := range nginx.Status.Ingresses {
		var ing networkingv1.Ingress
		err := m.cli.Get(ctx, types.NamespacedName{Name: si.Name, Namespace: nginx.Namespace}, &ing)
		if err != nil {
			return nil, err
		}

		ingresses = append(ingresses, ing)
	}

	return ingresses, nil
}

func (m *k8sRpaasManager) getInstanceAddresses(ctx context.Context, nginx *nginxv1alpha1.Nginx) ([]clientTypes.InstanceAddress, error) {
	if nginx == nil {
		return nil, fmt.Errorf("nginx cannot be nil")
	}

	services, err := m.getServices(ctx, nginx)
	if err != nil {
		return nil, err
	}

	var externalAddresses []clientTypes.InstanceAddress
	var internalAddresses []clientTypes.InstanceAddress
	for _, svc := range services {
		if svc.Spec.Type == corev1.ServiceTypeLoadBalancer {
			var lbAddresses []clientTypes.InstanceAddress
			lbAddresses, err = m.loadBalancerInstanceAddresses(ctx, &svc)
			if err != nil {
				return nil, err
			}
			externalAddresses = append(externalAddresses, lbAddresses...)
		}

		if svc.Spec.ClusterIP != "" {
			internalAddresses = append(internalAddresses, clientTypes.InstanceAddress{
				Type:        clientTypes.InstanceAddressTypeClusterInternal,
				ServiceName: svc.ObjectMeta.Name,
				Hostname:    fmt.Sprintf("%s.%s.svc.cluster.local", svc.ObjectMeta.Name, svc.ObjectMeta.Namespace),
				IP:          svc.Spec.ClusterIP,
				Status:      "ready",
			})
		}
	}

	ingresses, err := m.getIngresses(ctx, nginx)
	if err != nil {
		return nil, err
	}

	for _, ing := range ingresses {
		addrs, err := m.ingressAddresses(ctx, &ing)
		if err != nil {
			return nil, err
		}

		externalAddresses = append(externalAddresses, addrs...)
	}

	sortAddresses(externalAddresses)
	sortAddresses(internalAddresses)

	var addresses []clientTypes.InstanceAddress
	addresses = append(addresses, externalAddresses...)
	addresses = append(addresses, internalAddresses...)

	return addresses, nil
}

func sortAddresses(addresses []clientTypes.InstanceAddress) {
	sort.SliceStable(addresses, func(i, j int) bool {
		if addresses[i].IP != addresses[j].IP {
			return addresses[i].IP < addresses[j].IP
		}

		return addresses[i].Hostname < addresses[j].Hostname
	})
}

func (m *k8sRpaasManager) loadBalancerInstanceAddresses(ctx context.Context, svc *v1.Service) ([]clientTypes.InstanceAddress, error) {
	var addresses []clientTypes.InstanceAddress

	if isLoadBalancerReady(svc.Status.LoadBalancer.Ingress) {
		status := "ready"
		for _, lbIngress := range svc.Status.LoadBalancer.Ingress {
			hostname := lbIngress.Hostname
			if vhost, ok := svc.Annotations[externalDNSHostnameLabel]; ok {
				if hostname != "" {
					hostname += ","
				}

				hostname += vhost
			}

			addresses = append(addresses, clientTypes.InstanceAddress{
				Type:        clientTypes.InstanceAddressTypeClusterExternal,
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
			Type:        clientTypes.InstanceAddressTypeClusterExternal,
			ServiceName: svc.ObjectMeta.Name,
			Status:      "pending: " + formatEventsToString(events),
		})
	}

	return addresses, nil
}

func (m *k8sRpaasManager) ingressAddresses(ctx context.Context, ing *networkingv1.Ingress) ([]clientTypes.InstanceAddress, error) {
	var addresses []clientTypes.InstanceAddress

	if isLoadBalancerReady(ing.Status.LoadBalancer.Ingress) {
		for _, lbIngress := range ing.Status.LoadBalancer.Ingress {
			hostname := lbIngress.Hostname
			if h, ok := ing.Annotations[externalDNSHostnameLabel]; ok {
				if hostname != "" {
					hostname += ","
				}

				hostname += h
			}

			addresses = append(addresses, clientTypes.InstanceAddress{
				Type:        clientTypes.InstanceAddressTypeClusterExternal,
				IngressName: ing.Name,
				Hostname:    hostname,
				IP:          lbIngress.IP,
				Status:      "ready",
			})
		}
	} else {
		events, err := m.eventsForObject(ctx, ing.Namespace, "Ingress", ing.UID)
		if err != nil {
			return nil, err
		}

		addresses = append(addresses, clientTypes.InstanceAddress{
			Type:        clientTypes.InstanceAddressTypeClusterExternal,
			IngressName: ing.Name,
			Status:      fmt.Sprintf("pending: %s", formatEventsToString(events)),
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

func isLoadBalancerReady(ings []v1.LoadBalancerIngress) bool {
	if len(ings) == 0 {
		return false
	}

	// NOTE: AWS load balancers does not have IP
	return ings[0].IP != "" || ings[0].Hostname != ""
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
		if podIsAllowedToFail(pod) {
			continue
		}
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

	var terminatedAt time.Time
	if d := pod.DeletionTimestamp; d != nil {
		phase = "Terminating"
		terminatedAt = d.In(time.UTC)
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
		CreatedAt:    pod.CreationTimestamp.In(time.UTC),
		TerminatedAt: terminatedAt,
		Name:         pod.Name,
		IP:           pod.Status.PodIP,
		HostIP:       pod.Status.HostIP,
		Status:       string(phase),
		Ports:        getPortsForPod(pod),
		Errors:       errors,
		Restarts:     restarts,
		Ready:        ready,
	}, nil
}

func (m *k8sRpaasManager) getCertificatesInfo(ctx context.Context, instance *v1alpha1.RpaasInstance) ([]clientTypes.CertificateInfo, error) {
	certs, err := m.GetCertificates(ctx, instance.Name)
	if err != nil {
		return nil, err
	}

	var certsInfo []clientTypes.CertificateInfo
	for _, cert := range certs {
		certs, err := pki.DecodeX509CertificateChainBytes([]byte(cert.Certificate))
		if err != nil {
			return nil, err
		}

		if len(certs) == 0 {
			return nil, fmt.Errorf("no certificates found in pem file")
		}

		leaf := certs[0]

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
		return errors[i].Last.Before(errors[j].Last) // ascending order by last event occurrence
	})

	return errors, nil
}

func (m *k8sRpaasManager) getEvents(ctx context.Context, nginx *nginxv1alpha1.Nginx) ([]clientTypes.Event, error) {
	events, err := m.eventsForObject(ctx, nginx.Namespace, "Nginx", nginx.UID)
	if err != nil {
		return nil, err
	}

	if len(events) == 0 {
		return nil, nil
	}

	e := make([]clientTypes.Event, 0, len(events))
	for _, evt := range events {
		e = append(e, clientTypes.Event{
			First:   evt.FirstTimestamp.Time.In(time.UTC),
			Last:    evt.LastTimestamp.Time.In(time.UTC),
			Count:   evt.Count,
			Type:    evt.Type,
			Reason:  evt.Reason,
			Message: evt.Message,
		})
	}

	sort.SliceStable(e, func(i, j int) bool {
		return e[i].Last.Before(e[j].Last) // ascending order by last event occurrence
	})

	return e, nil
}

func (m *k8sRpaasManager) patchInstance(ctx context.Context, originalInstance *v1alpha1.RpaasInstance, updatedInstance *v1alpha1.RpaasInstance) error {
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

func (m *k8sRpaasManager) AddUpstream(ctx context.Context, instanceName string, upstream v1alpha1.AllowedUpstream) error {
	instance, err := m.GetInstance(ctx, instanceName)
	if err != nil {
		return err
	}
	originalInstance := instance.DeepCopy()

	if upstream.Host == "" {
		return &ValidationError{Msg: "cannot add an upstream with empty host"}
	}

	for _, u := range instance.Spec.AllowedUpstreams {
		if u.Host == upstream.Host && u.Port == upstream.Port {
			return &ConflictError{Msg: fmt.Sprintf("upstream already present in instance: %s", instanceName)}
		}
	}
	instance.Spec.AllowedUpstreams = append(instance.Spec.AllowedUpstreams, upstream)

	return m.patchInstance(ctx, originalInstance, instance)
}

func (m *k8sRpaasManager) GetUpstreams(ctx context.Context, instanceName string) ([]v1alpha1.AllowedUpstream, error) {
	instance, err := m.GetInstance(ctx, instanceName)
	if err != nil {
		return nil, err
	}

	return instance.Spec.AllowedUpstreams, nil
}

func (m *k8sRpaasManager) DeleteUpstream(ctx context.Context, instanceName string, upstream v1alpha1.AllowedUpstream) error {
	instance, err := m.GetInstance(ctx, instanceName)
	if err != nil {
		return err
	}
	originalInstance := instance.DeepCopy()

	found := false
	upstreams := instance.Spec.AllowedUpstreams
	for i, u := range upstreams {
		if u.Port == upstream.Port && strings.Compare(u.Host, upstream.Host) == 0 {
			found = true
			upstreams = append(upstreams[:i], upstreams[i+1:]...)
			break
		}
	}
	if !found {
		return &NotFoundError{Msg: fmt.Sprintf("upstream not found inside list of allowed upstreams of %s", instanceName)}
	}

	instance.Spec.AllowedUpstreams = upstreams
	return m.patchInstance(ctx, originalInstance, instance)
}

func hasIntersection(a []string, b []string) bool {
	for _, x := range a {
		for _, y := range b {
			if x == y {
				return true
			}
		}
	}

	return false
}

func podIsAllowedToFail(pod corev1.Pod) bool {
	reason := strings.ToLower(pod.Status.Reason)
	return pod.Status.Phase == corev1.PodFailed && podAllowedReasonsToFail[reason]
}

func contains(ss []string, s string) bool {
	for i := range ss {
		if ss[i] == s {
			return true
		}
	}

	return false
}
