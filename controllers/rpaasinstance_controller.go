// Copyright 2020 tsuru authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package controllers

import (
	"context"
	"fmt"
	"reflect"
	"strconv"

	cmv1 "github.com/cert-manager/cert-manager/pkg/apis/certmanager/v1"
	"github.com/go-logr/logr"
	nginxv1alpha1 "github.com/tsuru/nginx-operator/api/v1alpha1"
	batchv1beta1 "k8s.io/api/batch/v1beta1"
	corev1 "k8s.io/api/core/v1"
	k8sErrors "k8s.io/apimachinery/pkg/api/errors"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"github.com/tsuru/rpaas-operator/api/v1alpha1"
	extensionsv1alpha1 "github.com/tsuru/rpaas-operator/api/v1alpha1"
	"github.com/tsuru/rpaas-operator/internal/controllers/certificates"
	"github.com/tsuru/rpaas-operator/internal/pkg/rpaas/nginx"
	"github.com/tsuru/rpaas-operator/internal/registry"
)

// RpaasInstanceReconciler reconciles a RpaasInstance object
type RpaasInstanceReconciler struct {
	client.Client
	Log                 logr.Logger
	Scheme              *runtime.Scheme
	RolloutNginxEnabled bool
	PortRangeMin        int32
	PortRangeMax        int32
	ImageMetadata       registry.ImageMetadata
}

// +kubebuilder:rbac:groups="",resources=configmaps;persistentvolumeclaims;secrets;services,verbs=get;list;watch;create;update;delete
// +kubebuilder:rbac:groups=batch,resources=cronjobs,verbs=get;list;watch;create;update;delete
// +kubebuilder:rbac:groups=autoscaling,resources=horizontalpodautoscalers,verbs=get;list;watch;create;update;delete
// +kubebuilder:rbac:groups=policy,resources=poddisruptionbudgets,verbs=get;list;watch;create;update;delete

// +kubebuilder:rbac:groups=cert-manager.io,resources=certificates,verbs=get;list;watch;create;update;delete
// +kubebuilder:rbac:groups=cert-manager.io,resources=clusterissuers;issuers,verbs=get;list;watch

// +kubebuilder:rbac:groups=nginx.tsuru.io,resources=nginxes,verbs=get;list;watch;create;update

// +kubebuilder:rbac:groups=extensions.tsuru.io,resources=rpaasflavors,verbs=get;list;watch
// +kubebuilder:rbac:groups=extensions.tsuru.io,resources=rpaasplans,verbs=get;list;watch
// +kubebuilder:rbac:groups=extensions.tsuru.io,resources=rpaasinstances,verbs=get;list;watch;update;patch
// +kubebuilder:rbac:groups=extensions.tsuru.io,resources=rpaasinstances/status,verbs=get;update;patch

