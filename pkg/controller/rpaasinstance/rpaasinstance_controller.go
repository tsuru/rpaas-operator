// Copyright 2019 tsuru authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package rpaasinstance

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"sort"

	jsonpatch "github.com/evanphx/json-patch"
	"github.com/sirupsen/logrus"
	nginxV1alpha1 "github.com/tsuru/nginx-operator/pkg/apis/nginx/v1alpha1"
	"github.com/tsuru/rpaas-operator/pkg/apis/extensions/v1alpha1"
	extensionsv1alpha1 "github.com/tsuru/rpaas-operator/pkg/apis/extensions/v1alpha1"
	"github.com/tsuru/rpaas-operator/pkg/util"
	"github.com/tsuru/rpaas-operator/rpaas/nginx"
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
	instance := &v1alpha1.RpaasInstance{}
	err := r.client.Get(context.TODO(), request.NamespacedName, instance)
	if err != nil && k8sErrors.IsNotFound(err) {
		reqLogger.Info("Nothing to do due the RpaasInstance was removed")
		return reconcile.Result{}, nil
	}

	if err != nil {
		return reconcile.Result{}, err
	}
	plan := &v1alpha1.RpaasPlan{}
	err = r.client.Get(context.TODO(), types.NamespacedName{Name: instance.Spec.PlanName}, plan)
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
	err = r.reconcileNginx(nginx)
	return reconcile.Result{}, err
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
