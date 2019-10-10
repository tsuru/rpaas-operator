// Copyright 2019 tsuru authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package rpaasinstance

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"reflect"
	"sort"

	jsonpatch "github.com/evanphx/json-patch"
	"github.com/imdario/mergo"
	"github.com/sirupsen/logrus"
	nginxV1alpha1 "github.com/tsuru/nginx-operator/pkg/apis/nginx/v1alpha1"
	"github.com/tsuru/rpaas-operator/internal/pkg/rpaas/nginx"
	"github.com/tsuru/rpaas-operator/pkg/apis/extensions/v1alpha1"
	extensionsv1alpha1 "github.com/tsuru/rpaas-operator/pkg/apis/extensions/v1alpha1"
	"github.com/tsuru/rpaas-operator/pkg/util"
	autoscalingv2beta2 "k8s.io/api/autoscaling/v2beta2"
	corev1 "k8s.io/api/core/v1"
	k8sErrors "k8s.io/apimachinery/pkg/api/errors"
	k8sResources "k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/jsonmergepatch"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	logf "sigs.k8s.io/controller-runtime/pkg/runtime/log"
	"sigs.k8s.io/controller-runtime/pkg/source"
)

const (
	defaultConfigHistoryLimit = 10
)

var log = logf.Log.WithName("controller_rpaasinstance")

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

	err = c.Watch(&source.Kind{Type: &corev1.Secret{}}, &handler.EnqueueRequestForOwner{
		IsController: true,
		OwnerType:    &extensionsv1alpha1.RpaasInstance{},
	})
	if err != nil {
		return err
	}

	err = c.Watch(&source.Kind{Type: &corev1.ConfigMap{}}, &handler.EnqueueRequestForOwner{
		IsController: true,
		OwnerType:    &extensionsv1alpha1.RpaasInstance{},
	})

	return err
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
func (r *ReconcileRpaasInstance) Reconcile(request reconcile.Request) (reconcile.Result, error) {
	reqLogger := log.WithValues("Request.Namespace", request.Namespace, "Request.Name", request.Name)
	reqLogger.Info("Reconciling RpaasInstance")

	instance, err := r.getRpaasInstance(context.TODO(), request.NamespacedName)
	if err != nil && k8sErrors.IsNotFound(err) {
		reqLogger.Info("Nothing to do due the RpaasInstance was removed")
		return reconcile.Result{}, nil
	}

	if err != nil {
		return reconcile.Result{}, err
	}

	planName := types.NamespacedName{
		Name:      instance.Spec.PlanName,
		Namespace: instance.Namespace,
	}
	plan := &v1alpha1.RpaasPlan{}
	err = r.client.Get(context.TODO(), planName, plan)
	if err != nil {
		return reconcile.Result{}, err
	}

	if instance.Spec.PlanTemplate != nil {
		plan.Spec, err = mergePlans(plan.Spec, *instance.Spec.PlanTemplate)
		if err != nil {
			return reconcile.Result{}, err
		}
	}

	rendered, err := r.renderTemplate(instance, plan)
	if err != nil {
		return reconcile.Result{}, err
	}
	configMap := newConfigMap(instance, rendered)
	err = r.reconcileConfigMap(configMap)
	if err != nil {
		return reconcile.Result{}, err
	}
	configList, err := r.listConfigs(instance)
	if err != nil {
		return reconcile.Result{}, err
	}
	if shouldDeleteOldConfig(instance, configList) {
		if err = r.deleteOldConfig(instance, configList); err != nil {
			return reconcile.Result{}, err
		}
	}
	nginx := newNginx(instance, plan, configMap)

	if err = r.reconcileNginx(nginx); err != nil {
		return reconcile.Result{}, err
	}

	if err = r.reconcileHPA(context.TODO(), *instance, *nginx); err != nil {
		return reconcile.Result{}, err
	}

	return reconcile.Result{}, nil
}

func (r *ReconcileRpaasInstance) getRpaasInstance(ctx context.Context, objKey types.NamespacedName) (*v1alpha1.RpaasInstance, error) {
	logger := log.WithName("getRpaasInstance").WithValues("RpaasInstance", objKey)
	logger.V(4).Info("Getting the RpaasInstance resource")

	var instance v1alpha1.RpaasInstance
	if err := r.client.Get(ctx, objKey, &instance); err != nil {
		return nil, err
	}

	return r.mergeInstanceWithFlavors(ctx, instance.DeepCopy())
}

