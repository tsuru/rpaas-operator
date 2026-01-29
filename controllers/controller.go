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
	"errors"
	"fmt"
	"reflect"
	"slices"
	"sort"
	"strconv"
	"strings"
	"text/template"

	cmv1 "github.com/cert-manager/cert-manager/pkg/apis/certmanager/v1"
	"github.com/go-logr/logr"
	"github.com/imdario/mergo"
	kedav1alpha1 "github.com/kedacore/keda/v2/apis/keda/v1alpha1"
	"github.com/sirupsen/logrus"
	nginxv1alpha1 "github.com/tsuru/nginx-operator/api/v1alpha1"
	nginxk8s "github.com/tsuru/nginx-operator/pkg/k8s"
	appsv1 "k8s.io/api/apps/v1"
	autoscalingv2 "k8s.io/api/autoscaling/v2"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	policyv1 "k8s.io/api/policy/v1"
	"k8s.io/apimachinery/pkg/api/equality"
	k8sErrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	k8slabels "k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/tsuru/rpaas-operator/api/v1alpha1"
	"github.com/tsuru/rpaas-operator/internal/controllers/certificates"
	controllerUtil "github.com/tsuru/rpaas-operator/internal/controllers/util"
	"github.com/tsuru/rpaas-operator/internal/pkg/rpaas/nginx"
	"github.com/tsuru/rpaas-operator/pkg/util"
)

const (
	defaultConfigHistoryLimit = 10

	sessionTicketsSecretSuffix  = "-session-tickets"
	sessionTicketsCronJobSuffix = "-session-tickets"

	externalDNSHostnameLabel = "external-dns.alpha.kubernetes.io/hostname"
	externalDNSTTLLabel      = "external-dns.alpha.kubernetes.io/ttl"
)

