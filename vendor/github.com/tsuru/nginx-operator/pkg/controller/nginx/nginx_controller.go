package nginx

import (
	"context"
	"fmt"
	"reflect"
	"sort"

	nginxv1alpha1 "github.com/tsuru/nginx-operator/pkg/apis/nginx/v1alpha1"
	"github.com/tsuru/nginx-operator/pkg/k8s"
	appv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	logf "sigs.k8s.io/controller-runtime/pkg/runtime/log"
	"sigs.k8s.io/controller-runtime/pkg/source"
)

var log = logf.Log.WithName("controller_nginx")

// Add creates a new Nginx Controller and adds it to the Manager. The Manager will set fields on the Controller
// and Start it when the Manager is Started.
func Add(mgr manager.Manager) error {
	return add(mgr, newReconciler(mgr))
}

// newReconciler returns a new reconcile.Reconciler
func newReconciler(mgr manager.Manager) reconcile.Reconciler {
	return &ReconcileNginx{client: mgr.GetClient(), scheme: mgr.GetScheme()}
}

// add adds a new Controller to mgr with r as the reconcile.Reconciler
func add(mgr manager.Manager, r reconcile.Reconciler) error {
	// Create a new controller
	c, err := controller.New("nginx-controller", mgr, controller.Options{Reconciler: r})
	if err != nil {
		return err
	}

	// Watch for changes to primary resource Nginx
	err = c.Watch(&source.Kind{Type: &nginxv1alpha1.Nginx{}}, &handler.EnqueueRequestForObject{})
	if err != nil {
		return err
	}

	// HACK(nettoclaudio): Since the Nginx needs store all its pods' info into
	// the status field, we need watching every pod changes and enqueue a new
	// reconcile request to its Nginx owner, if any.
	return c.Watch(&source.Kind{Type: &corev1.Pod{}},
		&handler.EnqueueRequestsFromMapFunc{
			ToRequests: handler.ToRequestsFunc(func(o handler.MapObject) []reconcile.Request {
				if nginxCR, ok := o.Meta.GetLabels()["nginx_cr"]; ok {
					return []reconcile.Request{
						{NamespacedName: types.NamespacedName{
							Name:      nginxCR,
							Namespace: o.Meta.GetNamespace(),
						}},
					}
				}
				return []reconcile.Request{}
			}),
		},
	)
}

var _ reconcile.Reconciler = &ReconcileNginx{}

// ReconcileNginx reconciles a Nginx object
type ReconcileNginx struct {
	// This client, initialized using mgr.Client() above, is a split client
	// that reads objects from the cache and writes to the apiserver
	client client.Client
	scheme *runtime.Scheme
}

// Reconcile reads that state of the cluster for a Nginx object and makes changes based on the state read
// and what is in the Nginx.Spec
// TODO(user): Modify this Reconcile function to implement your Controller logic.  This example creates
// a Pod as an example
// Note:
// The Controller will requeue the Request to be processed again if the returned error is non-nil or
// Result.Requeue is true, otherwise upon completion it will remove the work from the queue.
func (r *ReconcileNginx) Reconcile(request reconcile.Request) (reconcile.Result, error) {
	reqLogger := log.WithValues("Request.Namespace", request.Namespace, "Request.Name", request.Name)
	reqLogger.Info("Reconciling Nginx")

	// Fetch the Nginx instance
	instance := &nginxv1alpha1.Nginx{}
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

	if err := r.reconcileNginx(instance); err != nil {
		reqLogger.Error(err, "fail to reconcile")
		return reconcile.Result{}, err
	}

	if err := r.refreshStatus(instance); err != nil {
		reqLogger.Error(err, "fail to refresh status")
		return reconcile.Result{}, err
	}

	return reconcile.Result{}, nil
}

func (r *ReconcileNginx) reconcileNginx(nginx *nginxv1alpha1.Nginx) error {

	if err := r.reconcileDeployment(nginx); err != nil {
		return err
	}

	if err := r.reconcileService(nginx); err != nil {
		return err
	}

	return nil
}

func (r *ReconcileNginx) reconcileDeployment(nginx *nginxv1alpha1.Nginx) error {
	newDeploy, err := k8s.NewDeployment(nginx)
	if err != nil {
		return fmt.Errorf("failed to assemble deployment from nginx: %v", err)
	}

	err = r.client.Create(context.TODO(), newDeploy)
	if err != nil && !errors.IsAlreadyExists(err) {
		return fmt.Errorf("failed to create deployment: %v", err)
	}

	if err == nil {
		return nil
	}

	currDeploy := &appv1.Deployment{}

	err = r.client.Get(context.TODO(), types.NamespacedName{Name: newDeploy.Name, Namespace: newDeploy.Namespace}, currDeploy)
	if err != nil {
		return fmt.Errorf("failed to retrieve deployment: %v", err)
	}

	currSpec, err := k8s.ExtractNginxSpec(currDeploy.ObjectMeta)
	if err != nil {
		return fmt.Errorf("failed to extract nginx from deployment: %v", err)
	}

	if reflect.DeepEqual(nginx.Spec, currSpec) {
		return nil
	}

	currDeploy.Spec = newDeploy.Spec
	if err := k8s.SetNginxSpec(&currDeploy.ObjectMeta, nginx.Spec); err != nil {
		return fmt.Errorf("failed to set nginx spec into object meta: %v", err)
	}

	if err := r.client.Update(context.TODO(), currDeploy); err != nil {
		return fmt.Errorf("failed to update deployment: %v", err)
	}

	return nil
}

