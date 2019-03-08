package rpaasinstance

import (
	"context"
	"fmt"

	"github.com/sirupsen/logrus"
	nginxV1alpha1 "github.com/tsuru/nginx-operator/pkg/apis/nginx/v1alpha1"
	"github.com/tsuru/rpaas-operator/pkg/apis/extensions/v1alpha1"
	extensionsv1alpha1 "github.com/tsuru/rpaas-operator/pkg/apis/extensions/v1alpha1"
	"github.com/tsuru/rpaas-operator/pkg/generator"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	k8sErrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	logf "sigs.k8s.io/controller-runtime/pkg/runtime/log"
	"sigs.k8s.io/controller-runtime/pkg/source"
)

var log = logf.Log.WithName("controller_rpaasinstance")

/**
* USER ACTION REQUIRED: This is a scaffold file intended for the user to modify with their own Controller
* business logic.  Delete these comments after modifying this file.*
 */

// Add creates a new RpaasInstance Controller and adds it to the Manager. The Manager will set fields on the Controller
// and Start it when the Manager is Started.
func Add(mgr manager.Manager) error {
	return add(mgr, newReconciler(mgr))
}

// newReconciler returns a new reconcile.Reconciler
func newReconciler(mgr manager.Manager) reconcile.Reconciler {
	return &ReconcileRpaasInstance{client: mgr.GetClient(), scheme: mgr.GetScheme()}
}

// add adds a new Controller to mgr with r as the reconcile.Reconciler
func add(mgr manager.Manager, r reconcile.Reconciler) error {
	// Create a new controller
	c, err := controller.New("rpaasinstance-controller", mgr, controller.Options{Reconciler: r})
	if err != nil {
		return err
	}

	// Watch for changes to primary resource RpaasInstance
	err = c.Watch(&source.Kind{Type: &extensionsv1alpha1.RpaasInstance{}}, &handler.EnqueueRequestForObject{})
	if err != nil {
		return err
	}

	// TODO(user): Modify this to be the types you create that are owned by the primary resource
	// Watch for changes to secondary resource Pods and requeue the owner RpaasInstance
	err = c.Watch(&source.Kind{Type: &corev1.Pod{}}, &handler.EnqueueRequestForOwner{
		IsController: true,
		OwnerType:    &extensionsv1alpha1.RpaasInstance{},
	})
	if err != nil {
		return err
	}

	return nil
}

var _ reconcile.Reconciler = &ReconcileRpaasInstance{}

// ReconcileRpaasInstance reconciles a RpaasInstance object
type ReconcileRpaasInstance struct {
	// This client, initialized using mgr.Client() above, is a split client
	// that reads objects from the cache and writes to the apiserver
	client client.Client
	scheme *runtime.Scheme
}

// Reconcile reads that state of the cluster for a RpaasInstance object and makes changes based on the state read
// and what is in the RpaasInstance.Spec
// TODO(user): Modify this Reconcile function to implement your Controller logic.  This example creates
// a Pod as an example
// Note:
// The Controller will requeue the Request to be processed again if the returned error is non-nil or
// Result.Requeue is true, otherwise upon completion it will remove the work from the queue.
func (r *ReconcileRpaasInstance) Reconcile(request reconcile.Request) (reconcile.Result, error) {
	reqLogger := log.WithValues("Request.Namespace", request.Namespace, "Request.Name", request.Name)
	reqLogger.Info("Reconciling RpaasInstance")

	// Fetch the RpaasInstance instance
	instance := &extensionsv1alpha1.RpaasInstance{}
	err := r.client.Get(context.TODO(), request.NamespacedName, instance)
	if err != nil {
		if errors.IsNotFound(err) {
			// Request object not found, could have been deleted after reconcile request.
			// Owned objects are automatically garbage collected. For additional cleanup logic use finalizers.
			// Return and don't requeue
			return reconcile.Result{}, nil
		}
		// Error reading the object - requeue the request.
		return reconcile.Result{}, err
	}

	plan, err := getPlan(r, instance)
	if err != nil {
		return reconcile.Result{}, err
	}
	rendered, err := r.renderTemplate(instance, plan)
	if err != nil {
		return reconcile.Result{}, err
	}
	configMap := newConfigMap(instance, rendered)
	if err != nil {
		return reconcile.Result{}, err
	}
	nginx := newNginx(instance, plan, configMap)
	if err != nil {
		return reconcile.Result{}, err
	}
	err = r.client.Create(context.TODO(), configMap)
	if err != nil && !k8sErrors.IsAlreadyExists(err) {
		logrus.Errorf("Failed to create configmap: %v", err)
		return reconcile.Result{}, err
	}
	err = r.reconcileNginx(nginx)
	return reconcile.Result{}, err
}

