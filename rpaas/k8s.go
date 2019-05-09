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
	"strings"

	nginxv1alpha1 "github.com/tsuru/nginx-operator/pkg/apis/nginx/v1alpha1"
	"github.com/tsuru/rpaas-operator/config"
	"github.com/tsuru/rpaas-operator/pkg/apis/extensions/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	k8sErrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	defaultNamespacePrefix   = "rpaasv2"
	serviceAnnotationsConfig = "service-annotations"
)

var (
	ErrBlockInvalid      = ValidationError{Msg: fmt.Sprintf("rpaas: block is not valid (acceptable values are: %v)", getAvailableBlocks())}
	ErrBlockIsNotDefined = ValidationError{Msg: "rpaas: block is not defined"}
)

type K8SOptions struct {
	Cli client.Client
	Ctx context.Context
}

type k8sRpaasManager struct {
	cli client.Client
	ctx context.Context
}

func NewK8S(o K8SOptions) RpaasManager {
	return &k8sRpaasManager{
		cli: o.Cli,
		ctx: o.Ctx,
	}
}

func (m *k8sRpaasManager) DeleteInstance(name string) error {
	instance, err := m.GetInstance(name)
	if err != nil {
		return err
	}
	return m.cli.Delete(m.ctx, instance)
}

func (m *k8sRpaasManager) CreateInstance(args CreateArgs) error {
	parseTags(args)
	plan, err := m.validateCreate(args)
	if err != nil {
		return err
	}
	_, err = m.GetInstance(args.Name)
	if err == nil {
		return ConflictError{Msg: fmt.Sprintf("rpaas instance named %q already exists", args.Name)}
	}
	if !IsNotFoundError(err) {
		return err
	}
	data, err := json.Marshal(args)
	if err != nil {
		return err
	}
	var annotationsBase map[string]interface{}
	err = json.Unmarshal(data, &annotationsBase)
	if err != nil {
		return err
	}
	annotations := map[string]string{}
	for k, v := range annotationsBase {
		annotations[k] = fmt.Sprint(v)
	}
	namespaceName := NamespaceName(args.Team)
	if err = m.createNamespace(namespaceName); err != nil && !k8sErrors.IsAlreadyExists(err) {
		return err
	}
	instance := &v1alpha1.RpaasInstance{
		TypeMeta: metav1.TypeMeta{
			Kind:       "RpaasInstance",
			APIVersion: "extensions.tsuru.io/v1alpha1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:        args.Name,
			Namespace:   namespaceName,
			Annotations: annotations,
		},
		Spec: v1alpha1.RpaasInstanceSpec{
			PlanName: plan.ObjectMeta.Name,
			Service: &nginxv1alpha1.NginxService{
				Type:           corev1.ServiceTypeLoadBalancer,
				LoadBalancerIP: args.IP,
				Annotations:    config.StringMap(serviceAnnotationsConfig),
			},
		},
	}
	err = m.cli.Create(m.ctx, instance)
	if err != nil {
		if k8sErrors.IsAlreadyExists(err) {
			return ConflictError{Msg: args.Name + " instance already exists"}
		}
		return err
	}
	return nil
}

func (m *k8sRpaasManager) DeleteBlock(instanceName, blockName string) error {
	instance, err := m.GetInstance(instanceName)
	if err != nil {
		return err
	}
	if !isBlockValid(blockName) {
		return ErrBlockInvalid
	}
	if err = m.deleteConfigurationBlocks(*instance, blockName); err != nil {
		return err
	}
	if instance.Spec.Blocks == nil {
		return ErrBlockIsNotDefined
	}
	blockType := v1alpha1.BlockType(blockName)
	if _, ok := instance.Spec.Blocks[blockType]; !ok {
		return ErrBlockIsNotDefined
	}
	delete(instance.Spec.Blocks, blockType)
	return m.cli.Update(m.ctx, instance)
}

