package rpaas

import (
	"bytes"
	"context"
	"crypto"
	"crypto/ecdsa"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"encoding/pem"
	"errors"
	"fmt"

	nginxv1alpha1 "github.com/tsuru/nginx-operator/pkg/apis/nginx/v1alpha1"
	"github.com/tsuru/rpaas-operator/pkg/apis/extensions/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	k8sErrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type RpaasManager interface {
	UpdateCertificate(string, tls.Certificate) error
}

type K8SOptions struct {
	Cli       client.Client
	Ctx       context.Context
	Namespace string
}

type k8sRpaasManager struct {
	cli       client.Client
	ctx       context.Context
	namespace string
}

func NewK8S(o K8SOptions) RpaasManager {
	return &k8sRpaasManager{
		cli:       o.Cli,
		ctx:       o.Ctx,
		namespace: o.Namespace,
	}
}

func (m *k8sRpaasManager) UpdateCertificate(instance string, c tls.Certificate) error {
	rpaasInstance, err := m.getRpaasInstanceByName(instance)
	if err != nil {
		return err
	}
	secret, err := m.getCertificateSecret(rpaasInstance, v1alpha1.CertificateNameDefault)
	if err == nil {
		return m.updateCertificateSecret(secret, &c)
	}
	if !k8sErrors.IsNotFound(err) {
		return err
	}
	secret, err = m.createCertificateSecret(rpaasInstance, v1alpha1.CertificateNameDefault, &c)
	if err != nil {
		return err
	}
	certs := map[v1alpha1.CertificateName]nginxv1alpha1.TLSSecret{
		v1alpha1.CertificateNameDefault: *newTLSSecret(secret, v1alpha1.CertificateNameDefault),
	}
	return m.updateCertificates(rpaasInstance, certs)
}

func (m *k8sRpaasManager) getRpaasInstanceByName(name string) (*v1alpha1.RpaasInstance, error) {
	namespacedName := types.NamespacedName{
		Name:      name,
		Namespace: m.namespace,
	}
	rpaasInstance := &v1alpha1.RpaasInstance{}
	err := m.cli.Get(context.TODO(), namespacedName, rpaasInstance)
	return rpaasInstance, err
}

func (m *k8sRpaasManager) getCertificateSecret(ri *v1alpha1.RpaasInstance, name v1alpha1.CertificateName) (*corev1.Secret, error) {
	namespacedName := types.NamespacedName{
		Name:      formatCertificateSecretName(ri, name),
		Namespace: m.namespace,
	}
	secret := &corev1.Secret{}
	err := m.cli.Get(context.TODO(), namespacedName, secret)
	return secret, err
}

func (m *k8sRpaasManager) createCertificateSecret(ri *v1alpha1.RpaasInstance, name v1alpha1.CertificateName, c *tls.Certificate) (*corev1.Secret, error) {
	rawCertPem, rawKeyPem, err := convertTLSCertificate(c)
	if err != nil {
		return nil, err
	}
	secret := newCertificateSecret(ri, name, rawCertPem, rawKeyPem)
	err = m.cli.Create(context.TODO(), secret)
	return secret, err
}

func (m *k8sRpaasManager) updateCertificateSecret(s *corev1.Secret, c *tls.Certificate) error {
	certificatePem, keyPem, err := convertTLSCertificate(c)
	if err != nil {
		return err
	}
	s.Data["certificate"] = certificatePem
	s.Data["key"] = keyPem
	return m.cli.Update(context.TODO(), s)
}

func (m *k8sRpaasManager) updateCertificates(ri *v1alpha1.RpaasInstance, certs map[v1alpha1.CertificateName]nginxv1alpha1.TLSSecret) error {
	ri.Spec.Certificates = certs
	return m.cli.Update(context.TODO(), ri)
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

func formatCertificateSecretName(ri *v1alpha1.RpaasInstance, name v1alpha1.CertificateName) string {
	return fmt.Sprintf("%s-certificate-%s", ri.ObjectMeta.Name, name)
}

func newCertificateSecret(ri *v1alpha1.RpaasInstance, name v1alpha1.CertificateName, rawCertPem, rawKeyPem []byte) *corev1.Secret {
	return &corev1.Secret{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "v1",
			Kind:       "Secret",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      formatCertificateSecretName(ri, name),
			Namespace: ri.ObjectMeta.Namespace,
			OwnerReferences: []metav1.OwnerReference{
				*metav1.NewControllerRef(ri, schema.GroupVersionKind{
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

func newTLSSecret(s *corev1.Secret, name v1alpha1.CertificateName) *nginxv1alpha1.TLSSecret {
	return &nginxv1alpha1.TLSSecret{
		SecretName:       s.ObjectMeta.Name,
		CertificateField: "certificate",
		CertificatePath:  fmt.Sprintf("%s.crt.pem", name),
		KeyField:         "key",
		KeyPath:          fmt.Sprintf("%s.key.pem", name),
	}
}