func (r *ReconcileRpaasInstance) mergeInstanceWithFlavors(ctx context.Context, instance *v1alpha1.RpaasInstance) (*v1alpha1.RpaasInstance, error) {
	logger := log.WithName("mergeInstanceWithFlavors").
		WithValues("RpaasInstance", types.NamespacedName{Name: instance.Name, Namespace: instance.Namespace})

	for _, flavorName := range instance.Spec.Flavors {
		flavorObjectKey := types.NamespacedName{
			Name:      flavorName,
			Namespace: instance.Namespace,
		}

		logger := logger.WithValues("RpaasFlavor", flavorObjectKey)
		logger.V(4).Info("Getting RpaasFlavor resource")

		var flavor v1alpha1.RpaasFlavor
		if err := r.client.Get(ctx, flavorObjectKey, &flavor); err != nil {
			logger.Error(err, "Unable to get the RpaasFlavor resource")
			return nil, err
		}

		if flavor.Spec.InstanceTemplate != nil {
			mergedInstanceSpec, err := mergeInstance(*flavor.Spec.InstanceTemplate, instance.Spec)
			if err != nil {
				logger.Error(err, "Could not merge instance against flavor instance template")
				return nil, err
			}

			logger.
				WithValues("RpaasInstanceSpec", instance.Spec).
				WithValues("InstanceTemplate", flavor.Spec.InstanceTemplate).
				WithValues("Merged", mergedInstanceSpec).
				V(4).
				Info("RpaasInstanceSpec successfully merged")

			instance.Spec = mergedInstanceSpec
		} else {
			logger.V(4).
				Info("Skipping RpaasInstance merge since there is no instance template in RpaasFlavor")
		}
	}

	return instance, nil
}

func (r *ReconcileRpaasInstance) reconcileHPA(ctx context.Context, instance v1alpha1.RpaasInstance, nginx nginxV1alpha1.Nginx) error {
	logger := log.WithName("reconcileHPA").
		WithValues("RpaasInstance", types.NamespacedName{Name: instance.Name, Namespace: instance.Namespace}).
		WithValues("Nginx", types.NamespacedName{Name: nginx.Name, Namespace: nginx.Namespace})

	logger.V(4).Info("Starting reconciliation of HorizontalPodAutoscaler")
	defer logger.V(4).Info("Finishing reconciliation of HorizontalPodAutoscaler")

	var hpa autoscalingv2beta2.HorizontalPodAutoscaler
	err := r.client.Get(ctx, types.NamespacedName{Name: instance.Name, Namespace: instance.Namespace}, &hpa)
	if err != nil && k8sErrors.IsNotFound(err) {
		if instance.Spec.Autoscale == nil {
			logger.V(4).Info("Skipping HorizontalPodAutoscaler reconciliation: both HPA resource and desired RpaasAutoscaleSpec not found")
			return nil
		}

		logger.V(4).Info("Creating HorizontalPodAutoscaler resource")

		hpa = newHPA(instance, nginx)
		if err = r.client.Create(ctx, &hpa); err != nil {
			logger.Error(err, "Unable to create the HorizontalPodAutoscaler resource")
			return err
		}

		return nil
	}

	if err != nil {
		logger.Error(err, "Unable to get the HorizontalPodAutoscaler resource")
		return err
	}

	logger = logger.WithValues("HorizontalPodAutoscaler", types.NamespacedName{Name: hpa.Name, Namespace: hpa.Namespace})

	if instance.Spec.Autoscale == nil {
		logger.V(4).Info("Deleting HorizontalPodAutoscaler resource")
		if err = r.client.Delete(ctx, &hpa); err != nil {
			logger.Error(err, "Unable to delete the HorizontalPodAutoscaler resource")
			return err
		}

		return nil
	}

	newerHPA := newHPA(instance, nginx)
	if !reflect.DeepEqual(hpa.Spec, newerHPA.Spec) {
		logger.V(4).Info("Updating the HorizontalPodAustocaler spec")

		hpa.Spec = newerHPA.Spec
		if err = r.client.Update(ctx, &hpa); err != nil {
			logger.Error(err, "Unable to update the HorizontalPodAustoscaler resource")
			return err
		}

		return nil
	}

	return nil
}

func mergePlans(base v1alpha1.RpaasPlanSpec, override v1alpha1.RpaasPlanSpec) (v1alpha1.RpaasPlanSpec, error) {
	baseData, err := json.Marshal(base)
	if err != nil {
		return base, err
	}
	overrideData, err := json.Marshal(override)
	if err != nil {
		return base, err
	}
	patch, err := jsonmergepatch.CreateThreeWayJSONMergePatch(baseData, overrideData, baseData)
	if err != nil {
		return base, err
	}
	merged, err := jsonpatch.MergePatch(baseData, patch)
	if err != nil {
		return base, err
	}
	err = json.Unmarshal(merged, &base)
	if err != nil {
		return base, err
	}
	return base, nil
}