func (m *k8sRpaasManager) ListBlocks(instanceName string) ([]ConfigurationBlock, error) {
	instance, err := m.GetInstance(instanceName)
	if err != nil {
		return nil, err
	}
	configBlocks, err := m.getConfigurationBlocks(*instance)
	if err != nil && k8sErrors.IsNotFound(err) {
		return []ConfigurationBlock{}, nil
	}
	if err != nil {
		return nil, err
	}
	var index int
	blocks := make([]ConfigurationBlock, len(configBlocks.Data))
	for name, content := range configBlocks.Data {
		blocks[index] = ConfigurationBlock{Name: name, Content: content}
		index++
	}
	return blocks, nil
}

func (m *k8sRpaasManager) UpdateBlock(instanceName string, block ConfigurationBlock) error {
	instance, err := m.GetInstance(instanceName)
	if err != nil {
		return err
	}
	if !isBlockValid(block.Name) {
		return ErrBlockInvalid
	}
	if err = m.updateConfigurationBlocks(*instance, block.Name, block.Content); err != nil {
		return err
	}
	if instance.Spec.Blocks == nil {
		instance.Spec.Blocks = map[v1alpha1.BlockType]v1alpha1.ConfigRef{}
	}
	blockType := v1alpha1.BlockType(block.Name)
	instance.Spec.Blocks[blockType] = v1alpha1.ConfigRef{
		Name: formatConfigurationBlocksName(*instance),
		Kind: v1alpha1.ConfigKindConfigMap,
	}
	return m.cli.Update(m.ctx, instance)
}

func (m *k8sRpaasManager) UpdateCertificate(instance, name string, c tls.Certificate) error {
	rpaasInstance, err := m.GetInstance(instance)
	if err != nil {
		return err
	}
	if name == "" {
		name = v1alpha1.CertificateNameDefault
	}
	secret, err := m.getCertificateSecret(*rpaasInstance, name)
	if err == nil {
		return m.updateCertificateSecret(secret, &c)
	}
	if !k8sErrors.IsNotFound(err) {
		return err
	}
	secret, err = m.createCertificateSecret(*rpaasInstance, name, &c)
	if err != nil {
		return err
	}
	certs := map[string]nginxv1alpha1.TLSSecret{
		name: *newTLSSecret(secret, name),
	}
	return m.updateCertificates(rpaasInstance, certs)
}

func (m *k8sRpaasManager) GetInstanceAddress(name string) (string, error) {
	rpaasInstance, err := m.GetInstance(name)
	if err != nil {
		return "", err
	}
	nginx := nginxv1alpha1.Nginx{}
	err = m.cli.Get(m.ctx, types.NamespacedName{Name: rpaasInstance.Name, Namespace: rpaasInstance.Namespace}, &nginx)
	if err != nil {
		if IsNotFoundError(err) {
			return "", nil
		}
		return "", err
	}
	if len(nginx.Status.Services) == 0 {
		return "", nil
	}
	svcName := nginx.Status.Services[0].Name
	var svc corev1.Service
	err = m.cli.Get(m.ctx, types.NamespacedName{Name: svcName, Namespace: rpaasInstance.Namespace}, &svc)
	if err != nil {
		return "", err
	}
	if len(svc.Status.LoadBalancer.Ingress) > 0 {
		return svc.Status.LoadBalancer.Ingress[0].IP, nil
	}
	return svc.Spec.ClusterIP, nil
}

func (m *k8sRpaasManager) GetInstance(name string) (*v1alpha1.RpaasInstance, error) {
	list := &v1alpha1.RpaasInstanceList{}
	err := m.cli.List(m.ctx, client.MatchingField("metadata.name", name), list)
	if err != nil {
		return nil, err
	}
	// Let's filter the list again, field selector implementation is not always
	// trustyworthy (mainly on tests). If it works correctly we're only
	// iterating on 1 item at most, so no problem playing safe here.
	for i := 0; i < len(list.Items); i++ {
		if list.Items[i].Name != name {
			list.Items[i] = list.Items[len(list.Items)-1]
			list.Items = list.Items[:len(list.Items)-1]
			i--
		}
	}
	if len(list.Items) == 0 {
		return nil, NotFoundError{Msg: fmt.Sprintf("rpaas instances %q not found", name)}
	}
	if len(list.Items) > 1 {
		return nil, ConflictError{Msg: fmt.Sprintf("multiple instances found for name %q: %#v", name, list.Items)}
	}
	return &list.Items[0], nil
}