func (r *ReconcileRpaasInstance) reconcileNginx(nginx *nginxV1alpha1.Nginx) error {
	foundNginx := &nginxV1alpha1.Nginx{}
	err := r.client.Get(context.TODO(), types.NamespacedName{Name: nginx.ObjectMeta.Name, Namespace: nginx.ObjectMeta.Namespace}, foundNginx)
	if err != nil {
		if !k8sErrors.IsAlreadyExists(err) {
			logrus.Errorf("Failed to get nginx CR: %v", err)
			return err
		}
		err = r.client.Create(context.TODO(), nginx)
		if err != nil {
			logrus.Errorf("Failed to create nginx CR: %v", err)
			return err
		}
		return nil
	}

	nginx.ObjectMeta.ResourceVersion = foundNginx.ObjectMeta.ResourceVersion
	err = r.client.Update(context.TODO(), nginx)
	if err != nil {
		logrus.Errorf("Failed to update nginx CR: %v", err)
		return err
	}
	return nil
}

func (r *ReconcileRpaasInstance) renderTemplate(instance *v1alpha1.RpaasInstance, plan *v1alpha1.RpaasPlan) (string, error) {
	builder := generator.ConfigBuilder{
		RefReader: r,
	}
	renderedTemplate, err := builder.Interpolate(*instance, plan.Spec)
	if err != nil {
		return "", err
	}
	return renderedTemplate, nil
}

func (r *ReconcileRpaasInstance) ReadConfigRef(ref v1alpha1.ConfigRef, ns string) (string, error) {
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
		err := r.client.Get(context.TODO(), types.NamespacedName{
			Namespace: ns,
			Name:      ref.Name,
		}, configMap)
		if err != nil {
			return "", err
		}
		return configMap.Data[ref.Value], nil
	default:
		return "", fmt.Errorf("invalid config kind for %#v", ref)
	}
}

// newPodForCR returns a busybox pod with the same name/namespace as the cr
func newPodForCR(cr *extensionsv1alpha1.RpaasInstance) *corev1.Pod {
	labels := map[string]string{
		"app": cr.Name,
	}
	return &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      cr.Name + "-pod",
			Namespace: cr.Namespace,
			Labels:    labels,
		},
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{
				{
					Name:    "busybox",
					Image:   "busybox",
					Command: []string{"sleep", "3600"},
				},
			},
		},
	}
}

func getPlan(r *ReconcileRpaasInstance, instance *v1alpha1.RpaasInstance) (*v1alpha1.RpaasPlan, error) {
	plan := &v1alpha1.RpaasPlan{
		ObjectMeta: metav1.ObjectMeta{
			Name:      instance.Spec.PlanName,
			Namespace: instance.Namespace,
		},
		TypeMeta: metav1.TypeMeta{
			Kind:       "RpaasPlan",
			APIVersion: "extensions.tsuru.io/v1alpha1",
		},
	}
	err := r.client.Get(context.TODO(), types.NamespacedName{
		Namespace: instance.Namespace,
		Name:      instance.Spec.PlanName,
	}, plan)
	if err != nil {
		return nil, err
	}
	return plan, nil
}

func newConfigMap(instance *v1alpha1.RpaasInstance, renderedTemplate string) *corev1.ConfigMap {
	return &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      fmt.Sprintf("nginx-%s", instance.Name),
			Namespace: instance.Namespace,
			OwnerReferences: []metav1.OwnerReference{
				*metav1.NewControllerRef(instance, schema.GroupVersionKind{
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
}

func newNginx(instance *v1alpha1.RpaasInstance, plan *v1alpha1.RpaasPlan, configMap *corev1.ConfigMap) *nginxV1alpha1.Nginx {
	return &nginxV1alpha1.Nginx{
		ObjectMeta: metav1.ObjectMeta{
			Name:      instance.Name,
			Namespace: instance.Namespace,
			OwnerReferences: []metav1.OwnerReference{
				*metav1.NewControllerRef(instance, schema.GroupVersionKind{
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
			Replicas: instance.Spec.Replicas,
			Config: &nginxV1alpha1.ConfigRef{
				Name: configMap.Name,
				Kind: nginxV1alpha1.ConfigKindConfigMap,
			},
			Service: &nginxV1alpha1.NginxService{
				Name: configMap.Name,
				Type: "NodePort",
			},
		},
	}
}