func (r *ReconcileRpaasInstance) reconcileConfigMap(configMap *corev1.ConfigMap) error {
	found := &corev1.ConfigMap{}
	err := r.client.Get(context.TODO(), types.NamespacedName{Name: configMap.ObjectMeta.Name, Namespace: configMap.ObjectMeta.Namespace}, found)
	if err != nil {
		if !k8sErrors.IsNotFound(err) {
			logrus.Errorf("Failed to get configMap: %v", err)
			return err
		}
		err = r.client.Create(context.TODO(), configMap)
		if err != nil {
			logrus.Errorf("Failed to create configMap: %v", err)
			return err
		}
		return nil
	}

	configMap.ObjectMeta.ResourceVersion = found.ObjectMeta.ResourceVersion
	err = r.client.Update(context.TODO(), configMap)
	if err != nil {
		logrus.Errorf("Failed to update configMap: %v", err)
	}
	return err
}

func (r *ReconcileRpaasInstance) reconcileNginx(nginx *nginxV1alpha1.Nginx) error {
	found := &nginxV1alpha1.Nginx{}
	err := r.client.Get(context.TODO(), types.NamespacedName{Name: nginx.ObjectMeta.Name, Namespace: nginx.ObjectMeta.Namespace}, found)
	if err != nil {
		if !k8sErrors.IsNotFound(err) {
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

	nginx.ObjectMeta.ResourceVersion = found.ObjectMeta.ResourceVersion
	err = r.client.Update(context.TODO(), nginx)
	if err != nil {
		logrus.Errorf("Failed to update nginx CR: %v", err)
	}
	return err
}

func (r *ReconcileRpaasInstance) renderTemplate(instance *v1alpha1.RpaasInstance, plan *v1alpha1.RpaasPlan) (string, error) {
	blocks, err := r.getConfigurationBlocks(instance, plan)
	if err != nil {
		return "", err
	}
	if err = r.updateLocationValues(instance); err != nil {
		return "", err
	}
	data := nginx.ConfigurationData{
		Instance: instance,
		Config:   &plan.Spec.Config,
	}
	return nginx.NewRpaasConfigurationRenderer(blocks).Render(data)
}

func (r *ReconcileRpaasInstance) getConfigurationBlocks(instance *v1alpha1.RpaasInstance, plan *v1alpha1.RpaasPlan) (nginx.ConfigurationBlocks, error) {
	var blocks nginx.ConfigurationBlocks

	if plan.Spec.Template != nil {
		mainBlock, err := util.GetValue(context.TODO(), r.client, "", plan.Spec.Template)
		if err != nil {
			return blocks, err
		}

		blocks.MainBlock = mainBlock
	}

	for blockType, blockValue := range instance.Spec.Blocks {
		content, err := util.GetValue(context.TODO(), r.client, instance.Namespace, &blockValue)
		if err != nil {
			return blocks, err
		}

		switch blockType {
		case v1alpha1.BlockTypeRoot:
			blocks.RootBlock = content
		case v1alpha1.BlockTypeHTTP:
			blocks.HttpBlock = content
		case v1alpha1.BlockTypeServer:
			blocks.ServerBlock = content
		case v1alpha1.BlockTypeLuaServer:
			blocks.LuaServerBlock = content
		case v1alpha1.BlockTypeLuaWorker:
			blocks.LuaWorkerBlock = content
		}
	}

	return blocks, nil
}

func (r *ReconcileRpaasInstance) updateLocationValues(instance *v1alpha1.RpaasInstance) error {
	for _, location := range instance.Spec.Locations {
		if location.Content == nil {
			continue
		}

		content, err := util.GetValue(context.TODO(), r.client, instance.Namespace, location.Content)
		if err != nil {
			return err
		}

		location.Content.Value = content
	}
	return nil
}

func (r *ReconcileRpaasInstance) listConfigs(instance *v1alpha1.RpaasInstance) (*corev1.ConfigMapList, error) {
	configList := &corev1.ConfigMapList{}
	listOptions := &client.ListOptions{Namespace: instance.ObjectMeta.Namespace}
	labelSelector := fmt.Sprintf("instance=%s,type=config", instance.Name)

	if err := listOptions.SetLabelSelector(labelSelector); err != nil {
		logrus.Errorf("Failed to query nginx configs: %v", err)
		return nil, err
	}

	err := r.client.List(context.TODO(), listOptions, configList)
	return configList, err
}

func (r *ReconcileRpaasInstance) deleteOldConfig(instance *v1alpha1.RpaasInstance, configList *corev1.ConfigMapList) error {
	list := configList.Items
	sort.Slice(list, func(i, j int) bool {
		return list[i].ObjectMeta.CreationTimestamp.String() < list[j].ObjectMeta.CreationTimestamp.String()
	})
	if err := r.client.Delete(context.TODO(), &list[0]); err != nil {
		return err
	}
	return nil
}

func newConfigMap(instance *v1alpha1.RpaasInstance, renderedTemplate string) *corev1.ConfigMap {
	hash := fmt.Sprintf("%x", sha256.Sum256([]byte(renderedTemplate)))
	return &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      fmt.Sprintf("%s-config-%s", instance.Name, hash[:10]),
			Namespace: instance.Namespace,
			Labels: map[string]string{
				"type":     "config",
				"instance": instance.Name,
			},
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
	var cacheConfig nginxV1alpha1.NginxCacheSpec
	if v1alpha1.BoolValue(plan.Spec.Config.CacheEnabled) {
		cacheConfig.Path = plan.Spec.Config.CachePath
		cacheConfig.InMemory = true
		cacheMaxSize, err := k8sResources.ParseQuantity(plan.Spec.Config.CacheSize)
		if err == nil && !cacheMaxSize.IsZero() {
			cacheConfig.Size = &cacheMaxSize
		}
	}
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
			Resources:       plan.Spec.Resources,
			Service:         instance.Spec.Service,
			HealthcheckPath: "/_nginx_healthcheck",
			ExtraFiles:      instance.Spec.ExtraFiles,
			Certificates:    instance.Spec.Certificates,
			Cache:           cacheConfig,
			PodTemplate:     instance.Spec.PodTemplate,
		},
	}
}

