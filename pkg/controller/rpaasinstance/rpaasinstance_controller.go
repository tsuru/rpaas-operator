// Copyright 2019 tsuru authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package rpaasinstance

import (
	"bytes"
	"context"
	"crypto/sha256"
	"fmt"
	"reflect"
	"sort"
	"strings"
	"text/template"

	"github.com/imdario/mergo"
	"github.com/sirupsen/logrus"
	nginxv1alpha1 "github.com/tsuru/nginx-operator/pkg/apis/nginx/v1alpha1"
	"github.com/tsuru/rpaas-operator/config"
	"github.com/tsuru/rpaas-operator/internal/pkg/rpaas/nginx"
	"github.com/tsuru/rpaas-operator/pkg/apis/extensions/v1alpha1"
	extensionsv1alpha1 "github.com/tsuru/rpaas-operator/pkg/apis/extensions/v1alpha1"
	"github.com/tsuru/rpaas-operator/pkg/util"
	"github.com/willf/bitset"
	autoscalingv2beta2 "k8s.io/api/autoscaling/v2beta2"
	batchv1 "k8s.io/api/batch/v1"
	batchv1beta1 "k8s.io/api/batch/v1beta1"
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
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"
)

const (
	defaultConfigHistoryLimit     = 10
	defaultCacheSnapshotCronImage = "bitnami/kubectl:latest"
	defaultCacheSnapshotSchedule  = "* * * * *"
	defaultPortAllocationResource = "default"
	volumeTeamLabel               = "tsuru.io/volume-team"

	cacheSnapshotCronJobSuffix = "-snapshot-cron-job"
	cacheSnapshotVolumeSuffix  = "-snapshot-volume"

	cacheSnapshotMountPoint = "/var/cache/cache-snapshot"

	rsyncCommandPodToPVC = "rsync -avz --recursive --delete --temp-dir=${CACHE_SNAPSHOT_MOUNTPOINT}/temp ${CACHE_PATH}/nginx ${CACHE_SNAPSHOT_MOUNTPOINT}"
	rsyncCommandPVCToPod = "rsync -avz --recursive --delete --temp-dir=${CACHE_PATH}/nginx_tmp ${CACHE_SNAPSHOT_MOUNTPOINT}/nginx ${CACHE_PATH}"
)

