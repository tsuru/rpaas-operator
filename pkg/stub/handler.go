package stub

import (
	"context"
	"fmt"

	"github.com/operator-framework/operator-sdk/pkg/sdk"
	"github.com/sirupsen/logrus"
	nginxV1alpha1 "github.com/tsuru/nginx-operator/pkg/apis/nginx/v1alpha1"
	"github.com/tsuru/rpaas-operator/pkg/apis/extensions/v1alpha1"
	"github.com/tsuru/rpaas-operator/pkg/stub/generator"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	k8sErrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

func NewHandler() sdk.Handler {
	return &Handler{}
}

type Handler struct {
	// Fill me
}

func (h *Handler) ReadConfigRef(ref v1alpha1.ConfigRef, ns string) (string, error) {
	switch ref.Kind {
	case v1alpha1.ConfigKindInline:
		return ref.Value, nil
	case v1alpha1.ConfigKindConfigMap:
		configMap := &corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{
				Name:      ref.Name,
				Namespace: ns,
			},
			TypeMeta: metav1.TypeMeta{
				Kind:       "ConfigMap",
				APIVersion: "v1",
			},
		}
		err := sdk.Get(configMap)
		if err != nil {
			return "", err
		}
		return configMap.Data[ref.Value], nil
	default:
		return "", fmt.Errorf("invalid config kind for %#v", ref)
	}
}

func (h *Handler) Handle(ctx context.Context, event sdk.Event) error {
	switch o := event.Object.(type) {
	case *v1alpha1.RpaasInstance:
		nginx, err := h.newNginx(o)
		if err != nil {
			return err
		}
		err = sdk.Create(nginx)
		if err != nil && !errors.IsAlreadyExists(err) {
			logrus.Errorf("Failed to create busybox pod : %v", err)
			return err
		}
	}
	return nil
}

func (h *Handler) newNginx(cr *v1alpha1.RpaasInstance) (*nginxV1alpha1.Nginx, error) {
	plan := &v1alpha1.RpaasPlan{
		ObjectMeta: metav1.ObjectMeta{
			Name:      cr.Spec.PlanName,
			Namespace: cr.Namespace,
		},
		TypeMeta: metav1.TypeMeta{
			Kind:       "RpaasPlan",
			APIVersion: "extensions.tsuru.io/v1alpha1",
		},
	}
	err := sdk.Get(plan)
	if err != nil {
		return nil, err
	}
	builder := generator.ConfigBuilder{
		RefReader: h,
	}
	renderedTemplate, err := builder.Interpolate(*cr, plan.Spec)
	if err != nil {
		return nil, err
	}
	confMap := corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      fmt.Sprintf("nginx-%s", cr.Name),
			Namespace: cr.Namespace,
			OwnerReferences: []metav1.OwnerReference{
				*metav1.NewControllerRef(cr, schema.GroupVersionKind{
					Group:   v1alpha1.SchemeGroupVersion.Group,
					Version: v1alpha1.SchemeGroupVersion.Version,
					Kind:    "RpaasInstance",
				}),
			},
		},
		TypeMeta: metav1.TypeMeta{
			Kind:       "ConfigMap",
			APIVersion: "v1",
		},
		Data: map[string]string{
			"nginx.conf": renderedTemplate,
		},
	}
	err = sdk.Create(&confMap)
	if err != nil && !k8sErrors.IsAlreadyExists(err) {
		return nil, err
	}
	nginx := &nginxV1alpha1.Nginx{
		ObjectMeta: metav1.ObjectMeta{
			Name:      cr.Name,
			Namespace: cr.Namespace,
			OwnerReferences: []metav1.OwnerReference{
				*metav1.NewControllerRef(cr, schema.GroupVersionKind{
					Group:   v1alpha1.SchemeGroupVersion.Group,
					Version: v1alpha1.SchemeGroupVersion.Version,
					Kind:    "RpaasInstance",
				}),
			},
		},
		TypeMeta: metav1.TypeMeta{
			Kind:       "Nginx",
			APIVersion: "nginx.tsuru.io/v1alpha1",
		},
		Spec: nginxV1alpha1.NginxSpec{
			Image:    plan.Spec.Image,
			Replicas: cr.Spec.Replicas,
			Config: &nginxV1alpha1.ConfigRef{
				Name: confMap.Name,
				Kind: nginxV1alpha1.ConfigKindConfigMap,
			},
		},
	}
	return nginx, nil
}
