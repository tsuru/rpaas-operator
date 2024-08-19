// Copyright 2020 tsuru authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package controllers

import (
	"context"
	"fmt"
	"reflect"
	"sort"
	"strings"
	"time"

	cmv1 "github.com/cert-manager/cert-manager/pkg/apis/certmanager/v1"
	"github.com/go-logr/logr"
	nginxv1alpha1 "github.com/tsuru/nginx-operator/api/v1alpha1"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	k8sErrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"github.com/tsuru/rpaas-operator/api/v1alpha1"
	"github.com/tsuru/rpaas-operator/internal/controllers/certificates"
	"github.com/tsuru/rpaas-operator/internal/pkg/rpaas/nginx"
)

// RpaasInstanceReconciler reconciles a RpaasInstance object
type RpaasInstanceReconciler struct {
	client.Client
	Log               logr.Logger
	SystemRateLimiter SystemRolloutRateLimiter
	EventRecorder     record.EventRecorder
}

// +kubebuilder:rbac:groups="",resources=configmaps;secrets;services,verbs=get;list;watch;create;update;delete
// +kubebuilder:rbac:groups="",resources=events,verbs=create;update;patch

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

// +kubebuilder:rbac:groups=keda.sh,resources=scaledobjects,verbs=get;list;watch;create;update;delete

func (r *RpaasInstanceReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {

	instance, err := r.getRpaasInstance(ctx, req.NamespacedName)
	if k8sErrors.IsNotFound(err) {
		return reconcile.Result{}, nil
	}

	logger := r.Log.WithName("Reconcile").
		WithValues("RpaasInstance", types.NamespacedName{Name: instance.Name, Namespace: instance.Namespace})

	if err != nil {
		return reconcile.Result{}, err
	}

	instanceHash, err := generateSpecHash(&instance.Spec)
	if err != nil {
		return reconcile.Result{}, err
	}

	systemRollout := isSystemRollout(instanceHash, instance)

	var rolloutAllowed bool
	var reservation SystemRolloutReservation
	if systemRollout {
		rolloutAllowed, reservation = r.SystemRateLimiter.Reserve()

		if !rolloutAllowed {
			return ctrl.Result{
				Requeue:      true,
				RequeueAfter: time.Minute,
			}, nil
		}
	} else {
		reservation = NoopReservation()
	}

	if s := instance.Spec.Suspend; s != nil && *s {
		r.EventRecorder.Eventf(instance, corev1.EventTypeWarning, "RpaasInstanceSuspended", "no modifications will be done by RPaaS controller")
		return reconcile.Result{Requeue: true}, nil
	}

	planName := types.NamespacedName{
		Name:      instance.Spec.PlanName,
		Namespace: instance.Namespace,
	}
	if instance.Spec.PlanNamespace != "" {
		planName.Namespace = instance.Spec.PlanNamespace
	}

	plan := &v1alpha1.RpaasPlan{}
	err = r.Client.Get(ctx, planName, plan)
	if err != nil {
		return reconcile.Result{}, err
	}

	instanceMergedWithFlavors, err := r.mergeWithFlavors(ctx, instance.DeepCopy())
	if err != nil {
		return reconcile.Result{}, nil
	}

	if instanceMergedWithFlavors.Spec.PlanTemplate != nil {
		plan.Spec, err = mergePlans(plan.Spec, *instanceMergedWithFlavors.Spec.PlanTemplate)
		if err != nil {
			return reconcile.Result{}, err
		}
	}

	certManagerCertificates, err := certificates.ReconcileCertManager(ctx, r.Client, instance, instanceMergedWithFlavors)
	if err != nil {
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

	changes := map[string]bool{}

	configMap := newConfigMap(instanceMergedWithFlavors, rendered)
	changes["configMap"], err = r.reconcileConfigMap(ctx, configMap)
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

	certificateSecrets, err := certificates.ListCertificateSecrets(ctx, r.Client, instanceMergedWithFlavors)
	if err != nil {
		return reconcile.Result{}, err
	}

	// Nginx CRD
	nginx := newNginx(newNginxOptions{
		instanceMergedWithFlavors: instanceMergedWithFlavors,
		plan:                      plan,
		configMap:                 configMap,
		certManagerCertificates:   certManagerCertificates,
		certificateSecrets:        certificateSecrets,
	})
	changes["nginx"], err = r.reconcileNginx(ctx, instanceMergedWithFlavors, nginx)
	if err != nil {
		return ctrl.Result{}, err
	}

	// Session Resumption
	changes["sessionResumption"], err = r.reconcileTLSSessionResumption(ctx, instanceMergedWithFlavors)
	if err != nil {
		return ctrl.Result{}, err
	}

	// HPA
	changes["hpa"], err = r.reconcileHPA(ctx, instanceMergedWithFlavors, nginx)
	if err != nil {
		return ctrl.Result{}, err
	}

	// PDB
	changes["pdb"], err = r.reconcilePDB(ctx, instanceMergedWithFlavors, nginx)
	if err != nil {
		return ctrl.Result{}, err
	}

	if listOfChanges := getChangesList(changes); len(listOfChanges) > 0 {
		if systemRollout {
			msg := fmt.Sprintf("RPaaS controller has updated these resources: %s to ensure system consistency", strings.Join(listOfChanges, ", "))
			logger.Info(msg)
			r.EventRecorder.Event(instance, corev1.EventTypeWarning, "RpaasInstanceSystemRolloutApplied", msg)
		}
	} else {
		reservation.Cancel()
	}

	if err = r.refreshStatus(ctx, instance, instanceHash, nginx); err != nil {
		return ctrl.Result{}, err
	}

	return ctrl.Result{}, nil
}

func getChangesList(changes map[string]bool) []string {
	changed := []string{}

	for k, v := range changes {
		if v {
			changed = append(changed, k)
		}
	}
	sort.Strings(changed)
	return changed
}

func (r *RpaasInstanceReconciler) refreshStatus(ctx context.Context, instance *v1alpha1.RpaasInstance, instanceHash string, newNginx *nginxv1alpha1.Nginx) error {
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
		RevisionHash:              instanceHash,
		ObservedGeneration:        instance.Generation,
		WantedNginxRevisionHash:   newHash,
		ObservedNginxRevisionHash: existingHash,
		NginxUpdated:              newHash == existingHash,
		ExternalAddresses:         externalAddresssesFromNginx(existingNginx),
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

func isSystemRollout(currentHash string, instance *v1alpha1.RpaasInstance) bool {
	return instance.Status.RevisionHash != "" && currentHash == instance.Status.RevisionHash
}

func (r *RpaasInstanceReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&v1alpha1.RpaasInstance{}).
		Owns(&corev1.ConfigMap{}).
		Owns(&corev1.Secret{}).
		Owns(&batchv1.CronJob{}).
		Owns(&nginxv1alpha1.Nginx{}).
		Owns(&cmv1.Certificate{}).
		Complete(r)
}