var (
	defaultCacheSnapshotCmdPodToPVC = []string{
		"/bin/bash",
		"-c",
		`pods=($(kubectl -n ${SERVICE_NAME} get pod -l rpaas.extensions.tsuru.io/service-name=${SERVICE_NAME} -l rpaas.extensions.tsuru.io/instance-name=${INSTANCE_NAME} --field-selector status.phase=Running -o=jsonpath='{.items[*].metadata.name}'));
		echo "${SERVICE_NAME}-${INSTANCE_NAME}-snapshot-cronjob: ${pods}"
for pod in ${pods[@]}; do
	kubectl -n ${SERVICE_NAME} exec ${pod} -- ${POD_CMD};
	if [[ $? == 0 ]]; then
		exit 0;
	fi
done
echo "${SERVICE_NAME}-${INSTANCE_NAME}-snapshot-cronjob: No pods found";
exit 1
`}

	defaultCacheSnapshotCmdPVCToPod = []string{
		"/bin/bash",
		"-c",
		`
mkdir -p ${CACHE_SNAPSHOT_MOUNTPOINT}/temp;
mkdir -p ${CACHE_SNAPSHOT_MOUNTPOINT}/nginx;
mkdir -p ${CACHE_PATH}/nginx_tmp;
${POD_CMD}
`}
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

	err = c.Watch(&source.Kind{Type: &corev1.PersistentVolumeClaim{}}, &handler.EnqueueRequestForOwner{
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
	if err != nil {
		return err
	}

	err = c.Watch(&source.Kind{Type: &batchv1beta1.CronJob{}}, &handler.EnqueueRequestForOwner{
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

	ctx := context.TODO()

	instance, err := r.getRpaasInstance(ctx, request.NamespacedName)
	if err != nil && k8sErrors.IsNotFound(err) {
		_, err = r.reconcilePorts(ctx, nil, 0)
		return reconcile.Result{}, err
	}

	if err != nil {
		return reconcile.Result{}, err
	}

	planName := types.NamespacedName{
		Name:      instance.Spec.PlanName,
		Namespace: instance.Namespace,
	}
	plan := &v1alpha1.RpaasPlan{}
	err = r.client.Get(ctx, planName, plan)
	if err != nil {
		return reconcile.Result{}, err
	}

	if instance.Spec.PlanTemplate != nil {
		plan.Spec, err = mergePlans(plan.Spec, *instance.Spec.PlanTemplate)
		if err != nil {
			return reconcile.Result{}, err
		}
	}

	ports, err := r.reconcilePorts(ctx, instance, 3)
	if err != nil {
		return reconcile.Result{}, err
	}
	if len(ports) == 3 {
		sort.Ints(ports)
		instance.Spec.PodTemplate.Ports = []corev1.ContainerPort{
			{
				Name:          nginx.PortNameHTTP,
				ContainerPort: int32(ports[0]),
				Protocol:      corev1.ProtocolTCP,
			},
			{
				Name:          nginx.PortNameHTTPS,
				ContainerPort: int32(ports[1]),
				Protocol:      corev1.ProtocolTCP,
			},
			{
				Name:          nginx.PortNameManagement,
				ContainerPort: int32(ports[2]),
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
			return reconcile.Result{}, err
		}
	}

	nginx := newNginx(instance, plan, configMap)

	if err = r.reconcileNginx(ctx, nginx); err != nil {
		return reconcile.Result{}, err
	}

	if err = r.reconcileCacheSnapshot(ctx, instance, plan); err != nil {
		return reconcile.Result{}, err
	}

	if err = r.reconcileHPA(ctx, instance, nginx); err != nil {
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

	mergedInstance, err := r.mergeInstanceWithFlavors(ctx, instance.DeepCopy())
	if err != nil {
		return nil, err
	}

	if err = renderCustomValues(mergedInstance); err != nil {
		return nil, err
	}

	return mergedInstance, nil
}

func (r *ReconcileRpaasInstance) mergeInstanceWithFlavors(ctx context.Context, instance *v1alpha1.RpaasInstance) (*v1alpha1.RpaasInstance, error) {
	logger := log.WithName("mergeInstanceWithFlavors").
		WithValues("RpaasInstance", types.NamespacedName{Name: instance.Name, Namespace: instance.Namespace})

	defaultFlavors, err := r.listDefaultFlavors(ctx, instance)
	if err != nil {
		return nil, err
	}

	for _, defaultFlavor := range defaultFlavors {
		if err := mergeInstanceWithFlavor(instance, defaultFlavor); err != nil {
			return nil, err
		}
	}

	for _, flavorName := range instance.Spec.Flavors {
		flavorObjectKey := types.NamespacedName{
			Name:      flavorName,
			Namespace: instance.Namespace,
		}

		logger = logger.WithValues("RpaasFlavor", flavorObjectKey)
		logger.V(4).Info("Getting RpaasFlavor resource")

		var flavor v1alpha1.RpaasFlavor
		if err := r.client.Get(ctx, flavorObjectKey, &flavor); err != nil {
			logger.Error(err, "Unable to get the RpaasFlavor resource")
			return nil, err
		}

		if flavor.Spec.Default {
			continue
		}

		if err := mergeInstanceWithFlavor(instance, flavor); err != nil {
			return nil, err
		}

	}

	return instance, nil
}

func mergeInstanceWithFlavor(instance *v1alpha1.RpaasInstance, flavor v1alpha1.RpaasFlavor) error {
	logger := log.WithName("mergeInstanceWithFlavor").
		WithValues("RpaasInstance", types.NamespacedName{Name: instance.Name, Namespace: instance.Namespace})
	if flavor.Spec.InstanceTemplate != nil {
		mergedInstanceSpec, err := mergeInstance(*flavor.Spec.InstanceTemplate, instance.Spec)
		if err != nil {
			logger.Error(err, "Could not merge instance against flavor instance template")
			return err
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

	return nil
}

func (r *ReconcileRpaasInstance) listDefaultFlavors(ctx context.Context, instance *v1alpha1.RpaasInstance) ([]v1alpha1.RpaasFlavor, error) {
	flavorList := &v1alpha1.RpaasFlavorList{}
	if err := r.client.List(ctx, flavorList, client.InNamespace(instance.Namespace)); err != nil {
		return nil, err
	}
	var result []v1alpha1.RpaasFlavor
	for _, flavor := range flavorList.Items {
		if flavor.Spec.Default {
			result = append(result, flavor)
		}
	}
	sort.SliceStable(result, func(i, j int) bool { return result[i].Name < result[j].Name })
	return result, nil
}

func (r *ReconcileRpaasInstance) reconcileHPA(ctx context.Context, instance *v1alpha1.RpaasInstance, nginx *nginxv1alpha1.Nginx) error {
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

func (r *ReconcileRpaasInstance) reconcileConfigMap(ctx context.Context, configMap *corev1.ConfigMap) error {
	found := &corev1.ConfigMap{}
	err := r.client.Get(ctx, types.NamespacedName{Name: configMap.ObjectMeta.Name, Namespace: configMap.ObjectMeta.Namespace}, found)
	if err != nil {
		if !k8sErrors.IsNotFound(err) {
			logrus.Errorf("Failed to get configMap: %v", err)
			return err
		}
		err = r.client.Create(ctx, configMap)
		if err != nil {
			logrus.Errorf("Failed to create configMap: %v", err)
			return err
		}
		return nil
	}

	configMap.ObjectMeta.ResourceVersion = found.ObjectMeta.ResourceVersion
	err = r.client.Update(ctx, configMap)
	if err != nil {
		logrus.Errorf("Failed to update configMap: %v", err)
	}
	return err
}

func (r *ReconcileRpaasInstance) reconcileNginx(ctx context.Context, nginx *nginxv1alpha1.Nginx) error {
	found := &nginxv1alpha1.Nginx{}
	err := r.client.Get(ctx, types.NamespacedName{Name: nginx.ObjectMeta.Name, Namespace: nginx.ObjectMeta.Namespace}, found)
	if err != nil {
		if !k8sErrors.IsNotFound(err) {
			logrus.Errorf("Failed to get nginx CR: %v", err)
			return err
		}
		err = r.client.Create(ctx, nginx)
		if err != nil {
			logrus.Errorf("Failed to create nginx CR: %v", err)
			return err
		}
		return nil
	}

	nginx.ObjectMeta.ResourceVersion = found.ObjectMeta.ResourceVersion
	err = r.client.Update(ctx, nginx)
	if err != nil {
		logrus.Errorf("Failed to update nginx CR: %v", err)
	}
	return err
}

func (r *ReconcileRpaasInstance) reconcileCacheSnapshot(ctx context.Context, instance *v1alpha1.RpaasInstance, plan *v1alpha1.RpaasPlan) error {
	if plan.Spec.Config.CacheSnapshotEnabled {
		err := r.reconcileCacheSnapshotCronJob(ctx, instance, plan)
		if err != nil {
			return err
		}
		return r.reconcileCacheSnapshotVolume(ctx, instance, plan)
	}

	err := r.destroyCacheSnapshotCronJob(ctx, instance)
	if err != nil {
		return err
	}
	return r.destroyCacheSnapshotVolume(ctx, instance)
}

func (r *ReconcileRpaasInstance) reconcileCacheSnapshotCronJob(ctx context.Context, instance *v1alpha1.RpaasInstance, plan *v1alpha1.RpaasPlan) error {
	foundCronJob := &batchv1beta1.CronJob{}
	cronName := instance.Name + cacheSnapshotCronJobSuffix
	err := r.client.Get(ctx, types.NamespacedName{Name: cronName, Namespace: instance.Namespace}, foundCronJob)
	if err != nil && !k8sErrors.IsNotFound(err) {
		return err
	}

	newestCronJob := newCronJob(instance, plan)
	if k8sErrors.IsNotFound(err) {
		return r.client.Create(ctx, newestCronJob)
	}

	newestCronJob.ObjectMeta.ResourceVersion = foundCronJob.ObjectMeta.ResourceVersion
	if !reflect.DeepEqual(foundCronJob.Spec, newestCronJob.Spec) {
		return r.client.Update(ctx, newestCronJob)
	}

	return nil
}

func (r *ReconcileRpaasInstance) destroyCacheSnapshotCronJob(ctx context.Context, instance *v1alpha1.RpaasInstance) error {
	cronName := instance.Name + cacheSnapshotCronJobSuffix
	cronJob := &batchv1beta1.CronJob{}

	err := r.client.Get(ctx, types.NamespacedName{Name: cronName, Namespace: instance.Namespace}, cronJob)
	isNotFound := k8sErrors.IsNotFound(err)
	if err != nil && !isNotFound {
		return err
	} else if isNotFound {
		return nil
	}

	logrus.Infof("deleting cronjob %s", cronName)
	return r.client.Delete(ctx, cronJob)
}
func (r *ReconcileRpaasInstance) reconcileCacheSnapshotVolume(ctx context.Context, instance *v1alpha1.RpaasInstance, plan *v1alpha1.RpaasPlan) error {
	pvcName := instance.Name + cacheSnapshotVolumeSuffix

	pvc := &corev1.PersistentVolumeClaim{}
	err := r.client.Get(ctx, types.NamespacedName{Name: pvcName, Namespace: instance.Namespace}, pvc)
	isNotFound := k8sErrors.IsNotFound(err)
	if err != nil && !isNotFound {
		return err
	} else if !isNotFound {
		return nil
	}

	cacheSnapshotStorage := plan.Spec.Config.CacheSnapshotStorage
	volumeMode := corev1.PersistentVolumeFilesystem
	labels := labelsForRpaasInstance(instance)
	if teamOwner := instance.TeamOwner(); teamOwner != "" {
		labels[volumeTeamLabel] = teamOwner
	}
	for k, v := range cacheSnapshotStorage.VolumeLabels {
		labels[k] = v
	}

	pvc = &corev1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{
			Name:      pvcName,
			Namespace: instance.Namespace,
			Labels:    labels,
			OwnerReferences: []metav1.OwnerReference{
				*metav1.NewControllerRef(instance, schema.GroupVersionKind{
					Group:   v1alpha1.SchemeGroupVersion.Group,
					Version: v1alpha1.SchemeGroupVersion.Version,
					Kind:    "RpaasInstance",
				}),
			},
		},
		TypeMeta: metav1.TypeMeta{
			APIVersion: "v1",
			Kind:       "PersistentVolumeClaim",
		},
		Spec: corev1.PersistentVolumeClaimSpec{
			AccessModes: []corev1.PersistentVolumeAccessMode{
				corev1.ReadWriteMany,
			},
			VolumeMode:       &volumeMode,
			StorageClassName: cacheSnapshotStorage.StorageClassName,
		},
	}

	storageSize := plan.Spec.Config.CacheSize
	if cacheSnapshotStorage.StorageSize != nil && !cacheSnapshotStorage.StorageSize.IsZero() {
		storageSize = cacheSnapshotStorage.StorageSize
	}

	if storageSize != nil && !storageSize.IsZero() {
		pvc.Spec.Resources = corev1.ResourceRequirements{
			Requests: corev1.ResourceList{
				"storage": *storageSize,
			},
		}
	}

	logrus.Infof("creating PersistentVolumeClaim %s", pvcName)
	return r.client.Create(ctx, pvc)
}

func (r *ReconcileRpaasInstance) destroyCacheSnapshotVolume(ctx context.Context, instance *v1alpha1.RpaasInstance) error {
	pvcName := instance.Name + cacheSnapshotVolumeSuffix

	pvc := &corev1.PersistentVolumeClaim{}
	err := r.client.Get(ctx, types.NamespacedName{Name: pvcName, Namespace: instance.Namespace}, pvc)
	isNotFound := k8sErrors.IsNotFound(err)
	if err != nil && !isNotFound {
		return err
	} else if isNotFound {
		return nil
	}

	logrus.Infof("deleting PersistentVolumeClaim %s", pvcName)
	return r.client.Delete(ctx, pvc)
}

func (r *ReconcileRpaasInstance) renderTemplate(ctx context.Context, instance *v1alpha1.RpaasInstance, plan *v1alpha1.RpaasPlan) (string, error) {
	blocks, err := r.getConfigurationBlocks(ctx, instance, plan)
	if err != nil {
		return "", err
	}

	if err = r.updateLocationValues(ctx, instance); err != nil {
		return "", err
	}

	cr, err := nginx.NewConfigurationRenderer(blocks)
	if err != nil {
		return "", err
	}

	return cr.Render(nginx.ConfigurationData{
		Instance: instance,
		Config:   &plan.Spec.Config,
	})
}

func (r *ReconcileRpaasInstance) getConfigurationBlocks(ctx context.Context, instance *v1alpha1.RpaasInstance, plan *v1alpha1.RpaasPlan) (nginx.ConfigurationBlocks, error) {
	var blocks nginx.ConfigurationBlocks

	if plan.Spec.Template != nil {
		mainBlock, err := util.GetValue(ctx, r.client, "", plan.Spec.Template)
		if err != nil {
			return blocks, err
		}

		blocks.MainBlock = mainBlock
	}

	for blockType, blockValue := range instance.Spec.Blocks {
		content, err := util.GetValue(ctx, r.client, instance.Namespace, &blockValue)
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

func (r *ReconcileRpaasInstance) updateLocationValues(ctx context.Context, instance *v1alpha1.RpaasInstance) error {
	for _, location := range instance.Spec.Locations {
		if location.Content == nil {
			continue
		}

		content, err := util.GetValue(ctx, r.client, instance.Namespace, location.Content)
		if err != nil {
			return err
		}

		location.Content.Value = content
	}
	return nil
}

func (r *ReconcileRpaasInstance) listConfigs(ctx context.Context, instance *v1alpha1.RpaasInstance) (*corev1.ConfigMapList, error) {
	configList := &corev1.ConfigMapList{}
	listOptions := &client.ListOptions{Namespace: instance.ObjectMeta.Namespace}
	client.MatchingLabels(map[string]string{
		"instance": instance.Name,
		"type":     "config",
	}).ApplyToList(listOptions)

	err := r.client.List(ctx, configList, listOptions)
	return configList, err
}

func (r *ReconcileRpaasInstance) deleteOldConfig(ctx context.Context, instance *v1alpha1.RpaasInstance, configList *corev1.ConfigMapList) error {
	list := configList.Items
	sort.Slice(list, func(i, j int) bool {
		return list[i].ObjectMeta.CreationTimestamp.String() < list[j].ObjectMeta.CreationTimestamp.String()
	})
	if err := r.client.Delete(ctx, &list[0]); err != nil {
		return err
	}
	return nil
}

func newConfigMap(instance *v1alpha1.RpaasInstance, renderedTemplate string) *corev1.ConfigMap {
	hash := fmt.Sprintf("%x", sha256.Sum256([]byte(renderedTemplate)))
	labels := labelsForRpaasInstance(instance)
	labels["type"] = "config"
	labels["instance"] = instance.Name

	return &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      fmt.Sprintf("%s-config-%s", instance.Name, hash[:10]),
			Namespace: instance.Namespace,
			Labels:    labels,
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

func newNginx(instance *v1alpha1.RpaasInstance, plan *v1alpha1.RpaasPlan, configMap *corev1.ConfigMap) *nginxv1alpha1.Nginx {
	var cacheConfig nginxv1alpha1.NginxCacheSpec
	if v1alpha1.BoolValue(plan.Spec.Config.CacheEnabled) {
		cacheConfig.Path = plan.Spec.Config.CachePath
		cacheConfig.InMemory = true
		if plan.Spec.Config.CacheSize != nil && !plan.Spec.Config.CacheSize.IsZero() {
			cacheConfig.Size = plan.Spec.Config.CacheSize
		}
	}
	n := &nginxv1alpha1.Nginx{
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
			Labels: labelsForRpaasInstance(instance),
		},
		TypeMeta: metav1.TypeMeta{
			Kind:       "Nginx",
			APIVersion: "nginx.tsuru.io/v1alpha1",
		},
		Spec: nginxv1alpha1.NginxSpec{
			Image:    plan.Spec.Image,
			Replicas: instance.Spec.Replicas,
			Config: &nginxv1alpha1.ConfigRef{
				Name: configMap.Name,
				Kind: nginxv1alpha1.ConfigKindConfigMap,
			},
			Resources:       plan.Spec.Resources,
			Service:         instance.Spec.Service,
			HealthcheckPath: "/_nginx_healthcheck",
			ExtraFiles:      instance.Spec.ExtraFiles,
			Certificates:    instance.Spec.Certificates,
			Cache:           cacheConfig,
			PodTemplate:     instance.Spec.PodTemplate,
			Lifecycle:       instance.Spec.Lifecycle,
		},
	}

	if !plan.Spec.Config.CacheSnapshotEnabled {
		return n
	}

	initCmd := defaultCacheSnapshotCmdPVCToPod
	if len(plan.Spec.Config.CacheSnapshotSync.CmdPVCToPod) > 0 {
		initCmd = plan.Spec.Config.CacheSnapshotSync.CmdPVCToPod
	}

	n.Spec.PodTemplate.Volumes = append(n.Spec.PodTemplate.Volumes, corev1.Volume{
		Name: "cache-snapshot-volume",
		VolumeSource: corev1.VolumeSource{
			PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
				ClaimName: instance.Name + cacheSnapshotVolumeSuffix,
			},
		},
	})

	cacheSnapshotVolume := corev1.VolumeMount{
		Name:      "cache-snapshot-volume",
		MountPath: cacheSnapshotMountPoint,
	}

	n.Spec.PodTemplate.VolumeMounts = append(n.Spec.PodTemplate.VolumeMounts, cacheSnapshotVolume)

	n.Spec.PodTemplate.InitContainers = append(n.Spec.PodTemplate.InitContainers, corev1.Container{
		Name:  "restore-snapshot",
		Image: plan.Spec.Image,
		Command: []string{
			initCmd[0],
		},
		Args: initCmd[1:],
		VolumeMounts: []corev1.VolumeMount{
			cacheSnapshotVolume,
			{
				Name:      "cache-vol",
				MountPath: plan.Spec.Config.CachePath,
			},
		},
		Env: append(cacheSnapshotEnvVars(instance, plan), corev1.EnvVar{
			Name:  "POD_CMD",
			Value: interpolateCacheSnapshotPodCmdTemplate(rsyncCommandPVCToPod, plan),
		}),
	})

	return n
}

func newHPA(instance *v1alpha1.RpaasInstance, nginx *nginxv1alpha1.Nginx) autoscalingv2beta2.HorizontalPodAutoscaler {
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
				*metav1.NewControllerRef(instance, schema.GroupVersionKind{
					Group:   v1alpha1.SchemeGroupVersion.Group,
					Version: v1alpha1.SchemeGroupVersion.Version,
					Kind:    "RpaasInstance",
				}),
			},
			Labels: labelsForRpaasInstance(instance),
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

func newCronJob(instance *v1alpha1.RpaasInstance, plan *v1alpha1.RpaasPlan) *batchv1beta1.CronJob {
	cronName := instance.Name + cacheSnapshotCronJobSuffix

	schedule := defaultCacheSnapshotSchedule
	if plan.Spec.Config.CacheSnapshotSync.Schedule != "" {
		schedule = plan.Spec.Config.CacheSnapshotSync.Schedule
	}

	image := defaultCacheSnapshotCronImage
	if plan.Spec.Config.CacheSnapshotSync.Image != "" {
		image = plan.Spec.Config.CacheSnapshotSync.Image
	}

	cmds := defaultCacheSnapshotCmdPodToPVC
	if len(plan.Spec.Config.CacheSnapshotSync.CmdPodToPVC) > 0 {
		cmds = plan.Spec.Config.CacheSnapshotSync.CmdPodToPVC
	}
	jobLabels := labelsForRpaasInstance(instance)
	jobLabels["log-app-name"] = instance.Name
	jobLabels["log-process-name"] = "cache-synchronize"

	return &batchv1beta1.CronJob{
		ObjectMeta: metav1.ObjectMeta{
			Name:      cronName,
			Namespace: instance.Namespace,
			OwnerReferences: []metav1.OwnerReference{
				*metav1.NewControllerRef(instance, schema.GroupVersionKind{
					Group:   v1alpha1.SchemeGroupVersion.Group,
					Version: v1alpha1.SchemeGroupVersion.Version,
					Kind:    "RpaasInstance",
				}),
			},
			Labels: labelsForRpaasInstance(instance),
		},
		TypeMeta: metav1.TypeMeta{
			APIVersion: "batch/v1beta1",
			Kind:       "CronJob",
		},
		Spec: batchv1beta1.CronJobSpec{
			Schedule:          schedule,
			ConcurrencyPolicy: batchv1beta1.ForbidConcurrent,
			JobTemplate: batchv1beta1.JobTemplateSpec{
				Spec: batchv1.JobSpec{
					Template: corev1.PodTemplateSpec{
						ObjectMeta: metav1.ObjectMeta{
							Labels: jobLabels,
						},
						Spec: corev1.PodSpec{
							ServiceAccountName: "rpaas-cache-snapshot-cronjob",
							Containers: []corev1.Container{
								{
									Name:  "cache-synchronize",
									Image: image,
									Command: []string{
										cmds[0],
									},
									Args: cmds[1:],
									Env: append(cacheSnapshotEnvVars(instance, plan), corev1.EnvVar{
										Name:  "POD_CMD",
										Value: interpolateCacheSnapshotPodCmdTemplate(rsyncCommandPodToPVC, plan),
									}),
								},
							},
							RestartPolicy: corev1.RestartPolicyNever,
						},
					},
				},
			},
		},
	}
}

func interpolateCacheSnapshotPodCmdTemplate(podCmd string, plan *v1alpha1.RpaasPlan) string {
	replacer := strings.NewReplacer(
		"${CACHE_SNAPSHOT_MOUNTPOINT}", cacheSnapshotMountPoint,
		"${CACHE_PATH}", plan.Spec.Config.CachePath,
	)
	return replacer.Replace(podCmd)
}

func cacheSnapshotEnvVars(instance *v1alpha1.RpaasInstance, plan *v1alpha1.RpaasPlan) []corev1.EnvVar {
	return []corev1.EnvVar{
		{Name: "SERVICE_NAME", Value: instance.Namespace},
		{Name: "INSTANCE_NAME", Value: instance.Name},
		{Name: "CACHE_SNAPSHOT_MOUNTPOINT", Value: cacheSnapshotMountPoint},
		{Name: "CACHE_PATH", Value: plan.Spec.Config.CachePath},
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

func renderCustomValues(instance *v1alpha1.RpaasInstance) error {
	if err := renderServiceCustomAnnotations(instance); err != nil {
		return err
	}

	return nil
}

func renderServiceCustomAnnotations(instance *v1alpha1.RpaasInstance) error {
	if instance == nil {
		return nil
	}

	if instance.Spec.Service == nil {
		return nil
	}

	for k, v := range instance.Spec.Service.Annotations {
		tmpl, err := template.New("rpaasv2.service.annotations").Parse(v)
		if err != nil {
			return err
		}

		var buffer bytes.Buffer
		if err = tmpl.Execute(&buffer, instance); err != nil {
			return err
		}

		instance.Spec.Service.Annotations[k] = buffer.String()
	}

	return nil
}

func mergeInstance(base v1alpha1.RpaasInstanceSpec, override v1alpha1.RpaasInstanceSpec) (merged v1alpha1.RpaasInstanceSpec, err error) {
	err = genericMerge(&merged, base, override)
	return
}

func mergePlans(base v1alpha1.RpaasPlanSpec, override v1alpha1.RpaasPlanSpec) (merged v1alpha1.RpaasPlanSpec, err error) {
	err = genericMerge(&merged, base, override)
	return
}

func genericMerge(dst interface{}, overrides ...interface{}) error {
	transformers := []func(*mergo.Config){
		mergo.WithOverride,
		mergo.WithAppendSlice,
		mergo.WithTransformers(boolPtrTransformer{}),
	}

	for _, override := range overrides {
		if err := mergo.Merge(dst, override, transformers...); err != nil {
			return err
		}
	}

	return nil
}

type boolPtrTransformer struct{}

func (_ boolPtrTransformer) Transformer(t reflect.Type) func(reflect.Value, reflect.Value) error {
	if reflect.TypeOf(v1alpha1.Bool(true)) != t {
		return nil
	}

	return func(dst, src reflect.Value) error {
		if src.IsNil() {
			return nil
		}

		if dst.Elem().Bool() == src.Elem().Bool() {
			return nil
		}

		if !dst.CanSet() {
			return fmt.Errorf("cannot set value to dst")
		}

		dst.Set(src)
		return nil
	}
}

func portBelongsTo(port extensionsv1alpha1.AllocatedPort, instance *extensionsv1alpha1.RpaasInstance) bool {
	if instance == nil {
		return false
	}
	return instance.UID == port.Owner.UID && port.Owner.Namespace == instance.Namespace && port.Owner.RpaasName == instance.Name
}

func (r *ReconcileRpaasInstance) reconcilePorts(ctx context.Context, instance *extensionsv1alpha1.RpaasInstance, portCount int) ([]int, error) {
	allocation := extensionsv1alpha1.RpaasPortAllocation{
		ObjectMeta: metav1.ObjectMeta{
			Name: defaultPortAllocationResource,
		},
	}
	err := r.client.Get(ctx, types.NamespacedName{
		Name: defaultPortAllocationResource,
	}, &allocation)
	if err != nil {
		if !errors.IsNotFound(err) {
			return nil, err
		}
		err = r.client.Create(ctx, &allocation)
		if err != nil {
			return nil, err
		}
	}

	portMin := config.Get().PortRangeMin
	portMax := config.Get().PortRangeMax

	var newPorts []extensionsv1alpha1.AllocatedPort
	var usedSet bitset.BitSet
	var instancePorts []int
	highestPortUsed := portMin - 1

	// Loop through all allocated ports and remove ports from removed Nginx
	// resources or from resources that have AllocateContainerPorts==false.
	for _, port := range allocation.Spec.Ports {
		if port.Port > highestPortUsed {
			highestPortUsed = port.Port
		}
		var rpaas extensionsv1alpha1.RpaasInstance
		err = r.client.Get(ctx, types.NamespacedName{
			Namespace: port.Owner.Namespace,
			Name:      port.Owner.RpaasName,
		}, &rpaas)
		if err != nil {
			if errors.IsNotFound(err) {
				continue
			}
			return nil, err
		}
		if portBelongsTo(port, instance) {
			if !instance.Spec.AllocateContainerPorts {
				continue
			}
			instancePorts = append(instancePorts, int(port.Port))
		}
		if portBelongsTo(port, &rpaas) {
			usedSet.Set(uint(port.Port))
			newPorts = append(newPorts, port)
		}
	}

	// If we should allocate ports and none are allocated yet we have to look
	// for available ports and allocate them.
	if instance != nil && instance.Spec.AllocateContainerPorts {
		for port := highestPortUsed + 1; port != highestPortUsed; port++ {
			if len(instancePorts) >= portCount {
				break
			}

			if port > portMax {
				port = portMin - 1
				continue
			}

			if usedSet.Test(uint(port)) {
				continue
			}

			usedSet.Set(uint(port))
			newPorts = append(newPorts, extensionsv1alpha1.AllocatedPort{
				Port: int32(port),
				Owner: extensionsv1alpha1.NamespacedOwner{
					Namespace: instance.Namespace,
					RpaasName: instance.Name,
					UID:       instance.UID,
				},
			})
			instancePorts = append(instancePorts, int(port))
		}

		if len(instancePorts) < portCount {
			return nil, fmt.Errorf("unable to allocate container ports, wanted %d, allocated %d", portCount, len(instancePorts))
		}
	}

	if !reflect.DeepEqual(allocation.Spec.Ports, newPorts) {
		allocation.Spec.Ports = newPorts
		err = r.client.Update(ctx, &allocation)
		if err != nil {
			return nil, err
		}
	}

	return instancePorts, nil
}

func labelsForRpaasInstance(instance *extensionsv1alpha1.RpaasInstance) map[string]string {
	return map[string]string{
		"rpaas.extensions.tsuru.io/instance-name": instance.Name,
		"rpaas.extensions.tsuru.io/plan-name":     instance.Spec.PlanName,
	}
}
