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
	"strings"

	"github.com/pkg/errors"
	nginxv1alpha1 "github.com/tsuru/nginx-operator/pkg/apis/nginx/v1alpha1"
	"github.com/tsuru/rpaas-operator/config"
	"github.com/tsuru/rpaas-operator/pkg/apis/extensions/v1alpha1"
	"github.com/tsuru/rpaas-operator/pkg/util"
	nginxManager "github.com/tsuru/rpaas-operator/rpaas/nginx"
	corev1 "k8s.io/api/core/v1"
	k8sErrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/manager"
)

const (
	defaultNamespacePrefix = "rpaasv2"
)

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
	parseTags(args)
	plan, err := m.validateCreate(ctx, args)
	if err != nil {
		return err
	}
	_, err = m.GetInstance(ctx, args.Name)
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
	if err = m.createNamespace(ctx, namespaceName); err != nil && !k8sErrors.IsAlreadyExists(err) {
		return err
	}
	if args.PlanOverride != "" && args.Flavor != "" {
		return errors.New("cannot set both plan-override and flavor")
	}
	var planTemplate *v1alpha1.RpaasPlanSpec
	if args.PlanOverride != "" {
		err = json.Unmarshal([]byte(args.PlanOverride), &planTemplate)
		if err != nil {
			return errors.Wrapf(err, "unable to parse planOverride from data %q", args.PlanOverride)
		}
	}
	if args.Flavor != "" {
		conf := config.Get()
		for _, flavor := range conf.Flavors {
			if flavor.Name == args.Flavor {
				planTemplate = &flavor.Spec
				break
			}
		}
		if planTemplate == nil {
			return errors.Errorf("flavor %q not found", args.Flavor)
		}
	}
	oneReplica := int32(1)
	conf := config.Get()
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
				Annotations:    conf.ServiceAnnotations,
			},
			Replicas:     &oneReplica,
			PlanTemplate: planTemplate,
		},
	}
	err = m.cli.Create(ctx, instance)
	if err != nil {
		if k8sErrors.IsAlreadyExists(err) {
			return ConflictError{Msg: args.Name + " instance already exists"}
		}
		return err
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
	list := &v1alpha1.RpaasInstanceList{}
	err := m.cli.List(ctx, client.MatchingField("metadata.name", name), list)
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
		return nil, NotFoundError{Msg: fmt.Sprintf("rpaas instance %q not found", name)}
	}
	if len(list.Items) > 1 {
		return nil, ConflictError{Msg: fmt.Sprintf("multiple instances found for name %q: %#v", name, list.Items)}
	}
	return &list.Items[0], nil
}

func (m *k8sRpaasManager) GetPlans(ctx context.Context) ([]v1alpha1.RpaasPlan, error) {
	planList := &v1alpha1.RpaasPlanList{}
	err := m.cli.List(ctx, &client.ListOptions{}, planList)
	if err != nil && !k8sErrors.IsNotFound(err) {
		return []v1alpha1.RpaasPlan{}, nil
	}
	if err != nil {
		return nil, err
	}
	return planList.Items, nil
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

	if instance.Spec.Host != "" && instance.Spec.Host != args.AppHost {
		return &ConflictError{Msg: "instance already bound with another application"}
	}

	instance.Spec.Host = args.AppHost

	return m.cli.Update(ctx, instance)
}

func (m *k8sRpaasManager) UnbindApp(ctx context.Context, instanceName string) error {
	instance, err := m.GetInstance(ctx, instanceName)
	if err != nil {
		return err
	}

	if instance.Spec.Host == "" {
		return &ValidationError{Msg: "instance not bound"}
	}

	instance.Spec.Host = ""

	return m.cli.Update(ctx, instance)
}

func (m *k8sRpaasManager) PurgeCache(ctx context.Context, instanceName string, args PurgeCacheArgs) (int, error) {
	podMap, err := m.GetInstanceStatus(ctx, instanceName)
	if err != nil {
		return 0, err
	}
	if args.Path == "" {
		return 0, ValidationError{Msg: "path is required"}
	}
	purgeCount := 0
	for _, podStatus := range podMap {
		if !podStatus.Running {
			continue
		}
		if err = m.cacheManager.PurgeCache(podStatus.Address, args.Path, args.PreservePath); err != nil {
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

	var plan v1alpha1.RpaasPlan
	if err := m.cli.Get(ctx, types.NamespacedName{Name: name}, &plan); err != nil {
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

func (m *k8sRpaasManager) createNamespace(ctx context.Context, name string) error {
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
	return m.cli.Create(ctx, ns)
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

func (m *k8sRpaasManager) validateCreate(ctx context.Context, args CreateArgs) (*v1alpha1.RpaasPlan, error) {
	if args.Name == "" {
		return nil, ValidationError{Msg: "name is required"}
	}
	if args.Team == "" {
		return nil, ValidationError{Msg: "team name is required"}
	}
	plan, err := m.getPlan(ctx, args.Plan)
	if err != nil && IsNotFoundError(err) {
		return nil, ValidationError{Msg: "invalid plan"}
	}
	return plan, err
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
	parseTagArg(args.Tags, "plan-override", &args.PlanOverride)
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

func (m *k8sRpaasManager) GetInstanceStatus(ctx context.Context, name string) (PodStatusMap, error) {
	rpaasInstance, err := m.GetInstance(ctx, name)
	if err != nil {
		return nil, err
	}
	var nginx nginxv1alpha1.Nginx
	err = m.cli.Get(ctx, types.NamespacedName{Name: rpaasInstance.Name, Namespace: rpaasInstance.Namespace}, &nginx)
	if err != nil {
		return nil, err
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
	return podMap, nil
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
	listOpts := client.
		MatchingField("involvedObject.kind", podKind).
		MatchingField("involvedObject.name", podName)
	listOpts.Namespace = ns
	var eventList corev1.EventList
	err := m.nonCachedCli.List(ctx, listOpts, &eventList)
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
