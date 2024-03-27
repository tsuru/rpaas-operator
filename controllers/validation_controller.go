package controllers

import (
	"context"
	"fmt"

	"github.com/go-logr/logr"
	"github.com/tsuru/rpaas-operator/api/v1alpha1"
	"github.com/tsuru/rpaas-operator/internal/pkg/rpaas/nginx"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/equality"
	k8sErrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/pointer"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

// RpaasValidationReconciler reconciles a RpaasValidation object
type RpaasValidationReconciler struct {
	client.Client
	Log logr.Logger
}

func (r *RpaasValidationReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	validation, err := r.getRpaasValidation(ctx, req.NamespacedName)
	if k8sErrors.IsNotFound(err) {
		return reconcile.Result{}, nil
	}

	logger := r.Log.WithName("Reconcile").
		WithValues("RpaasValidation", types.NamespacedName{Name: validation.Name, Namespace: validation.Namespace})

	if err != nil {
		return reconcile.Result{}, err
	}

	validationHash, err := generateSpecHash(&validation.Spec)
	if err != nil {
		return reconcile.Result{}, err
	}

	if validation.Status.RevisionHash == validationHash && validation.Status.Valid != nil {
		fmt.Println("job solved")
		return reconcile.Result{}, nil
	}

	planName := types.NamespacedName{
		Name:      validation.Spec.PlanName,
		Namespace: validation.Namespace,
	}
	if validation.Spec.PlanNamespace != "" {
		planName.Namespace = validation.Spec.PlanNamespace
	}

	plan := &v1alpha1.RpaasPlan{}
	err = r.Client.Get(ctx, planName, plan)
	if err != nil {
		return reconcile.Result{}, err
	}

	validationMergedWithFlavors, err := r.mergeWithFlavors(ctx, validation.DeepCopy())
	if err != nil {
		return reconcile.Result{}, nil
	}

	if validationMergedWithFlavors.Spec.PlanTemplate != nil {
		plan.Spec, err = mergePlans(plan.Spec, *validationMergedWithFlavors.Spec.PlanTemplate)
		if err != nil {
			return reconcile.Result{}, err
		}
	}

	rendered, err := r.renderTemplate(ctx, validationMergedWithFlavors, plan)
	if err != nil {
		return reconcile.Result{}, err
	}

	configMap := newValidationConfigMap(validationMergedWithFlavors, rendered)
	_, err = reconcileConfigMap(ctx, r.Client, configMap)
	if err != nil {
		return reconcile.Result{}, err
	}

	// TODO delete old config

	pod := newValidationPod(validationMergedWithFlavors, plan, configMap)

	existingPod, err := r.getPod(ctx, pod.Namespace, pod.Name)
	if err != nil && !k8sErrors.IsNotFound(err) {
		return ctrl.Result{}, nil
	}

	if existingPod != nil {

		if existingPod.Status.Phase == corev1.PodSucceeded && len(existingPod.Status.ContainerStatuses) > 0 {
			containerStatus := existingPod.Status.ContainerStatuses[0]

			if containerStatus.State.Terminated != nil {
				if containerStatus.State.Terminated.ExitCode == 0 {
					validation.Status.RevisionHash = validationHash
					validation.Status.ObservedGeneration = validation.ObjectMeta.Generation
					validation.Status.Valid = pointer.Bool(true)
					validation.Status.Error = ""

					err = r.Client.Status().Update(ctx, validation)
					if err != nil {
						return ctrl.Result{}, err
					}

					err = r.Client.Delete(ctx, existingPod)
					if err != nil {
						return ctrl.Result{}, err
					}

					// TODO: delete config-map
					return ctrl.Result{}, nil
				}
			}

		}

		if existingPod.Status.Phase == corev1.PodFailed && len(existingPod.Status.ContainerStatuses) > 0 {
			containerStatus := existingPod.Status.ContainerStatuses[0]

			if containerStatus.State.Terminated != nil {
				if containerStatus.State.Terminated.ExitCode != 0 {
					validation.Status.RevisionHash = validationHash
					validation.Status.ObservedGeneration = validation.ObjectMeta.Generation
					validation.Status.Valid = pointer.Bool(false)
					validation.Status.Error = containerStatus.State.Terminated.Message

					err = r.Client.Status().Update(ctx, validation)
					if err != nil {
						return ctrl.Result{}, err
					}

					err = r.Client.Delete(ctx, existingPod)
					if err != nil {
						return ctrl.Result{}, err
					}

					// TODO: delete config-map
					return ctrl.Result{}, nil
				}
			}

		}

	}

	_, err = r.reconcilePod(ctx, pod)
	if err != nil {
		return ctrl.Result{}, err
	}

	logger.Info("TODO? reconcile the job")

	return ctrl.Result{}, nil
}

func (r *RpaasValidationReconciler) getRpaasValidation(ctx context.Context, objKey types.NamespacedName) (*v1alpha1.RpaasValidation, error) {
	var instance v1alpha1.RpaasValidation
	if err := r.Client.Get(ctx, objKey, &instance); err != nil {
		return nil, err
	}

	return &instance, nil
}

func (r *RpaasValidationReconciler) mergeWithFlavors(ctx context.Context, validation *v1alpha1.RpaasValidation) (*v1alpha1.RpaasValidation, error) {
	mergedValidation, err := r.mergeValidationWithFlavors(ctx, validation)
	if err != nil {
		return nil, err
	}

	// NOTE: preventing this merged resource be persisted on k8s api server.
	mergedValidation.ResourceVersion = ""

	return mergedValidation, nil
}

func (r *RpaasValidationReconciler) mergeValidationWithFlavors(ctx context.Context, validation *v1alpha1.RpaasValidation) (*v1alpha1.RpaasValidation, error) {
	defaultFlavors, err := r.listDefaultFlavors(ctx, validation)
	if err != nil {
		return nil, err
	}

	for _, defaultFlavor := range defaultFlavors {
		if err := mergeValidationWithFlavor(validation, defaultFlavor); err != nil {
			return nil, err
		}
	}

	for _, flavorName := range validation.Spec.Flavors {
		flavorObjectKey := types.NamespacedName{
			Name:      flavorName,
			Namespace: validation.Namespace,
		}

		if validation.Spec.PlanNamespace != "" {
			flavorObjectKey.Namespace = validation.Spec.PlanNamespace
		}

		var flavor v1alpha1.RpaasFlavor
		if err := r.Client.Get(ctx, flavorObjectKey, &flavor); err != nil {
			return nil, err
		}

		if flavor.Spec.Default {
			continue
		}

		if err := mergeValidationWithFlavor(validation, flavor); err != nil {
			return nil, err
		}
	}

	return validation, nil
}

func (r *RpaasValidationReconciler) listDefaultFlavors(ctx context.Context, instance *v1alpha1.RpaasValidation) ([]v1alpha1.RpaasFlavor, error) {
	flavorNamespace := instance.Namespace
	if instance.Spec.PlanNamespace != "" {
		flavorNamespace = instance.Spec.PlanNamespace
	}

	return listDefaultFlavors(ctx, r.Client, flavorNamespace)
}

func mergeValidationWithFlavor(validation *v1alpha1.RpaasValidation, flavor v1alpha1.RpaasFlavor) error {
	if flavor.Spec.InstanceTemplate == nil {
		return nil
	}

	mergedInstanceSpec, err := mergeInstance(validation.Spec, *flavor.Spec.InstanceTemplate)
	if err != nil {
		return err
	}
	validation.Spec = mergedInstanceSpec
	return nil
}

func (r *RpaasValidationReconciler) renderTemplate(ctx context.Context, validation *v1alpha1.RpaasValidation, plan *v1alpha1.RpaasPlan) (string, error) {
	rf := &referenceFinder{
		spec:      &validation.Spec,
		client:    r.Client,
		namespace: validation.Namespace,
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

	config := nginx.ConfigurationData{
		Instance: &v1alpha1.RpaasInstance{
			Spec: validation.Spec,
		},
		Config: &plan.Spec.Config,
	}

	return cr.Render(config)
}

func newValidationConfigMap(validation *v1alpha1.RpaasValidation, renderedTemplate string) *corev1.ConfigMap {
	//hash := fmt.Sprintf("%x", sha256.Sum256([]byte(renderedTemplate)))

	return &corev1.ConfigMap{
		TypeMeta: metav1.TypeMeta{
			Kind:       "ConfigMap",
			APIVersion: "v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      fmt.Sprintf("validation-%s", validation.Name),
			Namespace: validation.Namespace,
			Labels: map[string]string{
				v1alpha1.RpaasOperatorValidationNameLabelKey: validation.Name,
			},
			OwnerReferences: []metav1.OwnerReference{
				*metav1.NewControllerRef(validation, schema.GroupVersionKind{
					Group:   v1alpha1.GroupVersion.Group,
					Version: v1alpha1.GroupVersion.Version,
					Kind:    "RpaasValidation",
				}),
			},
		},
		Data: map[string]string{
			"nginx.conf": renderedTemplate,
		},
	}
}

func (r *RpaasValidationReconciler) getPod(ctx context.Context, namespace, name string) (*corev1.Pod, error) {
	var existing corev1.Pod
	err := r.Client.Get(ctx, types.NamespacedName{
		Name:      name,
		Namespace: namespace,
	}, &existing)

	if err != nil {
		return nil, err
	}

	return &existing, nil
}

func (r *RpaasValidationReconciler) reconcilePod(ctx context.Context, pod *corev1.Pod) (hasChanged bool, err error) {
	existing, err := r.getPod(ctx, pod.Namespace, pod.Name)
	if err != nil && k8sErrors.IsNotFound(err) {
		err = r.Client.Create(ctx, pod)
		if err != nil {
			return false, err
		}

		return true, nil
	} else if err != nil {
		return false, err
	}

	if equality.Semantic.DeepDerivative(pod.Spec, existing.Spec) {
		return false, nil
	}

	err = r.Client.Delete(ctx, pod)
	if err != nil {
		return false, err
	}

	err = r.Client.Create(ctx, pod)
	if err != nil {
		return false, err
	}

	return true, nil
}

func newValidationPod(validationMergedWithFlavors *v1alpha1.RpaasValidation, plan *v1alpha1.RpaasPlan, configMap *corev1.ConfigMap) *corev1.Pod {
	n := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      configMap.Name,
			Namespace: validationMergedWithFlavors.Namespace,
			OwnerReferences: []metav1.OwnerReference{
				*metav1.NewControllerRef(validationMergedWithFlavors, schema.GroupVersionKind{
					Group:   v1alpha1.GroupVersion.Group,
					Version: v1alpha1.GroupVersion.Version,
					Kind:    "RpaasValidation",
				}),
			},
		},
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{
				{
					Name:         "validation",
					Image:        plan.Spec.Image,
					VolumeMounts: validationMergedWithFlavors.Spec.PodTemplate.VolumeMounts,
					Command: []string{
						"/bin/sh",
						"-c",
						"nginx -t 2> /dev/termination-log",
					},
				},
			},
			RestartPolicy: "Never",
			Volumes:       validationMergedWithFlavors.Spec.PodTemplate.Volumes,
		},
	}

	for i, f := range validationMergedWithFlavors.Spec.Files {
		volumeName := fmt.Sprintf("extra-files-%d", i)

		n.Spec.Volumes = append(n.Spec.Volumes, corev1.Volume{
			Name: volumeName,
			VolumeSource: corev1.VolumeSource{
				ConfigMap: &corev1.ConfigMapVolumeSource{
					LocalObjectReference: f.ConfigMap.LocalObjectReference,
				},
			},
		})

		n.Spec.Containers[0].VolumeMounts = append(n.Spec.Containers[0].VolumeMounts, corev1.VolumeMount{
			Name:      volumeName,
			MountPath: fmt.Sprintf("/etc/nginx/extra_files/%s", f.Name),
			SubPath:   f.Name,
			ReadOnly:  true,
		})
	}

	volumeName := "nginx-config"
	configMountPath := "/etc/nginx"
	configFileName := "nginx.conf"

	n.Spec.Containers[0].VolumeMounts = append(n.Spec.Containers[0].VolumeMounts, corev1.VolumeMount{
		Name:      volumeName,
		MountPath: fmt.Sprintf("%s/%s", configMountPath, configFileName),
		SubPath:   configFileName,
		ReadOnly:  true,
	})

	n.Spec.Volumes = append(n.Spec.Volumes, corev1.Volume{
		Name: volumeName,
		VolumeSource: corev1.VolumeSource{
			ConfigMap: &corev1.ConfigMapVolumeSource{
				LocalObjectReference: corev1.LocalObjectReference{
					Name: configMap.Name,
				},
				Optional: pointer.Bool(false),
			},
		},
	})

	if plan.Spec.Config.CacheEnabled != nil && *plan.Spec.Config.CacheEnabled {
		n.Spec.Containers[0].VolumeMounts = append(n.Spec.Containers[0].VolumeMounts, corev1.VolumeMount{
			Name:      "cache-vol",
			MountPath: plan.Spec.Config.CachePath,
		})

		n.Spec.Volumes = append(n.Spec.Volumes, corev1.Volume{
			Name: "cache-vol",
			VolumeSource: corev1.VolumeSource{
				EmptyDir: &corev1.EmptyDirVolumeSource{
					Medium: corev1.StorageMediumMemory,
				},
			},
		})
	}

	return n
}

func (r *RpaasValidationReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&v1alpha1.RpaasValidation{}).
		Owns(&corev1.Pod{}).
		Complete(r)
}