func (m *k8sRpaasManager) getPlan(name string) (*v1alpha1.RpaasPlan, error) {
	plan := &v1alpha1.RpaasPlan{}
	err := m.cli.Get(m.ctx, types.NamespacedName{Name: name}, plan)
	if err != nil && k8sErrors.IsNotFound(err) {
		return nil, NotFoundError{Msg: fmt.Sprintf("plan %q not found", name)}
	}
	if err != nil {
		return nil, err
	}
	return plan, nil
}

func (m *k8sRpaasManager) getCertificateSecret(ri v1alpha1.RpaasInstance, name string) (*corev1.Secret, error) {
	namespacedName := types.NamespacedName{
		Name:      formatCertificateSecretName(ri, name),
		Namespace: ri.Namespace,
	}
	secret := &corev1.Secret{}
	err := m.cli.Get(m.ctx, namespacedName, secret)
	return secret, err
}

func (m *k8sRpaasManager) createCertificateSecret(ri v1alpha1.RpaasInstance, name string, c *tls.Certificate) (*corev1.Secret, error) {
	rawCertPem, rawKeyPem, err := convertTLSCertificate(c)
	if err != nil {
		return nil, err
	}
	secret := newCertificateSecret(ri, name, rawCertPem, rawKeyPem)
	err = m.cli.Create(m.ctx, secret)
	return secret, err
}

func (m *k8sRpaasManager) updateCertificateSecret(s *corev1.Secret, c *tls.Certificate) error {
	certificatePem, keyPem, err := convertTLSCertificate(c)
	if err != nil {
		return err
	}
	s.Data["certificate"] = certificatePem
	s.Data["key"] = keyPem
	return m.cli.Update(m.ctx, s)
}

func (m *k8sRpaasManager) updateCertificates(ri *v1alpha1.RpaasInstance, certs map[string]nginxv1alpha1.TLSSecret) error {
	ri.Spec.Certificates = certs
	return m.cli.Update(m.ctx, ri)
}

func (m *k8sRpaasManager) createNamespace(name string) error {
	ns := &corev1.Namespace{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "v1",
			Kind:       "Namespace",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
		},
		Spec: corev1.NamespaceSpec{},
	}
	return m.cli.Create(m.ctx, ns)
}

func (m *k8sRpaasManager) createConfigurationBlocks(instance v1alpha1.RpaasInstance, block, content string) error {
	configBlocks := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      formatConfigurationBlocksName(instance),
			Namespace: instance.ObjectMeta.Namespace,
			OwnerReferences: []metav1.OwnerReference{
				*metav1.NewControllerRef(&instance, schema.GroupVersionKind{
					Group:   v1alpha1.SchemeGroupVersion.Group,
					Version: v1alpha1.SchemeGroupVersion.Version,
					Kind:    "RpaasInstance",
				}),
			},
		},
		Data: map[string]string{
			block: content,
		},
	}
	return m.cli.Create(m.ctx, configBlocks)
}

func (m *k8sRpaasManager) getConfigurationBlocks(instance v1alpha1.RpaasInstance) (*corev1.ConfigMap, error) {
	cm := &corev1.ConfigMap{}
	namespacedName := types.NamespacedName{
		Name:      formatConfigurationBlocksName(instance),
		Namespace: instance.ObjectMeta.Namespace,
	}
	err := m.cli.Get(m.ctx, namespacedName, cm)
	return cm, err
}