func newHPA(instance v1alpha1.RpaasInstance, nginx nginxV1alpha1.Nginx) autoscalingv2beta2.HorizontalPodAutoscaler {
	var metrics []autoscalingv2beta2.MetricSpec

	if instance.Spec.Autoscale.TargetCPUUtilizationPercentage != nil {
		metrics = append(metrics, autoscalingv2beta2.MetricSpec{
			Type: autoscalingv2beta2.ResourceMetricSourceType,
			Resource: &autoscalingv2beta2.ResourceMetricSource{
				Name: corev1.ResourceCPU,
				Target: autoscalingv2beta2.MetricTarget{
					Type:               autoscalingv2beta2.UtilizationMetricType,
					AverageUtilization: instance.Spec.Autoscale.TargetCPUUtilizationPercentage,
				},
			},
		})
	}

	if instance.Spec.Autoscale.TargetMemoryUtilizationPercentage != nil {
		metrics = append(metrics, autoscalingv2beta2.MetricSpec{
			Type: autoscalingv2beta2.ResourceMetricSourceType,
			Resource: &autoscalingv2beta2.ResourceMetricSource{
				Name: corev1.ResourceMemory,
				Target: autoscalingv2beta2.MetricTarget{
					Type:               autoscalingv2beta2.UtilizationMetricType,
					AverageUtilization: instance.Spec.Autoscale.TargetMemoryUtilizationPercentage,
				},
			},
		})
	}

	minReplicas := instance.Spec.Autoscale.MinReplicas
	if minReplicas == nil && instance.Spec.Replicas != nil {
		minReplicas = instance.Spec.Replicas
	}

	return autoscalingv2beta2.HorizontalPodAutoscaler{
		TypeMeta: metav1.TypeMeta{
			Kind:       "HorizontalPodAutoscaler",
			APIVersion: "autoscaling/v2beta2",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      instance.Name,
			Namespace: instance.Namespace,
			OwnerReferences: []metav1.OwnerReference{
				*metav1.NewControllerRef(&instance, schema.GroupVersionKind{
					Group:   v1alpha1.SchemeGroupVersion.Group,
					Version: v1alpha1.SchemeGroupVersion.Version,
					Kind:    "RpaasInstance",
				}),
			},
		},
		Spec: autoscalingv2beta2.HorizontalPodAutoscalerSpec{
			ScaleTargetRef: autoscalingv2beta2.CrossVersionObjectReference{
				APIVersion: "nginx.tsuru.io/v1alpha1",
				Kind:       "Nginx",
				Name:       nginx.Name,
			},
			MinReplicas: minReplicas,
			MaxReplicas: instance.Spec.Autoscale.MaxReplicas,
			Metrics:     metrics,
		},
	}
}

func shouldDeleteOldConfig(instance *v1alpha1.RpaasInstance, configList *corev1.ConfigMapList) bool {
	limit := defaultConfigHistoryLimit

	if instance.Spec.ConfigHistoryLimit != nil {
		configLimit := *instance.Spec.ConfigHistoryLimit
		if configLimit > 0 {
			limit = configLimit
		}
	}

	listSize := len(configList.Items)
	return listSize > limit
}

func mergeInstance(base v1alpha1.RpaasInstanceSpec, override v1alpha1.RpaasInstanceSpec) (merged v1alpha1.RpaasInstanceSpec, err error) {
	configs := []func(*mergo.Config){
		mergo.WithOverride,
		mergo.WithAppendSlice,
	}

	if err = mergo.Merge(&merged, base, configs...); err != nil {
		return
	}

	if err = mergo.Merge(&merged, override, configs...); err != nil {
		return
	}

	return
}
