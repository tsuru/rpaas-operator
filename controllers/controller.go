// Copyright 2019 tsuru authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package controllers

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base32"
	"encoding/json"
	"fmt"
	"reflect"
	"sort"
	"strings"
	"text/template"

	"github.com/imdario/mergo"
	"github.com/sirupsen/logrus"
	nginxv1alpha1 "github.com/tsuru/nginx-operator/api/v1alpha1"
	nginxk8s "github.com/tsuru/nginx-operator/pkg/k8s"
	"github.com/willf/bitset"
	autoscalingv2beta2 "k8s.io/api/autoscaling/v2beta2"
	batchv1 "k8s.io/api/batch/v1"
	batchv1beta1 "k8s.io/api/batch/v1beta1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	k8sErrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	k8slabels "k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/tsuru/rpaas-operator/api/v1alpha1"
	extensionsv1alpha1 "github.com/tsuru/rpaas-operator/api/v1alpha1"
	"github.com/tsuru/rpaas-operator/internal/config"
	"github.com/tsuru/rpaas-operator/internal/pkg/rpaas/nginx"
	"github.com/tsuru/rpaas-operator/pkg/util"
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

	sessionTicketsSecretSuffix  = "-session-tickets"
	sessionTicketsCronJobSuffix = "-session-tickets"
)