func (m *k8sRpaasManager) updateConfigurationBlocks(instance v1alpha1.RpaasInstance, block, content string) error {
	configBlocks, err := m.getConfigurationBlocks(instance)
	if err != nil && k8sErrors.IsNotFound(err) {
		return m.createConfigurationBlocks(instance, block, content)
	}
	if err != nil {
		return err
	}
	if configBlocks.Data == nil {
		configBlocks.Data = map[string]string{}
	}
	configBlocks.Data[block] = content
	return m.cli.Update(m.ctx, configBlocks)
}

func (m *k8sRpaasManager) deleteConfigurationBlocks(instance v1alpha1.RpaasInstance, block string) error {
	configBlocks, err := m.getConfigurationBlocks(instance)
	if err != nil && k8sErrors.IsNotFound(err) {
		return ErrBlockIsNotDefined
	}
	if err != nil {
		return err
	}
	if configBlocks.Data == nil {
		return ErrBlockIsNotDefined
	}
	if _, ok := configBlocks.Data[block]; !ok {
		return ErrBlockIsNotDefined
	}
	delete(configBlocks.Data, block)
	return m.cli.Update(m.ctx, configBlocks)
}

func convertTLSCertificate(c *tls.Certificate) ([]byte, []byte, error) {
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

func formatCertificateSecretName(ri v1alpha1.RpaasInstance, name string) string {
	return fmt.Sprintf("%s-certificate-%s", ri.ObjectMeta.Name, name)
}

func formatConfigurationBlocksName(instance v1alpha1.RpaasInstance) string {
	return fmt.Sprintf("%s-blocks", instance.ObjectMeta.Name)
}

func newCertificateSecret(ri v1alpha1.RpaasInstance, name string, rawCertPem, rawKeyPem []byte) *corev1.Secret {
	return &corev1.Secret{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "v1",
			Kind:       "Secret",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      formatCertificateSecretName(ri, name),
			Namespace: ri.ObjectMeta.Namespace,
			OwnerReferences: []metav1.OwnerReference{
				*metav1.NewControllerRef(&ri, schema.GroupVersionKind{
					Group:   v1alpha1.SchemeGroupVersion.Group,
					Version: v1alpha1.SchemeGroupVersion.Version,
					Kind:    "RpaasInstance",
				}),
			},
		},
		Data: map[string][]byte{
			"certificate": rawCertPem,
			"key":         rawKeyPem,
		},
	}
}

func newTLSSecret(s *corev1.Secret, name string) *nginxv1alpha1.TLSSecret {
	return &nginxv1alpha1.TLSSecret{
		SecretName:       s.ObjectMeta.Name,
		CertificateField: "certificate",
		CertificatePath:  fmt.Sprintf("%s.crt.pem", name),
		KeyField:         "key",
		KeyPath:          fmt.Sprintf("%s.key.pem", name),
	}
}

func (m *k8sRpaasManager) validateCreate(args CreateArgs) (*v1alpha1.RpaasPlan, error) {
	if args.Name == "" {
		return nil, ValidationError{Msg: "name is required"}
	}
	if args.Plan == "" {
		return nil, ValidationError{Msg: "plan is required"}
	}
	if args.Team == "" {
		return nil, ValidationError{Msg: "team name is required"}
	}
	return m.getPlan(args.Plan)
}

func NamespaceName(team string) string {
	return fmt.Sprintf("%s-%s", defaultNamespacePrefix, team)
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

func parseTags(args CreateArgs) {
	parseTagArg(args.Tags, "flavor", &args.Flavor)
	parseTagArg(args.Tags, "ip", &args.IP)
}

func getAvailableBlocks() []v1alpha1.BlockType {
	return []v1alpha1.BlockType{
		v1alpha1.BlockTypeRoot,
		v1alpha1.BlockTypeHTTP,
		v1alpha1.BlockTypeServer,
	}
}

func isBlockValid(block string) bool {
	for _, b := range getAvailableBlocks() {
		if v1alpha1.BlockType(block) == b {
			return true
		}
	}
	return false
}