var (
	defaultRotateTLSSessionTicketsImage = "tsuru/kubectl:latest"

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

func (r *RpaasInstanceReconciler) getRpaasInstance(ctx context.Context, objKey types.NamespacedName) (*v1alpha1.RpaasInstance, error) {
	var instance v1alpha1.RpaasInstance
	if err := r.Client.Get(ctx, objKey, &instance); err != nil {
		return nil, err
	}

	return &instance, nil
}

func (r *RpaasInstanceReconciler) mergeWithFlavors(ctx context.Context, instance *v1alpha1.RpaasInstance) (*v1alpha1.RpaasInstance, error) {
	mergedInstance, err := r.mergeInstanceWithFlavors(ctx, instance)
	if err != nil {
		return nil, err
	}

	if err = controllerUtil.RenderCustomValues(mergedInstance); err != nil {
		return nil, err
	}

	// NOTE: preventing this merged resource be persisted on k8s api server.
	mergedInstance.ResourceVersion = ""

	return mergedInstance, nil
}

func (r *RpaasInstanceReconciler) mergeInstanceWithFlavors(ctx context.Context, instance *v1alpha1.RpaasInstance) (*v1alpha1.RpaasInstance, error) {
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

		if instance.Spec.PlanNamespace != "" {
			flavorObjectKey.Namespace = instance.Spec.PlanNamespace
		}

		var flavor v1alpha1.RpaasFlavor
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

	return instance, nil
}

func mergeInstanceWithFlavor(instance *v1alpha1.RpaasInstance, flavor v1alpha1.RpaasFlavor) error {
	if flavor.Spec.InstanceTemplate == nil {
		return nil
	}

	mergedInstanceSpec, err := mergeInstance(instance.Spec, *flavor.Spec.InstanceTemplate)
	if err != nil {
		return err
	}
	instance.Spec = mergedInstanceSpec
	return nil
}

func (r *RpaasInstanceReconciler) listDefaultFlavors(ctx context.Context, instance *v1alpha1.RpaasInstance) ([]v1alpha1.RpaasFlavor, error) {
	flavorNamespace := instance.Namespace
	if instance.Spec.PlanNamespace != "" {
		flavorNamespace = instance.Spec.PlanNamespace
	}

	return listDefaultFlavors(ctx, r.Client, flavorNamespace)
}

func (r *RpaasInstanceReconciler) reconcileTLSSessionResumption(ctx context.Context, instance *v1alpha1.RpaasInstance) (hasChanged bool, err error) {
	secretChanged, err := r.reconcileSecretForSessionTickets(ctx, instance)
	if err != nil {
		return false, err
	}

	cronJobChanged, err := r.reconcileCronJobForSessionTickets(ctx, instance)
	if err != nil {
		return false, err
	}

	return cronJobChanged || secretChanged, nil
}

func (r *RpaasInstanceReconciler) reconcileSecretForSessionTickets(ctx context.Context, instance *v1alpha1.RpaasInstance) (hasChanged bool, err error) {
	enabled := isTLSSessionTicketEnabled(&instance.Spec)

	newSecret, err := newSecretForTLSSessionTickets(instance)
	if err != nil {
		return false, err
	}

	var secret corev1.Secret
	secretName := types.NamespacedName{
		Name:      newSecret.Name,
		Namespace: newSecret.Namespace,
	}
	err = r.Client.Get(ctx, secretName, &secret)
	if err != nil && k8sErrors.IsNotFound(err) {
		if !enabled {
			return false, nil
		}

		err = r.Client.Create(ctx, newSecret)
		if err != nil {
			return false, err
		}

		return true, nil
	} else if err != nil {
		return false, err
	}

	if !enabled {
		err = r.Client.Delete(ctx, &secret)
		if err != nil {
			return false, err
		}

		return true, nil
	}

	newData := newSessionTicketData(secret.Data, newSecret.Data)
	if !reflect.DeepEqual(newData, secret.Data) {
		secret.Data = newData
		err = r.Client.Update(ctx, &secret)
		if err != nil {
			return false, err
		}

		return true, nil
	}

	return false, nil
}

func (r *RpaasInstanceReconciler) reconcileCronJobForSessionTickets(ctx context.Context, instance *v1alpha1.RpaasInstance) (hasChanged bool, err error) {
	enabled := isTLSSessionTicketEnabled(&instance.Spec)

	newCronJob := newCronJobForSessionTickets(instance)

	var cj batchv1.CronJob
	cjName := types.NamespacedName{
		Name:      newCronJob.Name,
		Namespace: newCronJob.Namespace,
	}
	err = r.Client.Get(ctx, cjName, &cj)
	if err != nil && k8sErrors.IsNotFound(err) {
		if !enabled {
			return false, nil
		}

		err = r.Client.Create(ctx, newCronJob)
		if err != nil {
			return false, err
		}

		return true, nil
	} else if err != nil {
		return false, err
	}

	if !enabled {
		err = r.Client.Delete(ctx, &cj)
		if err != nil {
			return false, err
		}
		return true, nil
	}

	if equality.Semantic.DeepDerivative(newCronJob.Spec, cj.Spec) {
		return false, nil
	}

	newCronJob.ResourceVersion = cj.ResourceVersion
	err = r.Client.Update(ctx, newCronJob)
	if err != nil {
		return false, err
	}

	return true, nil
}

func newCronJobForSessionTickets(instance *v1alpha1.RpaasInstance) *batchv1.CronJob {
	enabled := isTLSSessionTicketEnabled(&instance.Spec)

	keyLength := v1alpha1.DefaultSessionTicketKeyLength
	if enabled && instance.Spec.TLSSessionResumption.SessionTicket.KeyLength != 0 {
		keyLength = instance.Spec.TLSSessionResumption.SessionTicket.KeyLength
	}

	rotationInterval := v1alpha1.DefaultSessionTicketKeyRotationInteval
	if enabled && instance.Spec.TLSSessionResumption.SessionTicket.KeyRotationInterval != 0 {
		rotationInterval = instance.Spec.TLSSessionResumption.SessionTicket.KeyRotationInterval
	}

	image := defaultRotateTLSSessionTicketsImage
	if enabled && instance.Spec.TLSSessionResumption.SessionTicket.Image != "" {
		image = instance.Spec.TLSSessionResumption.SessionTicket.Image
	}

	var jobsHistoryLimit int32 = 1
	return &batchv1.CronJob{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "batch/v1",
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
			Labels: instance.GetBaseLabels(nil),
		},
		Spec: batchv1.CronJobSpec{
			Schedule:                   minutesIntervalToSchedule(rotationInterval),
			SuccessfulJobsHistoryLimit: &jobsHistoryLimit,
			FailedJobsHistoryLimit:     &jobsHistoryLimit,
			JobTemplate: batchv1.JobTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: instance.GetBaseLabels(nil),
				},
				Spec: batchv1.JobSpec{
					Template: corev1.PodTemplateSpec{
						ObjectMeta: metav1.ObjectMeta{
							Annotations: map[string]string{
								rotateTLSSessionTicketsScriptFilename: rotateTLSSessionTicketsScript,
							},
							Labels: map[string]string{
								"rpaas.extensions.tsuru.io/component": "session-tickets",
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
											Value: secretNameForTLSSessionTickets(instance.Name),
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
	if isTLSSessionTicketEnabled(&instance.Spec) && instance.Spec.TLSSessionResumption.SessionTicket.KeyLength != 0 {
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
			Name:      secretNameForTLSSessionTickets(instance.Name),
			Namespace: instance.Namespace,
			Labels:    instance.GetBaseLabels(nil),
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

func isTLSSessionTicketEnabled(spec *v1alpha1.RpaasInstanceSpec) bool {
	return spec.TLSSessionResumption != nil && spec.TLSSessionResumption.SessionTicket != nil
}

func tlsSessionTicketKeys(instance *v1alpha1.RpaasInstance) int {
	var nkeys int
	if isTLSSessionTicketEnabled(&instance.Spec) {
		nkeys = int(instance.Spec.TLSSessionResumption.SessionTicket.KeepLastKeys)
	}
	return nkeys + 1
}

func secretNameForTLSSessionTickets(instanceName string) string {
	return fmt.Sprintf("%s%s", instanceName, sessionTicketsSecretSuffix)
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

func (r *RpaasInstanceReconciler) reconcileHPA(ctx context.Context, instance *v1alpha1.RpaasInstance, nginx *nginxv1alpha1.Nginx) (hasChanged bool, err error) {
	if isKEDAHandlingHPA(instance) {
		return r.reconcileKEDA(ctx, instance, nginx)
	}

	cleanedKeda, err := r.cleanUpKEDAScaledObject(ctx, instance)
	if err != nil {
		return false, err
	}

	logger := r.Log.WithName("reconcileHPA").
		WithValues("RpaasInstance", types.NamespacedName{Name: instance.Name, Namespace: instance.Namespace})

	logger.V(4).Info("Starting reconciliation of HorizontalPodAutoscaler")
	defer logger.V(4).Info("Finishing reconciliation of HorizontalPodAutoscaler")

	if a := instance.Spec.Autoscale; a != nil && a.TargetRequestsPerSecond != nil {
		r.EventRecorder.Event(instance, corev1.EventTypeWarning, "RpaasInstanceAutoscaleFailed", "native HPA controller doesn't support RPS metric target yet")
	}

	if a := instance.Spec.Autoscale; a != nil && len(a.Schedules) > 0 {
		r.EventRecorder.Event(instance, corev1.EventTypeWarning, "RpaasInstanceAutoscaleFailed", "native HPA controller doesn't support scheduled windows")
	}

	desired := newHPA(instance, nginx)

	var observed autoscalingv2.HorizontalPodAutoscaler
	err = r.Client.Get(ctx, types.NamespacedName{Name: desired.Name, Namespace: desired.Namespace}, &observed)
	if k8sErrors.IsNotFound(err) {
		if !isAutoscaleEnabled(&instance.Spec) {
			logger.V(4).Info("Skipping HorizontalPodAutoscaler reconciliation: both HPA resource and desired RpaasAutoscaleSpec not found")
			return cleanedKeda, nil
		}

		logger.V(4).Info("Creating HorizontalPodAutoscaler resource")

		if err = r.Client.Create(ctx, desired); err != nil {
			logger.Error(err, "Unable to create the HorizontalPodAutoscaler resource")
			return false, err
		}

		return true, nil
	}

	if err != nil {
		logger.Error(err, "Unable to get the HorizontalPodAutoscaler resource")
		return false, err
	}

	logger = logger.WithValues("HorizontalPodAutoscaler", types.NamespacedName{Name: observed.Name, Namespace: observed.Namespace})

	if !isAutoscaleEnabled(&instance.Spec) {
		logger.V(4).Info("Deleting HorizontalPodAutoscaler resource")
		if err = r.Client.Delete(ctx, &observed); err != nil {
			logger.Error(err, "Unable to delete the HorizontalPodAutoscaler resource")
			return false, err
		}

		return true, nil
	}

	if !reflect.DeepEqual(desired.Spec, observed.Spec) {
		logger.V(4).Info("Updating the HorizontalPodAustocaler spec")

		observed.Spec = desired.Spec
		if err = r.Client.Update(ctx, &observed); err != nil {
			logger.Error(err, "Unable to update the HorizontalPodAustoscaler resource")
			return false, err
		}

		return true, nil
	}

	return cleanedKeda, nil
}

func (r *RpaasInstanceReconciler) cleanUpKEDAScaledObject(ctx context.Context, instance *v1alpha1.RpaasInstance) (cleaned bool, err error) {
	var so kedav1alpha1.ScaledObject
	err = r.Client.Get(ctx, types.NamespacedName{Name: instance.Name, Namespace: instance.Namespace}, &so)
	if k8sErrors.IsNotFound(err) {
		return false, nil
	}

	if err != nil {
		return false, nil // custom resource does likely not exist in the cluster, so we should ignore it
	}

	err = r.Client.Delete(ctx, &so)
	if err != nil {
		return false, err
	}
	return true, nil
}

func isKEDAHandlingHPA(instance *v1alpha1.RpaasInstance) bool {
	return instance.Spec.Autoscale != nil &&
		(instance.Spec.Autoscale.TargetRequestsPerSecond != nil || len(instance.Spec.Autoscale.Schedules) > 0) &&
		instance.Spec.Autoscale.KEDAOptions != nil &&
		instance.Spec.Autoscale.KEDAOptions.Enabled
}

func (r *RpaasInstanceReconciler) reconcileKEDA(ctx context.Context, instance *v1alpha1.RpaasInstance, nginx *nginxv1alpha1.Nginx) (hasChanged bool, err error) {
	desired, err := newKEDAScaledObject(instance, nginx)
	if err != nil {
		return false, err
	}

	var observed kedav1alpha1.ScaledObject
	err = r.Client.Get(ctx, types.NamespacedName{Name: desired.Name, Namespace: desired.Namespace}, &observed)
	if k8sErrors.IsNotFound(err) {
		if !isAutoscaleEnabled(&instance.Spec) {
			return false, nil // nothing to do
		}

		err = r.Client.Create(ctx, desired)
		if err != nil {
			return false, err
		}
		return true, nil
	}

	if err != nil {
		return false, err
	}

	if !isAutoscaleEnabled(&instance.Spec) {
		err = r.Client.Delete(ctx, &observed)
		if err != nil {
			return false, err
		}
		return true, nil
	}

	if reflect.DeepEqual(desired.Spec, observed.Spec) {
		return false, nil
	}

	desired.ResourceVersion = observed.ResourceVersion
	err = r.Client.Update(ctx, desired)
	if err != nil {
		return false, err
	}
	return true, nil
}

func isAutoscaleValid(a *v1alpha1.RpaasInstanceAutoscaleSpec) bool {
	return a != nil &&
		(a.MinReplicas != nil && *a.MinReplicas > 0 && a.MaxReplicas > 0) &&
		(a.TargetCPUUtilizationPercentage != nil || a.TargetMemoryUtilizationPercentage != nil || a.TargetRequestsPerSecond != nil || len(a.Schedules) > 0)
}

func isAutoscaleEnabled(instance *v1alpha1.RpaasInstanceSpec) bool {
	return !instance.Shutdown && isAutoscaleValid(instance.Autoscale)
}

func buildMetadataForScaledObject(instance *v1alpha1.RpaasInstance, value string) map[string]string {
	metadata := map[string]string{
		"value": value,
	}
	if len(instance.Spec.PodTemplate.Containers) > 0 {
		metadata["containerName"] = "nginx"
	}

	return metadata
}

func newKEDAScaledObject(instance *v1alpha1.RpaasInstance, nginx *nginxv1alpha1.Nginx) (*kedav1alpha1.ScaledObject, error) {
	var triggers []kedav1alpha1.ScaleTriggers
	var hasAutoscale bool = instance.Spec.Autoscale != nil
	if hasAutoscale && instance.Spec.Autoscale.TargetCPUUtilizationPercentage != nil {
		triggers = append(triggers, kedav1alpha1.ScaleTriggers{
			Type:       "cpu",
			MetricType: autoscalingv2.UtilizationMetricType,
			Metadata:   buildMetadataForScaledObject(instance, strconv.Itoa(int(*instance.Spec.Autoscale.TargetCPUUtilizationPercentage))),
		})
	}

	if hasAutoscale && instance.Spec.Autoscale.TargetMemoryUtilizationPercentage != nil {
		triggers = append(triggers, kedav1alpha1.ScaleTriggers{
			Type:       "memory",
			MetricType: autoscalingv2.UtilizationMetricType,
			Metadata:   buildMetadataForScaledObject(instance, strconv.Itoa(int(*instance.Spec.Autoscale.TargetMemoryUtilizationPercentage))),
		})
	}

	if hasAutoscale && instance.Spec.Autoscale.TargetRequestsPerSecond != nil {
		kopts := instance.Spec.Autoscale.KEDAOptions
		if kopts == nil {
			return nil, errors.New("keda options not provided")
		}

		queryTemplate, err := template.New("rpaasv2-autoscale-rps-query").Parse(kopts.RPSQueryTemplate)
		if err != nil {
			return nil, fmt.Errorf("unable to parse the request per second query template: %w", err)
		}

		var query bytes.Buffer
		if err = queryTemplate.Execute(&query, instance); err != nil {
			return nil, fmt.Errorf("unable to render the requestg per second query template: %w", err)
		}

		triggers = append(triggers, kedav1alpha1.ScaleTriggers{
			Type: "prometheus",
			Metadata: map[string]string{
				"serverAddress": instance.Spec.Autoscale.KEDAOptions.PrometheusServerAddress,
				"query":         query.String(),
				"threshold":     strconv.Itoa(int(*instance.Spec.Autoscale.TargetRequestsPerSecond)),
			},
			AuthenticationRef: kopts.RPSAuthenticationRef,
		})
	}

	if hasAutoscale {
		for _, s := range instance.Spec.Autoscale.Schedules {
			timezone := s.Timezone
			if timezone == "" && instance.Spec.Autoscale.KEDAOptions != nil {
				timezone = instance.Spec.Autoscale.KEDAOptions.Timezone
			}

			triggers = append(triggers, kedav1alpha1.ScaleTriggers{
				Type: "cron",
				Metadata: map[string]string{
					"desiredReplicas": strconv.Itoa(int(s.MinReplicas)),
					"start":           s.Start,
					"end":             s.End,
					"timezone":        timezone,
				},
			})
		}
	}

	deployName := instance.Name
	if deployments := nginx.Status.Deployments; len(deployments) > 0 {
		deployName = deployments[0].Name
	}

	var minReplicas *int32
	if hasAutoscale {
		minReplicas = instance.Spec.Autoscale.MinReplicas
	}

	var maxReplicas *int32
	if hasAutoscale {
		maxReplicas = &instance.Spec.Autoscale.MaxReplicas
	}

	var pollingInterval *int32
	if hasAutoscale && instance.Spec.Autoscale.KEDAOptions != nil {
		pollingInterval = instance.Spec.Autoscale.KEDAOptions.PollingInterval
	}

	var scalingDownRules *autoscalingv2.HPAScalingRules
	if hasAutoscale && instance.Spec.Autoscale.Behavior != nil && instance.Spec.Autoscale.Behavior.ScaleDown != nil {
		var scalingDown = instance.Spec.Autoscale.Behavior.ScaleDown
		var policies []autoscalingv2.HPAScalingPolicy
		if scalingDown.StabilizationWindowSeconds != nil {
			scalingDownRules.StabilizationWindowSeconds = scalingDown.StabilizationWindowSeconds
		}
		if scalingDown.UnitsPolicyValue != nil {
			policy := autoscalingv2.HPAScalingPolicy{
				Type:  autoscalingv2.PodsScalingPolicy,
				Value: *scalingDown.UnitsPolicyValue,
			}
			policies = append(policies, policy)
		}
		if scalingDown.PercentPolicyValue != nil {
			policy := autoscalingv2.HPAScalingPolicy{
				Type:  autoscalingv2.PercentScalingPolicy,
				Value: *scalingDown.PercentPolicyValue,
			}
			policies = append(policies, policy)
		}
		if policies != nil {
			scalingDownRules.Policies = policies
		}
	}

	return &kedav1alpha1.ScaledObject{
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
			Labels: instance.GetBaseLabels(nil),
			Annotations: map[string]string{
				// NOTE: allows the KEDA controller to take over the ownership of HPA resources.
				"scaledobject.keda.sh/transfer-hpa-ownership": strconv.FormatBool(true),
			},
		},
		Spec: kedav1alpha1.ScaledObjectSpec{
			ScaleTargetRef: &kedav1alpha1.ScaleTarget{
				APIVersion: "apps/v1",
				Kind:       "Deployment",
				Name:       deployName,
			},
			MinReplicaCount: minReplicas,
			MaxReplicaCount: maxReplicas,
			PollingInterval: pollingInterval,
			Triggers:        triggers,
			Advanced: &kedav1alpha1.AdvancedConfig{
				HorizontalPodAutoscalerConfig: &kedav1alpha1.HorizontalPodAutoscalerConfig{
					Name: instance.Name,
					Behavior: &autoscalingv2.HorizontalPodAutoscalerBehavior{
						ScaleDown: scalingDownRules,
					},
				},
			},
		},
	}, nil
}

func (r *RpaasInstanceReconciler) reconcilePDB(ctx context.Context, instance *v1alpha1.RpaasInstance, nginx *nginxv1alpha1.Nginx) (hasChanged bool, err error) {
	if nginx.Status.PodSelector == "" {
		return false, nil
	}
	pdb, err := newPDB(instance, nginx)
	if err != nil {
		return false, err
	}

	var existingPDB policyv1.PodDisruptionBudget
	err = r.Get(ctx, client.ObjectKey{Name: pdb.Name, Namespace: pdb.Namespace}, &existingPDB)
	if err != nil {
		if !k8sErrors.IsNotFound(err) {
			return false, err
		}

		if instance.Spec.EnablePodDisruptionBudget != nil && *instance.Spec.EnablePodDisruptionBudget {
			err = r.Create(ctx, pdb)
			if err != nil {
				return false, err
			}
			return true, nil
		}

		return false, nil
	}

	if instance.Spec.EnablePodDisruptionBudget == nil || (instance.Spec.EnablePodDisruptionBudget != nil && !*instance.Spec.EnablePodDisruptionBudget) {
		err = r.Delete(ctx, &existingPDB)
		if err != nil {
			return false, err
		}
		return true, nil
	}

	if equality.Semantic.DeepDerivative(existingPDB.Spec, pdb.Spec) && reflect.DeepEqual(existingPDB.Labels, pdb.Labels) {
		return false, nil
	}

	pdb.ResourceVersion = existingPDB.ResourceVersion
	err = r.Update(ctx, pdb)
	if err != nil {
		return false, err
	}
	return true, nil
}

func newPDB(instance *v1alpha1.RpaasInstance, nginx *nginxv1alpha1.Nginx) (*policyv1.PodDisruptionBudget, error) {
	set, err := k8slabels.ConvertSelectorToLabelsMap(nginx.Status.PodSelector)
	if err != nil {
		return nil, err
	}

	// NOTE: taking 10% of the real min unavailable to support operational tasks
	// in the cluster, e.g scaling up/down nodes from Cluster Autoscaler.
	maxUnavailable := intstr.FromString("10%")

	return &policyv1.PodDisruptionBudget{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "policy/v1",
			Kind:       "PodDisruptionBudget",
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
			Labels: instance.GetBaseLabels(nil),
		},
		Spec: policyv1.PodDisruptionBudgetSpec{
			MaxUnavailable: &maxUnavailable,
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string(set),
			},
		},
	}, nil
}

func (r *RpaasInstanceReconciler) reconcileConfigMap(ctx context.Context, configMap *corev1.ConfigMap) (hasChanged bool, err error) {
	return reconcileConfigMap(ctx, r.Client, configMap)
}

func (r *RpaasInstanceReconciler) getNginx(ctx context.Context, instance *v1alpha1.RpaasInstance) (*nginxv1alpha1.Nginx, error) {
	found := &nginxv1alpha1.Nginx{}
	err := r.Client.Get(ctx, types.NamespacedName{Name: instance.Name, Namespace: instance.Namespace}, found)
	if k8sErrors.IsNotFound(err) {
		return nil, err
	}
	return found, err
}

func externalAddresssesFromNginx(nginx *nginxv1alpha1.Nginx) v1alpha1.RpaasInstanceExternalAddressesStatus {
	ingressesStatus := v1alpha1.RpaasInstanceExternalAddressesStatus{}

	for _, service := range nginx.Status.Services {
		ingressesStatus.IPs = append(ingressesStatus.IPs, service.IPs...)
		ingressesStatus.Hostnames = append(ingressesStatus.Hostnames, service.Hostnames...)
	}

	for _, ingress := range nginx.Status.Ingresses {
		ingressesStatus.IPs = append(ingressesStatus.IPs, ingress.IPs...)
		ingressesStatus.Hostnames = append(ingressesStatus.Hostnames, ingress.Hostnames...)
	}

	slices.Sort(ingressesStatus.IPs)
	slices.Sort(ingressesStatus.Hostnames)

	return ingressesStatus
}

func (r *RpaasInstanceReconciler) reconcileNginx(ctx context.Context, instance *v1alpha1.RpaasInstance, nginx *nginxv1alpha1.Nginx) (hasChanged bool, certificatesHasChanges bool, err error) {
	found, err := r.getNginx(ctx, instance)
	if err != nil {
		if !k8sErrors.IsNotFound(err) {
			logrus.Errorf("Failed to get nginx CR: %v", err)
			return false, false, err
		}

		err = r.Client.Create(ctx, nginx)
		if err != nil {
			logrus.Errorf("Failed to create nginx CR: %v", err)
			return false, false, err
		}
		return true, false, nil
	}

	if equality.Semantic.DeepEqual(nginx.Spec, found.Spec) {
		return false, false, nil
	}

	// Certificates uses pod annotations, so we need to check if the pod template has changed
	certificatesHasChanges = !equality.Semantic.DeepEqual(nginx.Spec.PodTemplate.Annotations, found.Spec.PodTemplate.Annotations)

	nginx.ObjectMeta.ResourceVersion = found.ObjectMeta.ResourceVersion
	err = r.Client.Update(ctx, nginx)
	if err != nil {
		logrus.Errorf("Failed to update nginx CR: %v", err)
		return false, false, err
	}

	return true, certificatesHasChanges, nil
}

func (r *RpaasInstanceReconciler) renderTemplate(ctx context.Context, instance *v1alpha1.RpaasInstance, plan *v1alpha1.RpaasPlan, nginxTLS []nginxv1alpha1.NginxTLS) (string, error) {
	rf := &referenceFinder{
		spec:      &instance.Spec,
		client:    r.Client,
		namespace: instance.Namespace,
	}

	blocks, err := rf.getConfigurationBlocks(ctx, plan)
	if err != nil {
		return "", err
	}

	if err = rf.updateLocationValues(ctx); err != nil {
		return "", err
	}

	cr, err := nginx.NewConfigurationRenderer(blocks)
	if err != nil {
		return "", err
	}

	instanceWithNginxTLS := instance.DeepCopy()
	instanceWithNginxTLS.Spec.TLS = nginxTLS

	config := nginx.ConfigurationData{
		Instance: instanceWithNginxTLS,
		Config:   &plan.Spec.Config,
		Plan:     plan,
		NginxTLS: nginxTLS,
	}

	return cr.Render(config)
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

	return &corev1.ConfigMap{
		TypeMeta: metav1.TypeMeta{
			Kind:       "ConfigMap",
			APIVersion: "v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      fmt.Sprintf("%s-config-%s", instance.Name, hash[:10]),
			Namespace: instance.Namespace,
			Labels:    instance.GetBaseLabels(map[string]string{"type": "config", "instance": instance.Name}),
			OwnerReferences: []metav1.OwnerReference{
				*metav1.NewControllerRef(instance, schema.GroupVersionKind{
					Group:   v1alpha1.GroupVersion.Group,
					Version: v1alpha1.GroupVersion.Version,
					Kind:    "RpaasInstance",
				}),
			},
		},
		Data: map[string]string{
			"nginx.conf": renderedTemplate,
		},
	}
}

func mergeServiceWithDNS(instance *v1alpha1.RpaasInstance) *nginxv1alpha1.NginxService {
	if instance == nil {
		return nil
	}

	s := instance.Spec.Service
	if s == nil {
		return nil
	}

	if instance.Spec.DNS == nil {
		return s
	}

	if s.Annotations == nil {
		s.Annotations = make(map[string]string)
	}

	hostname := fmt.Sprintf("%s.%s", instance.Name, instance.Spec.DNS.Zone)
	if custom, found := s.Annotations[externalDNSHostnameLabel]; found {
		hostname = strings.Join([]string{hostname, custom}, ",")
	}

	s.Annotations[externalDNSHostnameLabel] = hostname

	if instance.Spec.DNS.TTL != nil {
		s.Annotations[externalDNSTTLLabel] = strconv.Itoa(int(*instance.Spec.DNS.TTL))
	}

	return s
}

type newNginxOptions struct {
	instanceMergedWithFlavors *v1alpha1.RpaasInstance
	plan                      *v1alpha1.RpaasPlan
	configMap                 *corev1.ConfigMap
	nginxTLS                  []nginxv1alpha1.NginxTLS
	certificatePodAnnotations map[string]string
}

func newNginx(opts newNginxOptions) *nginxv1alpha1.Nginx {
	var cacheConfig nginxv1alpha1.NginxCacheSpec
	if v1alpha1.BoolValue(opts.plan.Spec.Config.CacheEnabled) {
		cacheConfig.Path = opts.plan.Spec.Config.CachePath
		cacheConfig.InMemory = true
		if opts.plan.Spec.Config.CacheSize != nil && !opts.plan.Spec.Config.CacheSize.IsZero() {
			cacheConfig.Size = opts.plan.Spec.Config.CacheSize
		}
	}

	opts.instanceMergedWithFlavors.Spec.Service = mergeServiceWithDNS(opts.instanceMergedWithFlavors)

	if s := opts.instanceMergedWithFlavors.Spec.Service; s != nil {
		s.Labels = opts.instanceMergedWithFlavors.GetBaseLabels(s.Labels)
	}

	if ing := opts.instanceMergedWithFlavors.Spec.Ingress; ing != nil {
		ing.Labels = opts.instanceMergedWithFlavors.GetBaseLabels(ing.Labels)
	}

	replicas := opts.instanceMergedWithFlavors.Spec.Replicas
	if shutdown := opts.instanceMergedWithFlavors.Spec.Shutdown; shutdown {
		replicas = ptr.To(int32(0))
	}

	if isAutoscaleEnabled(&opts.instanceMergedWithFlavors.Spec) {
		// NOTE: we should avoid changing the number of replicas as it's managed by HPA.
		replicas = nil
	}

	n := &nginxv1alpha1.Nginx{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Nginx",
			APIVersion: "nginx.tsuru.io/v1alpha1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      opts.instanceMergedWithFlavors.Name,
			Namespace: opts.instanceMergedWithFlavors.Namespace,
			OwnerReferences: []metav1.OwnerReference{
				*metav1.NewControllerRef(opts.instanceMergedWithFlavors, schema.GroupVersionKind{
					Group:   v1alpha1.GroupVersion.Group,
					Version: v1alpha1.GroupVersion.Version,
					Kind:    "RpaasInstance",
				}),
			},
			Labels: opts.instanceMergedWithFlavors.GetBaseLabels(nil),
		},
		Spec: nginxv1alpha1.NginxSpec{
			Image:    opts.plan.Spec.Image,
			Replicas: replicas,
			Config: &nginxv1alpha1.ConfigRef{
				Name: opts.configMap.Name,
				Kind: nginxv1alpha1.ConfigKindConfigMap,
			},
			Resources:       opts.plan.Spec.Resources,
			Service:         opts.instanceMergedWithFlavors.Spec.Service.DeepCopy(),
			HealthcheckPath: "/_nginx_healthcheck",
			Cache:           cacheConfig,
			PodTemplate:     opts.instanceMergedWithFlavors.Spec.PodTemplate,
			Lifecycle:       opts.instanceMergedWithFlavors.Spec.Lifecycle,
			Ingress:         opts.instanceMergedWithFlavors.Spec.Ingress,
		},
	}

	if n.Spec.Service != nil && n.Spec.Service.Type == "" {
		n.Spec.Service.Type = corev1.ServiceTypeLoadBalancer
	}

	for i, f := range opts.instanceMergedWithFlavors.Spec.Files {
		volumeName := fmt.Sprintf("extra-files-%d", i)

		n.Spec.PodTemplate.Volumes = append(n.Spec.PodTemplate.Volumes, corev1.Volume{
			Name: volumeName,
			VolumeSource: corev1.VolumeSource{
				ConfigMap: &corev1.ConfigMapVolumeSource{
					LocalObjectReference: f.ConfigMap.LocalObjectReference,
				},
			},
		})

		n.Spec.PodTemplate.VolumeMounts = append(n.Spec.PodTemplate.VolumeMounts, corev1.VolumeMount{
			Name:      volumeName,
			MountPath: fmt.Sprintf("/etc/nginx/extra_files/%s", f.Name),
			SubPath:   f.Name,
			ReadOnly:  true,
		})
	}

	if isTLSSessionTicketEnabled(&opts.instanceMergedWithFlavors.Spec) {
		n.Spec.PodTemplate.Volumes = append(n.Spec.PodTemplate.Volumes, corev1.Volume{
			Name: sessionTicketsVolumeName,
			VolumeSource: corev1.VolumeSource{
				Secret: &corev1.SecretVolumeSource{
					SecretName: secretNameForTLSSessionTickets(opts.instanceMergedWithFlavors.Name),
				},
			},
		})

		n.Spec.PodTemplate.VolumeMounts = append(n.Spec.PodTemplate.VolumeMounts, corev1.VolumeMount{
			Name:      sessionTicketsVolumeName,
			MountPath: sessionTicketsVolumeMountPath,
			ReadOnly:  true,
		})
	}

	n.Spec.TLS = opts.nginxTLS

	if len(opts.certificatePodAnnotations) > 0 {
		if n.Spec.PodTemplate.Annotations == nil {
			n.Spec.PodTemplate.Annotations = make(map[string]string)
		}

		for k, v := range opts.certificatePodAnnotations {
			n.Spec.PodTemplate.Annotations[k] = v
		}
	}

	return n
}

func newNginxTLS(logger *logr.Logger, certificateSecrets []corev1.Secret, userDefinedCertificates []nginxv1alpha1.NginxTLS, certManagerCertificates []cmv1.Certificate) (podAnnotations map[string]string, tls []nginxv1alpha1.NginxTLS) {
	mapSecretNameToCertName := make(map[string]string)
	secretsByName := make(map[string]corev1.Secret)

	for _, secret := range certificateSecrets {
		secretsByName[secret.Name] = secret
		certName := secret.Labels[certificates.CertificateNameLabel]
		if certName == "" {
			logger.V(4).Info("certificate secret without certificate name label", "secret", secret.Name)
		} else {
			mapSecretNameToCertName[secret.Name] = certName
		}
	}

	podAnnotations = make(map[string]string)
	tlsByCertName := make(map[string]nginxv1alpha1.NginxTLS)
	for _, tls := range userDefinedCertificates {
		certName := mapSecretNameToCertName[tls.SecretName]
		if certName == "" {
			logger.V(4).Info("certificate secret missing for user-defined certificate", "secret", tls.SecretName)
			continue
		}
		tlsByCertName[certName] = tls

		if secret, ok := secretsByName[tls.SecretName]; ok {
			podAnnotations[certificateHashAnnotationKey(certName)] = util.SHA256(secret.Data[corev1.TLSCertKey])
			podAnnotations[keyHashAnnotationKey(certName)] = util.SHA256(secret.Data[corev1.TLSPrivateKeyKey])
		} else {
			logger.V(4).Info("certificate secret missing for user-defined certificate", "secret", tls.SecretName, "cert-name", certName)
		}
	}

	for _, cert := range certManagerCertificates {
		certName := cert.Labels[certificates.CertificateNameLabel]

		if certName == "" {
			logger.V(4).Info("cert-manager certificate without certificate name label", "certificate", cert.Name)
			continue
		}

		tlsByCertName[certName] = nginxv1alpha1.NginxTLS{
			SecretName: cert.Spec.SecretName,
			Hosts:      cert.Spec.DNSNames,
		}

		if secret, ok := secretsByName[cert.Spec.SecretName]; ok {
			podAnnotations[certificateHashAnnotationKey(cert.Name)] = util.SHA256(secret.Data[corev1.TLSCertKey])
			podAnnotations[keyHashAnnotationKey(cert.Name)] = util.SHA256(secret.Data[corev1.TLSPrivateKeyKey])
		} else {
			logger.V(4).Info("cert-manager secret is missing", "secret", cert.Spec.SecretName, "cert-name", cert.Name)
		}
	}

	for _, tlsItem := range tlsByCertName {
		tls = append(tls, tlsItem)
	}

	if len(tls) > 1 {
		sort.Slice(tls, func(i, j int) bool {
			return tls[i].SecretName < tls[j].SecretName
		})
	}

	return podAnnotations, tls
}

func generateNginxHash(nginx *nginxv1alpha1.Nginx) (string, error) {
	if nginx == nil {
		return "", nil
	}
	nginx = nginx.DeepCopy()
	nginx.Spec.Replicas = nil
	data, err := json.Marshal(nginx.Spec)
	if err != nil {
		return "", err
	}
	hash := sha256.Sum256(data)
	return strings.ToLower(base32.StdEncoding.WithPadding(base32.NoPadding).EncodeToString(hash[:])), nil
}

func certificateHashAnnotationKey(certName string) string {
	keyFormat := "rpaas.extensions.tsuru.io/%s-cert-sha256"

	// NOTE: Annotation keys must not be greater than 63 chars.
	// See more: https://kubernetes.io/docs/concepts/overview/working-with-objects/labels/#syntax-and-character-set
	maxIssuer := 63 - len(fmt.Sprintf(keyFormat, ""))

	if len(certName) > maxIssuer {
		certName = certName[:maxIssuer]
	}

	return fmt.Sprintf(keyFormat, certName)
}

func keyHashAnnotationKey(certName string) string {
	keyFormat := "rpaas.extensions.tsuru.io/%s-key-sha256"

	// NOTE: Annotation keys must not be greater than 63 chars.
	// See more: https://kubernetes.io/docs/concepts/overview/working-with-objects/labels/#syntax-and-character-set
	maxIssuer := 63 - len(fmt.Sprintf(keyFormat, ""))

	if len(certName) > maxIssuer {
		certName = certName[:maxIssuer]
	}

	return fmt.Sprintf(keyFormat, certName)
}

func generateSpecHash(spec *v1alpha1.RpaasInstanceSpec, certificatePodAnnotations map[string]string) (string, error) {
	if spec == nil {
		return "", nil
	}

	hasher := sha256.New()

	data, err := json.Marshal(spec)
	if err != nil {
		return "", err
	}
	hasher.Write(data)

	if len(certificatePodAnnotations) > 0 {
		data, err = json.Marshal(certificatePodAnnotations)
		if err != nil {
			return "", err
		}
		hasher.Write(data)

	}
	hash := hasher.Sum(nil)
	return strings.ToLower(base32.StdEncoding.WithPadding(base32.NoPadding).EncodeToString(hash)), nil
}

func buildNGNIXContainerMetrics(instance *v1alpha1.RpaasInstance) []autoscalingv2.MetricSpec {
	var metrics []autoscalingv2.MetricSpec

	if a := instance.Spec.Autoscale; a != nil && a.TargetCPUUtilizationPercentage != nil {
		metrics = append(metrics, autoscalingv2.MetricSpec{
			Type: autoscalingv2.ContainerResourceMetricSourceType,
			ContainerResource: &autoscalingv2.ContainerResourceMetricSource{
				Name:      corev1.ResourceCPU,
				Container: "nginx",
				Target: autoscalingv2.MetricTarget{
					Type:               autoscalingv2.UtilizationMetricType,
					AverageUtilization: instance.Spec.Autoscale.TargetCPUUtilizationPercentage,
				},
			},
		})
	}

	if a := instance.Spec.Autoscale; a != nil && a.TargetMemoryUtilizationPercentage != nil {
		metrics = append(metrics, autoscalingv2.MetricSpec{
			Type: autoscalingv2.ContainerResourceMetricSourceType,
			ContainerResource: &autoscalingv2.ContainerResourceMetricSource{
				Name:      corev1.ResourceMemory,
				Container: "nginx",
				Target: autoscalingv2.MetricTarget{
					Type:               autoscalingv2.UtilizationMetricType,
					AverageUtilization: instance.Spec.Autoscale.TargetMemoryUtilizationPercentage,
				},
			},
		})
	}

	return metrics
}

func buildHPAMetrics(instance *v1alpha1.RpaasInstance) []autoscalingv2.MetricSpec {
	var metrics []autoscalingv2.MetricSpec

	if len(instance.Spec.PodTemplate.Containers) > 0 {
		// only look at nginx container metrics if there are multiple containers
		return buildNGNIXContainerMetrics(instance)
	}

	if a := instance.Spec.Autoscale; a != nil && a.TargetCPUUtilizationPercentage != nil {
		metrics = append(metrics, autoscalingv2.MetricSpec{
			Type: autoscalingv2.ResourceMetricSourceType,
			Resource: &autoscalingv2.ResourceMetricSource{
				Name: corev1.ResourceCPU,
				Target: autoscalingv2.MetricTarget{
					Type:               autoscalingv2.UtilizationMetricType,
					AverageUtilization: instance.Spec.Autoscale.TargetCPUUtilizationPercentage,
				},
			},
		})
	}

	if a := instance.Spec.Autoscale; a != nil && a.TargetMemoryUtilizationPercentage != nil {
		metrics = append(metrics, autoscalingv2.MetricSpec{
			Type: autoscalingv2.ResourceMetricSourceType,
			Resource: &autoscalingv2.ResourceMetricSource{
				Name: corev1.ResourceMemory,
				Target: autoscalingv2.MetricTarget{
					Type:               autoscalingv2.UtilizationMetricType,
					AverageUtilization: instance.Spec.Autoscale.TargetMemoryUtilizationPercentage,
				},
			},
		})
	}

	return metrics
}

func newHPA(instance *v1alpha1.RpaasInstance, nginx *nginxv1alpha1.Nginx) *autoscalingv2.HorizontalPodAutoscaler {
	metrics := buildHPAMetrics(instance)

	minReplicas := instance.Spec.Replicas
	if a := instance.Spec.Autoscale; a != nil && a.MinReplicas != nil {
		minReplicas = a.MinReplicas
	}

	var maxReplicas int32
	if a := instance.Spec.Autoscale; a != nil {
		maxReplicas = a.MaxReplicas
	}

	targetResourceName := nginx.Name
	if len(nginx.Status.Deployments) > 0 {
		if n := nginx.Status.Deployments[0].Name; n != "" {
			targetResourceName = n
		}
	}

	var scalingDownRules *autoscalingv2.HPAScalingRules
	if a := instance.Spec.Autoscale; a != nil && a.Behavior != nil && a.Behavior.ScaleDown != nil {
		var scalingDown = instance.Spec.Autoscale.Behavior.ScaleDown
		var policies []autoscalingv2.HPAScalingPolicy
		if scalingDown.StabilizationWindowSeconds != nil {
			scalingDownRules.StabilizationWindowSeconds = scalingDown.StabilizationWindowSeconds
		}
		if scalingDown.UnitsPolicyValue != nil {
			policy := autoscalingv2.HPAScalingPolicy{
				Type:  autoscalingv2.PodsScalingPolicy,
				Value: *scalingDown.UnitsPolicyValue,
			}
			policies = append(policies, policy)
		}
		if scalingDown.PercentPolicyValue != nil {
			policy := autoscalingv2.HPAScalingPolicy{
				Type:  autoscalingv2.PercentScalingPolicy,
				Value: *scalingDown.PercentPolicyValue,
			}
			policies = append(policies, policy)
		}
		if policies != nil {
			scalingDownRules.Policies = policies
		}
	}

	return &autoscalingv2.HorizontalPodAutoscaler{
		TypeMeta: metav1.TypeMeta{
			Kind:       "HorizontalPodAutoscaler",
			APIVersion: "autoscaling/v2",
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
			Labels: instance.GetBaseLabels(nil),
		},
		Spec: autoscalingv2.HorizontalPodAutoscalerSpec{
			ScaleTargetRef: autoscalingv2.CrossVersionObjectReference{
				APIVersion: appsv1.SchemeGroupVersion.String(),
				Kind:       "Deployment",
				Name:       targetResourceName,
			},
			MinReplicas: minReplicas,
			MaxReplicas: maxReplicas,
			Metrics:     metrics,
			Behavior: &autoscalingv2.HorizontalPodAutoscalerBehavior{
				ScaleDown: scalingDownRules,
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
	switch t {
	case reflect.TypeOf(v1alpha1.Bool(true)):
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

	case reflect.TypeOf(corev1.ResourceList{}):
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

	case reflect.TypeOf(resource.Quantity{}):
		return func(dst, src reflect.Value) error {
			if src.IsZero() {
				return nil
			}

			if reflect.DeepEqual(src, dst) {
				return nil
			}

			if !dst.CanSet() {
				return fmt.Errorf("cannot set value to destination")
			}

			dst.Set(src)

			return nil
		}
	}

	return nil
}

func nameForCronJob(name string) string {
	const cronjobMaxChars = 52

	if len(name) <= cronjobMaxChars {
		return name
	}

	digest := util.SHA256(name)[:10]
	return name[:cronjobMaxChars-len(digest)] + digest
}