var (
	defaultCacheSnapshotCmdPodToPVC = []string{
		"/bin/bash",
		"-c",
		`pods=($(kubectl -n ${SERVICE_NAME} get pod -l rpaas.extensions.tsuru.io/service-name=${SERVICE_NAME} -l rpaas.extensions.tsuru.io/instance-name=${INSTANCE_NAME} --field-selector status.phase=Running -o=jsonpath='{.items[*].metadata.name}'));
for pod in ${pods[@]}; do
	kubectl -n ${SERVICE_NAME} exec ${pod} -- ${POD_CMD};
	if [[ $? == 0 ]]; then
		exit 0;
	fi
done
echo "No pods found";
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

	defaultRotateTLSSessionTicketsImage = "bitnami/kubectl:latest"

	sessionTicketsVolumeName      = "tls-session-tickets"
	sessionTicketsVolumeMountPath = "/etc/nginx/tickets"

	rotateTLSSessionTicketsServiceAccountName = "rpaas-session-tickets-rotator"
	rotateTLSSessionTicketsVolumeName         = "tls-session-tickets-script"
	rotateTLSSessionTicketsScriptDir          = "/var/run/rpaasv2"
	rotateTLSSessionTicketsScriptFilename     = "tls_session_tickets_rotate.sh"
	rotateTLSSessionTicketsScriptPath         = fmt.Sprintf("%s/%s", rotateTLSSessionTicketsScriptDir, rotateTLSSessionTicketsScriptFilename)
	rotateTLSSessionTicketsScript             = `#!/bin/bash
set -euf -o pipefail

KUBECTL=${KUBECTL:-kubectl}
OPENSSL=${OPENSSL:-openssl}
BASE64=${BASE64:-base64}

SESSION_TICKET_KEY_LENGTH=${SESSION_TICKET_KEY_LENGTH:?missing session ticket key length}
SESSION_TICKET_KEYS=${SESSION_TICKET_KEYS:?missing number of session ticket keys}

SECRET_NAME=${SECRET_NAME:?missing Secret's name}
SECRET_NAMESPACE=${SECRET_NAMESPACE:?missing Secret's namespace}

function validate_key_length() {
  case ${SESSION_TICKET_KEY_LENGTH} in
    48|80)
      ;;
    *)
      echo "Nginx only has support to tickets with either 48 or 80 bytes, got ${SESSION_TICKET_KEY_LENGTH} bytes." &> /dev/stderr
      exit 1
  esac
}

function generate_key() {
  base64 -w0 <(${OPENSSL} rand ${SESSION_TICKET_KEY_LENGTH})
}

function json_merge_patch_payload() {
  local key=${1}

  local others=''
  for (( i = ${SESSION_TICKET_KEYS} - 1; i >= 1; i-- )) do
    others+=$(printf '{"op": "copy", "from": "/data/ticket.%d.key", "path": "/data/ticket.%d.key"},\n' $(( ${i} - 1 )) ${i})
  done

  cat <<-EOL
[
  ${others}
  {
    "op": "replace",
    "path": "/data/ticket.0.key",
    "value": "${key}"
  }
]
EOL
}

function rotate_session_tickets() {
  local key=${1}

  ${KUBECTL} patch secrets ${SECRET_NAME} --namespace ${SECRET_NAMESPACE} --type=json \
    --patch="$(json_merge_patch_payload ${key})"
}

function update_nginx_pods() {
  local selector=${1}

  ${KUBECTL} annotate pods --overwrite --namespace ${SECRET_NAMESPACE} --selector ${selector} \
    rpaas.extensions.tsuru.io/last-session-ticket-key-rotation="$(date +'%Y-%m-%dT%H:%M:%SZ')"
}

function main() {
  echo "Starting rotation of TLS session tickets within Secret (${SECRET_NAMESPACE}/${SECRET_NAME})..."
  rotate_session_tickets $(generate_key)
  echo "TLS session tickets successfully updated."

  if [[ -n ${NGINX_LABEL_SELECTOR} ]]; then
    echo "Updating Nginx pods with selector (${NGINX_LABEL_SELECTOR})..."
    update_nginx_pods ${NGINX_LABEL_SELECTOR}
  fi
}

main $@
`
)

func (r *RpaasInstanceReconciler) getRpaasInstance(ctx context.Context, objKey types.NamespacedName) (*extensionsv1alpha1.RpaasInstance, error) {
	var instance extensionsv1alpha1.RpaasInstance
	if err := r.Client.Get(ctx, objKey, &instance); err != nil {
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

func (r *RpaasInstanceReconciler) mergeInstanceWithFlavors(ctx context.Context, instance *extensionsv1alpha1.RpaasInstance) (*extensionsv1alpha1.RpaasInstance, error) {
	defaultFlavors, err := r.listDefaultFlavors(ctx, instance)
	if err != nil {
		return nil, err
	}

	for _, flavorName := range instance.Spec.Flavors {
		flavorObjectKey := types.NamespacedName{
			Name:      flavorName,
			Namespace: instance.Namespace,
		}

		var flavor extensionsv1alpha1.RpaasFlavor
		if err := r.Client.Get(ctx, flavorObjectKey, &flavor); err != nil {
			return nil, err
		}

		if flavor.Spec.Default {
			continue
		}

		if err := mergeInstanceWithFlavor(instance, flavor); err != nil {
			return nil, err
		}
	}

	for _, defaultFlavor := range defaultFlavors {
		if err := mergeInstanceWithFlavor(instance, defaultFlavor); err != nil {
			return nil, err
		}
	}

	return instance, nil
}

func mergeInstanceWithFlavor(instance *extensionsv1alpha1.RpaasInstance, flavor extensionsv1alpha1.RpaasFlavor) error {
	if flavor.Spec.InstanceTemplate == nil {
		return nil
	}

	mergedInstanceSpec, err := mergeInstance(*flavor.Spec.InstanceTemplate, instance.Spec)
	if err != nil {
		return err
	}
	instance.Spec = mergedInstanceSpec
	return nil
}

func (r *RpaasInstanceReconciler) listDefaultFlavors(ctx context.Context, instance *extensionsv1alpha1.RpaasInstance) ([]extensionsv1alpha1.RpaasFlavor, error) {
	flavorList := &v1alpha1.RpaasFlavorList{}
	if err := r.Client.List(ctx, flavorList, client.InNamespace(instance.Namespace)); err != nil {
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

func (r *RpaasInstanceReconciler) reconcileTLSSessionResumption(ctx context.Context, instance *v1alpha1.RpaasInstance) error {
	if err := r.reconcileSecretForSessionTickets(ctx, instance); err != nil {
		return err
	}

	return r.reconcileCronJobForSessionTickets(ctx, instance)
}

func (r *RpaasInstanceReconciler) reconcileSecretForSessionTickets(ctx context.Context, instance *v1alpha1.RpaasInstance) error {
	enabled := isTLSSessionTicketEnabled(instance)

	newSecret, err := newSecretForTLSSessionTickets(instance)
	if err != nil {
		return err
	}

	var secret corev1.Secret
	secretName := types.NamespacedName{
		Name:      newSecret.Name,
		Namespace: newSecret.Namespace,
	}
	err = r.Client.Get(ctx, secretName, &secret)
	if err != nil && k8sErrors.IsNotFound(err) {
		if !enabled {
			return nil
		}

		return r.Client.Create(ctx, newSecret)
	}

	if err != nil {
		return err
	}

	if !enabled {
		if !r.rolloutEnabled(instance) {
			return nil
		}
		return r.Client.Delete(ctx, &secret)
	}

	newData := newSessionTicketData(secret.Data, newSecret.Data)
	if !reflect.DeepEqual(newData, secret.Data) {
		secret.Data = newData
		return r.Client.Update(ctx, &secret)
	}

	return nil
}

func (r *RpaasInstanceReconciler) reconcileCronJobForSessionTickets(ctx context.Context, instance *v1alpha1.RpaasInstance) error {
	enabled := isTLSSessionTicketEnabled(instance)

	newCronJob := newCronJobForSessionTickets(instance)

	var cj batchv1beta1.CronJob
	cjName := types.NamespacedName{
		Name:      newCronJob.Name,
		Namespace: newCronJob.Namespace,
	}
	err := r.Client.Get(ctx, cjName, &cj)
	if err != nil && k8sErrors.IsNotFound(err) {
		if !enabled {
			return nil
		}

		return r.Client.Create(ctx, newCronJob)
	}

	if err != nil {
		return err
	}

	if !enabled {
		if !r.rolloutEnabled(instance) {
			return nil
		}
		return r.Client.Delete(ctx, &cj)
	}

	if reflect.DeepEqual(newCronJob.Spec, cj.Spec) {
		return nil
	}

	newCronJob.ResourceVersion = cj.ResourceVersion
	return r.Client.Update(ctx, newCronJob)
}

func newCronJobForSessionTickets(instance *v1alpha1.RpaasInstance) *batchv1beta1.CronJob {
	enabled := isTLSSessionTicketEnabled(instance)

	keyLength := v1alpha1.DefaultSessionTicketKeyLength
	if enabled && instance.Spec.TLSSessionResumption.SessionTicket.KeyLength != 0 {
		keyLength = instance.Spec.TLSSessionResumption.SessionTicket.KeyLength
	}

	rotationInterval := v1alpha1.DefaultSessionTicketKeyRotationInteval
	if enabled && instance.Spec.TLSSessionResumption.SessionTicket.KeyRotationInterval != 0 {
		rotationInterval = instance.Spec.TLSSessionResumption.SessionTicket.KeyRotationInterval
	}

	image := defaultCacheSnapshotCronImage
	if enabled && instance.Spec.TLSSessionResumption.SessionTicket.Image != "" {
		image = instance.Spec.TLSSessionResumption.SessionTicket.Image
	}

	return &batchv1beta1.CronJob{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "batch/v1beta1",
			Kind:       "CronJob",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      nameForCronJob(fmt.Sprintf("%s%s", instance.Name, sessionTicketsCronJobSuffix)),
			Namespace: instance.Namespace,
			OwnerReferences: []metav1.OwnerReference{
				*metav1.NewControllerRef(instance, schema.GroupVersionKind{
					Group:   v1alpha1.GroupVersion.Group,
					Version: v1alpha1.GroupVersion.Version,
					Kind:    "RpaasInstance",
				}),
			},
			Labels: labelsForRpaasInstance(instance),
		},
		Spec: batchv1beta1.CronJobSpec{
			Schedule: minutesIntervalToSchedule(rotationInterval),
			JobTemplate: batchv1beta1.JobTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{},
					Labels:      labelsForRpaasInstance(instance),
				},
				Spec: batchv1.JobSpec{
					Template: corev1.PodTemplateSpec{
						ObjectMeta: metav1.ObjectMeta{
							Annotations: map[string]string{
								rotateTLSSessionTicketsScriptFilename: rotateTLSSessionTicketsScript,
							},
						},
						Spec: corev1.PodSpec{
							ServiceAccountName: rotateTLSSessionTicketsServiceAccountName,
							RestartPolicy:      corev1.RestartPolicyNever,
							Containers: []corev1.Container{
								{
									Name:    "session-ticket-rotator",
									Image:   image,
									Command: []string{"/bin/bash"},
									Args:    []string{rotateTLSSessionTicketsScriptPath},
									Env: []corev1.EnvVar{
										{
											Name:  "SECRET_NAME",
											Value: secretNameForTLSSessionTickets(instance),
										},
										{
											Name:  "SECRET_NAMESPACE",
											Value: instance.Namespace,
										},
										{
											Name:  "SESSION_TICKET_KEY_LENGTH",
											Value: fmt.Sprint(keyLength),
										},
										{
											Name:  "SESSION_TICKET_KEYS",
											Value: fmt.Sprint(tlsSessionTicketKeys(instance)),
										},
										{
											Name:  "NGINX_LABEL_SELECTOR",
											Value: k8slabels.FormatLabels(nginxk8s.LabelsForNginx(instance.Name)),
										},
									},
									VolumeMounts: []corev1.VolumeMount{
										{
											Name:      rotateTLSSessionTicketsVolumeName,
											MountPath: rotateTLSSessionTicketsScriptPath,
											SubPath:   rotateTLSSessionTicketsScriptFilename,
										},
									},
								},
							},
							Volumes: []corev1.Volume{
								{
									Name: rotateTLSSessionTicketsVolumeName,
									VolumeSource: corev1.VolumeSource{
										DownwardAPI: &corev1.DownwardAPIVolumeSource{
											Items: []corev1.DownwardAPIVolumeFile{
												{
													Path: rotateTLSSessionTicketsScriptFilename,
													FieldRef: &corev1.ObjectFieldSelector{
														FieldPath: fmt.Sprintf("metadata.annotations['%s']", rotateTLSSessionTicketsScriptFilename),
													},
												},
											},
										},
									},
								},
							},
						},
					},
				},
			},
		},
	}
}

func newSecretForTLSSessionTickets(instance *v1alpha1.RpaasInstance) (*corev1.Secret, error) {
	keyLength := v1alpha1.DefaultSessionTicketKeyLength
	if isTLSSessionTicketEnabled(instance) && instance.Spec.TLSSessionResumption.SessionTicket.KeyLength != 0 {
		keyLength = instance.Spec.TLSSessionResumption.SessionTicket.KeyLength
	}

	data := make(map[string][]byte)
	for i := 0; i < tlsSessionTicketKeys(instance); i++ {
		key, err := generateSessionTicket(keyLength)
		if err != nil {
			return nil, err
		}

		data[fmt.Sprintf("ticket.%d.key", i)] = key
	}

	return &corev1.Secret{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "v1",
			Kind:       "Secret",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      secretNameForTLSSessionTickets(instance),
			Namespace: instance.Namespace,
			Labels:    labelsForRpaasInstance(instance),
			OwnerReferences: []metav1.OwnerReference{
				*metav1.NewControllerRef(instance, schema.GroupVersionKind{
					Group:   v1alpha1.GroupVersion.Group,
					Version: v1alpha1.GroupVersion.Version,
					Kind:    "RpaasInstance",
				}),
			},
		},
		Data: data,
	}, nil
}

func isTLSSessionTicketEnabled(instance *v1alpha1.RpaasInstance) bool {
	return instance.Spec.TLSSessionResumption != nil && instance.Spec.TLSSessionResumption.SessionTicket != nil
}

func tlsSessionTicketKeys(instance *v1alpha1.RpaasInstance) int {
	var nkeys int
	if isTLSSessionTicketEnabled(instance) {
		nkeys = int(instance.Spec.TLSSessionResumption.SessionTicket.KeepLastKeys)
	}
	return nkeys + 1
}

func secretNameForTLSSessionTickets(instance *v1alpha1.RpaasInstance) string {
	return fmt.Sprintf("%s%s", instance.Name, sessionTicketsSecretSuffix)
}

func generateSessionTicket(keyLength v1alpha1.SessionTicketKeyLength) ([]byte, error) {
	buffer := make([]byte, int(keyLength))
	_, err := rand.Read(buffer)
	if err != nil {
		return nil, err
	}
	return buffer, nil
}

func newSessionTicketData(old, new map[string][]byte) map[string][]byte {
	newest := make(map[string][]byte)
	for k, v := range new {
		if vv, found := old[k]; found {
			newest[k] = vv
			continue
		}
		newest[k] = v
	}

	for k, v := range old {
		if _, found := new[k]; found {
			newest[k] = v
		}
	}

	return newest
}

func (r *RpaasInstanceReconciler) reconcileHPA(ctx context.Context, instance *v1alpha1.RpaasInstance) error {
	logger := r.Log.WithName("reconcileHPA").
		WithValues("RpaasInstance", types.NamespacedName{Name: instance.Name, Namespace: instance.Namespace})

	logger.V(4).Info("Starting reconciliation of HorizontalPodAutoscaler")
	defer logger.V(4).Info("Finishing reconciliation of HorizontalPodAutoscaler")

	var hpa autoscalingv2beta2.HorizontalPodAutoscaler
	err := r.Client.Get(ctx, types.NamespacedName{Name: instance.Name, Namespace: instance.Namespace}, &hpa)
	if err != nil && k8sErrors.IsNotFound(err) {
		if instance.Spec.Autoscale == nil {
			logger.V(4).Info("Skipping HorizontalPodAutoscaler reconciliation: both HPA resource and desired RpaasAutoscaleSpec not found")
			return nil
		}

		logger.V(4).Info("Creating HorizontalPodAutoscaler resource")

		hpa = newHPA(instance)
		if err = r.Client.Create(ctx, &hpa); err != nil {
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
		if !r.rolloutEnabled(instance) {
			return nil
		}

		logger.V(4).Info("Deleting HorizontalPodAutoscaler resource")
		if err = r.Client.Delete(ctx, &hpa); err != nil {
			logger.Error(err, "Unable to delete the HorizontalPodAutoscaler resource")
			return err
		}

		return nil
	}

	newerHPA := newHPA(instance)
	if !reflect.DeepEqual(hpa.Spec, newerHPA.Spec) {
		logger.V(4).Info("Updating the HorizontalPodAustocaler spec")

		hpa.Spec = newerHPA.Spec
		if err = r.Client.Update(ctx, &hpa); err != nil {
			logger.Error(err, "Unable to update the HorizontalPodAustoscaler resource")
			return err
		}

		return nil
	}

	return nil
}

func (r *RpaasInstanceReconciler) reconcileConfigMap(ctx context.Context, configMap *corev1.ConfigMap) error {
	found := &corev1.ConfigMap{}
	err := r.Client.Get(ctx, types.NamespacedName{Name: configMap.ObjectMeta.Name, Namespace: configMap.ObjectMeta.Namespace}, found)
	if err != nil {
		if !k8sErrors.IsNotFound(err) {
			logrus.Errorf("Failed to get configMap: %v", err)
			return err
		}
		err = r.Client.Create(ctx, configMap)
		if err != nil {
			logrus.Errorf("Failed to create configMap: %v", err)
			return err
		}
		return nil
	}

	configMap.ObjectMeta.ResourceVersion = found.ObjectMeta.ResourceVersion
	err = r.Client.Update(ctx, configMap)
	if err != nil {
		logrus.Errorf("Failed to update configMap: %v", err)
	}
	return err
}

func (r *RpaasInstanceReconciler) getNginx(ctx context.Context, instance *v1alpha1.RpaasInstance) (*nginxv1alpha1.Nginx, error) {
	found := &nginxv1alpha1.Nginx{}
	err := r.Client.Get(ctx, types.NamespacedName{Name: instance.Name, Namespace: instance.Namespace}, found)
	if k8sErrors.IsNotFound(err) {
		return nil, err
	}
	return found, err
}

func (r *RpaasInstanceReconciler) reconcileNginx(ctx context.Context, instance *v1alpha1.RpaasInstance, nginx *nginxv1alpha1.Nginx) error {
	found, err := r.getNginx(ctx, instance)
	if err != nil {
		if !k8sErrors.IsNotFound(err) {
			logrus.Errorf("Failed to get nginx CR: %v", err)
			return err
		}
		err = r.Client.Create(ctx, nginx)
		if err != nil {
			logrus.Errorf("Failed to create nginx CR: %v", err)
			return err
		}
		return nil
	}

	if !r.rolloutEnabled(instance) {
		return nil
	}

	if found.Spec.Replicas != nil && *found.Spec.Replicas > 0 {
		nginx.Spec.Replicas = found.Spec.Replicas
	}
	nginx.ObjectMeta.ResourceVersion = found.ObjectMeta.ResourceVersion
	err = r.Client.Update(ctx, nginx)
	if err != nil {
		logrus.Errorf("Failed to update nginx CR: %v", err)
	}
	return err
}

func (r *RpaasInstanceReconciler) rolloutEnabled(instance *v1alpha1.RpaasInstance) bool {
	return r.RolloutNginxEnabled || instance.Spec.RolloutNginx || instance.Spec.RolloutNginxOnce
}

func (r *RpaasInstanceReconciler) reconcileCacheSnapshot(ctx context.Context, instance *v1alpha1.RpaasInstance, plan *v1alpha1.RpaasPlan) error {
	if plan.Spec.Config.CacheSnapshotEnabled {
		err := r.reconcileCacheSnapshotCronJob(ctx, instance, plan)
		if err != nil {
			return err
		}
		return r.reconcileCacheSnapshotVolume(ctx, instance, plan)
	}

	if !r.rolloutEnabled(instance) {
		return nil
	}

	err := r.destroyCacheSnapshotCronJob(ctx, instance)
	if err != nil {
		return err
	}
	return r.destroyCacheSnapshotVolume(ctx, instance)
}

func (r *RpaasInstanceReconciler) reconcileCacheSnapshotCronJob(ctx context.Context, instance *v1alpha1.RpaasInstance, plan *v1alpha1.RpaasPlan) error {
	foundCronJob := &batchv1beta1.CronJob{}
	cronName := nameForCronJob(instance.Name + cacheSnapshotCronJobSuffix)
	err := r.Client.Get(ctx, types.NamespacedName{Name: cronName, Namespace: instance.Namespace}, foundCronJob)
	if err != nil && !k8sErrors.IsNotFound(err) {
		return err
	}

	newestCronJob := newCronJob(instance, plan)
	if k8sErrors.IsNotFound(err) {
		return r.Client.Create(ctx, newestCronJob)
	}

	newestCronJob.ObjectMeta.ResourceVersion = foundCronJob.ObjectMeta.ResourceVersion
	if !reflect.DeepEqual(foundCronJob.Spec, newestCronJob.Spec) {
		return r.Client.Update(ctx, newestCronJob)
	}

	return nil
}

func (r *RpaasInstanceReconciler) destroyCacheSnapshotCronJob(ctx context.Context, instance *v1alpha1.RpaasInstance) error {
	cronName := nameForCronJob(instance.Name + cacheSnapshotCronJobSuffix)
	cronJob := &batchv1beta1.CronJob{}

	err := r.Client.Get(ctx, types.NamespacedName{Name: cronName, Namespace: instance.Namespace}, cronJob)
	isNotFound := k8sErrors.IsNotFound(err)
	if err != nil && !isNotFound {
		return err
	} else if isNotFound {
		return nil
	}

	logrus.Infof("deleting cronjob %s", cronName)
	return r.Client.Delete(ctx, cronJob)
}

func (r *RpaasInstanceReconciler) reconcileCacheSnapshotVolume(ctx context.Context, instance *v1alpha1.RpaasInstance, plan *v1alpha1.RpaasPlan) error {
	pvcName := instance.Name + cacheSnapshotVolumeSuffix

	pvc := &corev1.PersistentVolumeClaim{}
	err := r.Client.Get(ctx, types.NamespacedName{Name: pvcName, Namespace: instance.Namespace}, pvc)
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
					Group:   v1alpha1.GroupVersion.Group,
					Version: v1alpha1.GroupVersion.Version,
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
	return r.Client.Create(ctx, pvc)
}

func (r *RpaasInstanceReconciler) destroyCacheSnapshotVolume(ctx context.Context, instance *v1alpha1.RpaasInstance) error {
	pvcName := instance.Name + cacheSnapshotVolumeSuffix

	pvc := &corev1.PersistentVolumeClaim{}
	err := r.Client.Get(ctx, types.NamespacedName{Name: pvcName, Namespace: instance.Namespace}, pvc)
	isNotFound := k8sErrors.IsNotFound(err)
	if err != nil && !isNotFound {
		return err
	} else if isNotFound {
		return nil
	}

	logrus.Infof("deleting PersistentVolumeClaim %s", pvcName)
	return r.Client.Delete(ctx, pvc)
}

func (r *RpaasInstanceReconciler) renderTemplate(ctx context.Context, instance *v1alpha1.RpaasInstance, plan *v1alpha1.RpaasPlan) (string, error) {
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

func (r *RpaasInstanceReconciler) getConfigurationBlocks(ctx context.Context, instance *v1alpha1.RpaasInstance, plan *v1alpha1.RpaasPlan) (nginx.ConfigurationBlocks, error) {
	var blocks nginx.ConfigurationBlocks

	if plan.Spec.Template != nil {
		mainBlock, err := util.GetValue(ctx, r.Client, "", plan.Spec.Template)
		if err != nil {
			return blocks, err
		}

		blocks.MainBlock = mainBlock
	}

	for blockType, blockValue := range instance.Spec.Blocks {
		content, err := util.GetValue(ctx, r.Client, instance.Namespace, &blockValue)
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

func (r *RpaasInstanceReconciler) updateLocationValues(ctx context.Context, instance *v1alpha1.RpaasInstance) error {
	for _, location := range instance.Spec.Locations {
		if location.Content == nil {
			continue
		}

		content, err := util.GetValue(ctx, r.Client, instance.Namespace, location.Content)
		if err != nil {
			return err
		}

		location.Content.Value = content
	}
	return nil
}

func (r *RpaasInstanceReconciler) listConfigs(ctx context.Context, instance *v1alpha1.RpaasInstance) (*corev1.ConfigMapList, error) {
	configList := &corev1.ConfigMapList{}
	listOptions := &client.ListOptions{Namespace: instance.ObjectMeta.Namespace}
	client.MatchingLabels(map[string]string{
		"instance": instance.Name,
		"type":     "config",
	}).ApplyToList(listOptions)

	err := r.Client.List(ctx, configList, listOptions)
	return configList, err
}

func (r *RpaasInstanceReconciler) deleteOldConfig(ctx context.Context, instance *v1alpha1.RpaasInstance, configList *corev1.ConfigMapList) error {
	list := configList.Items
	sort.Slice(list, func(i, j int) bool {
		return list[i].ObjectMeta.CreationTimestamp.String() < list[j].ObjectMeta.CreationTimestamp.String()
	})

	var currentConfig string
	nginx, err := r.getNginx(ctx, instance)
	if err == nil && nginx.Spec.Config != nil {
		currentConfig = nginx.Spec.Config.Name
	}

	if list[0].Name == currentConfig {
		return nil
	}

	if err := r.Client.Delete(ctx, &list[0]); err != nil {
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
					Group:   v1alpha1.GroupVersion.Group,
					Version: v1alpha1.GroupVersion.Version,
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
					Group:   v1alpha1.GroupVersion.Group,
					Version: v1alpha1.GroupVersion.Version,
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

	if isTLSSessionTicketEnabled(instance) {
		n.Spec.PodTemplate.Volumes = append(n.Spec.PodTemplate.Volumes, corev1.Volume{
			Name: sessionTicketsVolumeName,
			VolumeSource: corev1.VolumeSource{
				Secret: &corev1.SecretVolumeSource{
					SecretName: secretNameForTLSSessionTickets(instance),
				},
			},
		})

		n.Spec.PodTemplate.VolumeMounts = append(n.Spec.PodTemplate.VolumeMounts, corev1.VolumeMount{
			Name:      sessionTicketsVolumeName,
			MountPath: sessionTicketsVolumeMountPath,
			ReadOnly:  true,
		})
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

func generateNginxHash(nginx *nginxv1alpha1.Nginx) (string, error) {
	nginx = nginx.DeepCopy()
	nginx.Spec.Replicas = nil
	data, err := json.Marshal(nginx.Spec)
	if err != nil {
		return "", err
	}
	hash := sha256.Sum256(data)
	return strings.ToLower(base32.StdEncoding.WithPadding(base32.NoPadding).EncodeToString(hash[:])), nil
}

func newHPA(instance *v1alpha1.RpaasInstance) autoscalingv2beta2.HorizontalPodAutoscaler {
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
					Group:   v1alpha1.GroupVersion.Group,
					Version: v1alpha1.GroupVersion.Version,
					Kind:    "RpaasInstance",
				}),
			},
			Labels: labelsForRpaasInstance(instance),
		},
		Spec: autoscalingv2beta2.HorizontalPodAutoscalerSpec{
			ScaleTargetRef: autoscalingv2beta2.CrossVersionObjectReference{
				APIVersion: "nginx.tsuru.io/v1alpha1",
				Kind:       "Nginx",
				Name:       instance.Name,
			},
			MinReplicas: minReplicas,
			MaxReplicas: instance.Spec.Autoscale.MaxReplicas,
			Metrics:     metrics,
		},
	}
}

func newCronJob(instance *v1alpha1.RpaasInstance, plan *v1alpha1.RpaasPlan) *batchv1beta1.CronJob {
	cronName := nameForCronJob(instance.Name + cacheSnapshotCronJobSuffix)

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
					Group:   v1alpha1.GroupVersion.Group,
					Version: v1alpha1.GroupVersion.Version,
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

func minutesIntervalToSchedule(minutes uint32) string {
	oneMinute := uint32(1)
	if minutes <= oneMinute {
		minutes = oneMinute
	}

	return fmt.Sprintf("*/%d * * * *", minutes)
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
		mergo.WithTransformers(rpaasMergoTransformers{}),
	}

	for _, override := range overrides {
		if err := mergo.Merge(dst, override, transformers...); err != nil {
			return err
		}
	}

	return nil
}

type rpaasMergoTransformers struct{}

func (_ rpaasMergoTransformers) Transformer(t reflect.Type) func(reflect.Value, reflect.Value) error {
	if reflect.TypeOf(v1alpha1.Bool(true)) == t {
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

	if reflect.TypeOf(corev1.ResourceList{}) == t {
		return func(dst, src reflect.Value) error {
			iter := src.MapRange()
			for iter.Next() {
				k := iter.Key()
				srcValue := iter.Value()
				dstValue := dst.MapIndex(k)

				if dstValue.IsZero() {
					continue
				}
				if reflect.DeepEqual(srcValue, dstValue) {
					continue
				}

				dst.SetMapIndex(k, srcValue)
			}
			return nil
		}
	}
	return nil

}

func portBelongsTo(port extensionsv1alpha1.AllocatedPort, instance *extensionsv1alpha1.RpaasInstance) bool {
	if instance == nil {
		return false
	}
	return instance.UID == port.Owner.UID && port.Owner.Namespace == instance.Namespace && port.Owner.RpaasName == instance.Name
}

func (r *RpaasInstanceReconciler) reconcileDedicatedPorts(ctx context.Context, instance *extensionsv1alpha1.RpaasInstance, portCount int) ([]int, error) {
	allocation := extensionsv1alpha1.RpaasPortAllocation{
		ObjectMeta: metav1.ObjectMeta{
			Name: defaultPortAllocationResource,
		},
	}

	err := r.Client.Get(ctx, types.NamespacedName{Name: allocation.Name}, &allocation)
	if err != nil {
		if !errors.IsNotFound(err) {
			return nil, err
		}

		err = r.Client.Create(ctx, &allocation)
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
	// resources or from resources that have AllocateContainerPorts==false (or nil).
	for _, port := range allocation.Spec.Ports {
		if port.Port > highestPortUsed {
			highestPortUsed = port.Port
		}
		var rpaas extensionsv1alpha1.RpaasInstance
		err = r.Client.Get(ctx, types.NamespacedName{
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
			if !v1alpha1.BoolValue(instance.Spec.AllocateContainerPorts) {
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
	if instance != nil && v1alpha1.BoolValue(instance.Spec.AllocateContainerPorts) {
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
		err = r.Client.Update(ctx, &allocation)
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

func nameForCronJob(name string) string {
	const cronjobMaxChars = 52

	if len(name) <= cronjobMaxChars {
		return name
	}

	digest := util.SHA256(name)[:10]
	return name[:cronjobMaxChars-len(digest)] + digest
}
