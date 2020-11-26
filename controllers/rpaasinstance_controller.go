// Copyright 2020 tsuru authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package controllers

import (
	"context"
	"fmt"
	"sort"

	"github.com/go-logr/logr"
	nginxv1alpha1 "github.com/tsuru/nginx-operator/api/v1alpha1"
	extensionsv1alpha1 "github.com/tsuru/rpaas-operator/api/v1alpha1"
	"github.com/tsuru/rpaas-operator/internal/pkg/rpaas/nginx"
	batchv1beta1 "k8s.io/api/batch/v1beta1"
	corev1 "k8s.io/api/core/v1"
	k8sErrors "k8s.io/apimachinery/pkg/api/errors"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

// RpaasInstanceReconciler reconciles a RpaasInstance object
type RpaasInstanceReconciler struct {
	client.Client
	Log                 logr.Logger
	Scheme              *runtime.Scheme
	RolloutNginxEnabled bool
}

// +kubebuilder:rbac:groups=extensions.tsuru.io,resources=rpaasinstances,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=extensions.tsuru.io,resources=rpaasinstances/status,verbs=get;update;patch

func (r *RpaasInstanceReconciler) Reconcile(req ctrl.Request) (ctrl.Result, error) {
	ctx := context.Background()
	_ = r.Log.WithValues("rpaasinstance", req.NamespacedName)

	instance, err := r.getRpaasInstance(ctx, req.NamespacedName)
	if err != nil && k8serrors.IsNotFound(err) {
		_, err = r.reconcileDedicatedPorts(ctx, nil, 0)
		return reconcile.Result{}, err
	}

	if err != nil {
		return reconcile.Result{}, err
	}

	planName := types.NamespacedName{
		Name:      instance.Spec.PlanName,
		Namespace: instance.Namespace,
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

	dedicatedPorts, err := r.reconcileDedicatedPorts(ctx, instance, 3)
	if err != nil {
		return reconcile.Result{}, err
	}

	if len(dedicatedPorts) == 0 {
		// nginx-operator will allocate http and https ports by hostNetwork setting
		instance.Spec.PodTemplate.Ports = []corev1.ContainerPort{
			{
				Name:          nginx.PortNameManagement,
				ContainerPort: nginx.DefaultManagePort,
				Protocol:      corev1.ProtocolTCP,
			},
		}
	} else if len(dedicatedPorts) == 3 {
		sort.Ints(dedicatedPorts)
		instance.Spec.PodTemplate.Ports = []corev1.ContainerPort{
			{
				Name:          nginx.PortNameHTTP,
				ContainerPort: int32(dedicatedPorts[0]),
				Protocol:      corev1.ProtocolTCP,
			},
			{
				Name:          nginx.PortNameHTTPS,
				ContainerPort: int32(dedicatedPorts[1]),
				Protocol:      corev1.ProtocolTCP,
			},
			{
				Name:          nginx.PortNameManagement,
				ContainerPort: int32(dedicatedPorts[2]),
				Protocol:      corev1.ProtocolTCP,
			},
		}
	}

	rendered, err := r.renderTemplate(ctx, instance, plan)
	if err != nil {
		return reconcile.Result{}, err
	}

	configMap := newConfigMap(instance, rendered)
	err = r.reconcileConfigMap(ctx, configMap)
	if err != nil {
		return reconcile.Result{}, err
	}

	configList, err := r.listConfigs(ctx, instance)
	if err != nil {
		return reconcile.Result{}, err
	}

	if shouldDeleteOldConfig(instance, configList) {
		if err = r.deleteOldConfig(ctx, instance, configList); err != nil {
			return ctrl.Result{}, err
		}
	}

	nginx := newNginx(instance, plan, configMap)
	if err = r.reconcileNginx(ctx, instance, nginx); err != nil {
		return ctrl.Result{}, err
	}

	if err = r.reconcileTLSSessionResumption(ctx, instance); err != nil {
		return ctrl.Result{}, err
	}

	if err = r.reconcileCacheSnapshot(ctx, instance, plan); err != nil {
		return ctrl.Result{}, err
	}

	if err = r.reconcileHPA(ctx, instance); err != nil {
		return ctrl.Result{}, err
	}

	if err = r.refreshStatus(ctx, instance, nginx); err != nil {
		return ctrl.Result{}, err
	}

	if err = r.resetRolloutOnce(ctx, instance); err != nil {
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

	if instance.Status.ObservedGeneration == instance.Generation &&
		instance.Status.WantedNginxRevisionHash == newHash &&
		instance.Status.ObservedNginxRevisionHash == existingHash {
		return nil
	}

	instance.Status.ObservedGeneration = instance.Generation
	instance.Status.WantedNginxRevisionHash = newHash
	instance.Status.ObservedNginxRevisionHash = existingHash

	err = r.Client.Status().Update(ctx, instance)
	if err != nil {
		return fmt.Errorf("failed to update rpaas instance status: %v", err)
	}

	return nil
}

func (r *RpaasInstanceReconciler) resetRolloutOnce(ctx context.Context, instance *extensionsv1alpha1.RpaasInstance) error {
	if !instance.Spec.RolloutNginxOnce {
		return nil
	}

	var rawInstance extensionsv1alpha1.RpaasInstance
	if err := r.Client.Get(ctx, types.NamespacedName{
		Name:      instance.Name,
		Namespace: instance.Namespace,
	}, &rawInstance); err != nil {
		return err
	}
	if !rawInstance.Spec.RolloutNginxOnce {
		return nil
	}

	rawInstance.Spec.RolloutNginxOnce = false
	err := r.Client.Update(ctx, &rawInstance)
	if err != nil {
		return fmt.Errorf("failed to update rpaas instance rollout once: %v", err)
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
		Complete(r)
}