func (r *RpaasInstanceReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	l := r.Log.WithValues("rpaasinstance", req.NamespacedName)

	instance, err := r.getRpaasInstance(ctx, req.NamespacedName)
	if k8serrors.IsNotFound(err) {
		return reconcile.Result{}, nil
	}

	if err != nil {
		return reconcile.Result{}, err
	}

	if s, ok := instance.Annotations[skipReconcileLabel]; ok {
		if skipped, _ := strconv.ParseBool(s); skipped {
			l.Info(fmt.Sprintf("Skipping reconciliation as %s=true annotation was found in the resource", skipReconcileLabel))
			return reconcile.Result{Requeue: true}, nil
		}
	}

	planName := types.NamespacedName{
		Name:      instance.Spec.PlanName,
		Namespace: instance.Namespace,
	}
	if instance.Spec.PlanNamespace != "" {
		planName.Namespace = instance.Spec.PlanNamespace
	}

	plan := &extensionsv1alpha1.RpaasPlan{}
	err = r.Client.Get(ctx, planName, plan)
	if err != nil {
		return reconcile.Result{}, err
	}

	if instance.Spec.PlanTemplate != nil {
		plan.Spec, err = mergePlans(plan.Spec, *instance.Spec.PlanTemplate)
		if err != nil {
			return reconcile.Result{}, err
		}
	}

	instanceMergedWithFlavors, err := r.mergeWithFlavors(ctx, instance.DeepCopy())
	if err != nil {
		return reconcile.Result{}, nil
	}

	if err = certificates.ReconcileDynamicCertificates(ctx, r.Client, instance, instanceMergedWithFlavors); err != nil {
		return reconcile.Result{}, err
	}

	instanceMergedWithFlavors.Spec.PodTemplate.Ports = []corev1.ContainerPort{
		{
			Name:          nginx.PortNameManagement,
			ContainerPort: nginx.DefaultManagePort,
			Protocol:      corev1.ProtocolTCP,
		},
	}

	if instanceMergedWithFlavors.Spec.ProxyProtocol {
		instanceMergedWithFlavors.Spec.PodTemplate.Ports = append(instanceMergedWithFlavors.Spec.PodTemplate.Ports, corev1.ContainerPort{
			Name:          nginx.PortNameProxyProtocolHTTP,
			ContainerPort: nginx.DefaultProxyProtocolHTTPPort,
			Protocol:      corev1.ProtocolTCP,
		})

		instanceMergedWithFlavors.Spec.PodTemplate.Ports = append(instanceMergedWithFlavors.Spec.PodTemplate.Ports, corev1.ContainerPort{
			Name:          nginx.PortNameProxyProtocolHTTPS,
			ContainerPort: nginx.DefaultProxyProtocolHTTPSPort,
			Protocol:      corev1.ProtocolTCP,
		})
	}

	rendered, err := r.renderTemplate(ctx, instanceMergedWithFlavors, plan)
	if err != nil {
		return reconcile.Result{}, err
	}

	configMap := newConfigMap(instanceMergedWithFlavors, rendered)
	err = r.reconcileConfigMap(ctx, configMap)
	if err != nil {
		return reconcile.Result{}, err
	}

	configList, err := r.listConfigs(ctx, instanceMergedWithFlavors)
	if err != nil {
		return reconcile.Result{}, err
	}

	if shouldDeleteOldConfig(instanceMergedWithFlavors, configList) {
		if err = r.deleteOldConfig(ctx, instanceMergedWithFlavors, configList); err != nil {
			return ctrl.Result{}, err
		}
	}

	nginx := newNginx(instanceMergedWithFlavors, plan, configMap)
	if err = r.reconcileNginx(ctx, instanceMergedWithFlavors, nginx); err != nil {
		return ctrl.Result{}, err
	}

	if err = r.reconcileTLSSessionResumption(ctx, instanceMergedWithFlavors); err != nil {
		return ctrl.Result{}, err
	}

	if err = r.reconcileCacheSnapshot(ctx, instanceMergedWithFlavors, plan); err != nil {
		return ctrl.Result{}, err
	}

	if err = r.reconcileHPA(ctx, instanceMergedWithFlavors, nginx); err != nil {
		return ctrl.Result{}, err
	}

	if err = r.reconcilePDB(ctx, instanceMergedWithFlavors, nginx); err != nil {
		return ctrl.Result{}, err
	}

	if err = r.refreshStatus(ctx, instance, nginx); err != nil {
		return ctrl.Result{}, err
	}

	return ctrl.Result{}, nil
}

func (r *RpaasInstanceReconciler) refreshStatus(ctx context.Context, instance *extensionsv1alpha1.RpaasInstance, newNginx *nginxv1alpha1.Nginx) error {
	existingNginx, err := r.getNginx(ctx, instance)
	if err != nil && !k8sErrors.IsNotFound(err) {
		return err
	}
	newHash, err := generateNginxHash(newNginx)
	if err != nil {
		return err
	}

	existingHash, err := generateNginxHash(existingNginx)
	if err != nil {
		return err
	}

	newStatus := v1alpha1.RpaasInstanceStatus{
		ObservedGeneration:        instance.Generation,
		WantedNginxRevisionHash:   newHash,
		ObservedNginxRevisionHash: existingHash,
		NginxUpdated:              newHash == existingHash,
	}

	if existingNginx != nil {
		newStatus.CurrentReplicas = existingNginx.Status.CurrentReplicas
		newStatus.PodSelector = existingNginx.Status.PodSelector
	}

	if reflect.DeepEqual(instance.Status, newStatus) {
		return nil
	}

	instance.Status = newStatus
	err = r.Client.Status().Update(ctx, instance)
	if err != nil {
		return fmt.Errorf("failed to update rpaas instance status: %v", err)
	}

	return nil
}

func (r *RpaasInstanceReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&extensionsv1alpha1.RpaasInstance{}).
		Owns(&corev1.ConfigMap{}).
		Owns(&corev1.Secret{}).
		Owns(&corev1.PersistentVolumeClaim{}).
		Owns(&batchv1beta1.CronJob{}).
		Owns(&nginxv1alpha1.Nginx{}).
		Owns(&cmv1.Certificate{}).
		Complete(r)
}