func (r *ReconcileNginx) reconcileService(nginx *nginxv1alpha1.Nginx) error {
	svcName := types.NamespacedName{
		Name:      fmt.Sprintf("%s-service", nginx.Name),
		Namespace: nginx.Namespace,
	}
	currentService := &corev1.Service{}
	err := r.client.Get(context.TODO(), svcName, currentService)

	if err != nil && errors.IsNotFound(err) {
		return r.client.Create(context.TODO(), k8s.NewService(nginx))
	}

	if err != nil {
		return fmt.Errorf("failed to retrieve Service resource: %v", err)
	}

	newService := k8s.NewService(nginx)
	// avoid nodeport reallocation preserving the current ones
	for _, currentPort := range currentService.Spec.Ports {
		for index, newPort := range newService.Spec.Ports {
			if currentPort.Port == newPort.Port {
				newService.Spec.Ports[index].NodePort = currentPort.NodePort
			}
		}
	}

	if reflect.DeepEqual(currentService.Spec.Ports, newService.Spec.Ports) {
		return nil
	}

	currentService.Spec.Ports = newService.Spec.Ports
	return r.client.Update(context.TODO(), currentService)
}

func (r *ReconcileNginx) refreshStatus(nginx *nginxv1alpha1.Nginx) error {

	pods, err := listPods(r.client, nginx)
	if err != nil {
		return fmt.Errorf("failed to list pods for nginx: %v", err)
	}

	services, err := listServices(r.client, nginx)
	if err != nil {
		return fmt.Errorf("failed to list services for nginx: %v", err)
	}

	sort.Slice(nginx.Status.Pods, func(i, j int) bool {
		return nginx.Status.Pods[i].Name < nginx.Status.Pods[j].Name
	})

	sort.Slice(nginx.Status.Services, func(i, j int) bool {
		return nginx.Status.Services[i].Name < nginx.Status.Services[j].Name
	})

	if !reflect.DeepEqual(pods, nginx.Status.Pods) || !reflect.DeepEqual(services, nginx.Status.Services) {
		nginx.Status.Pods = pods
		nginx.Status.Services = services
		err := r.client.Status().Update(context.TODO(), nginx)
		if err != nil {
			return fmt.Errorf("failed to update nginx status: %v", err)
		}
	}

	return nil
}

// listPods return all the pods for the given nginx sorted by name
func listPods(c client.Client, nginx *nginxv1alpha1.Nginx) ([]nginxv1alpha1.PodStatus, error) {
	podList := &corev1.PodList{}

	labelSelector := labels.SelectorFromSet(k8s.LabelsForNginx(nginx.Name))
	listOps := &client.ListOptions{Namespace: nginx.Namespace, LabelSelector: labelSelector}
	err := c.List(context.TODO(), listOps, podList)
	if err != nil {
		return nil, err
	}

	var pods []nginxv1alpha1.PodStatus
	for _, p := range podList.Items {
		if p.Status.PodIP == "" {
			p.Status.PodIP = "<pending>"
		}
		pods = append(pods, nginxv1alpha1.PodStatus{
			Name:  p.Name,
			PodIP: p.Status.PodIP,
		})
	}
	sort.Slice(pods, func(i, j int) bool {
		return pods[i].Name < pods[j].Name
	})

	return pods, nil
}

// listServices return all the services for the given nginx sorted by name
func listServices(c client.Client, nginx *nginxv1alpha1.Nginx) ([]nginxv1alpha1.ServiceStatus, error) {
	serviceList := &corev1.ServiceList{}

	labelSelector := labels.SelectorFromSet(k8s.LabelsForNginx(nginx.Name))
	listOps := &client.ListOptions{Namespace: nginx.Namespace, LabelSelector: labelSelector}
	err := c.List(context.TODO(), listOps, serviceList)
	if err != nil {
		return nil, err
	}

	var services []nginxv1alpha1.ServiceStatus
	for _, s := range serviceList.Items {
		services = append(services, nginxv1alpha1.ServiceStatus{
			Name: s.Name,
		})
	}

	sort.Slice(services, func(i, j int) bool {
		return services[i].Name < services[j].Name
	})

	return services, nil
}
