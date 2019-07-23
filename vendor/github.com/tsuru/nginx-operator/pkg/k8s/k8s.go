package k8s

import (
	"encoding/json"
	"fmt"
	"net/url"
	"sort"

	tsuruConfig "github.com/tsuru/config"
	"github.com/tsuru/nginx-operator/pkg/apis/nginx/v1alpha1"
	appv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/intstr"
)

const (
	// Default docker image used for nginx
	defaultNginxImage = "nginx:latest"

	// Default port names used by the nginx container and the ClusterIP service
	defaultHTTPPort     = int32(8080)
	defaultHTTPPortName = "http"

	defaultHTTPSPort     = int32(8443)
	defaultHTTPSPortName = "https"

	// Path and port to the healthcheck service
	healthcheckPort        = 59999
	healthcheckPath        = "/healthcheck"
	healthcheckSidecarName = "nginx-healthchecker"

	defaultSidecarContainerImage = "tsuru/nginx-operator-sidecar:latest"

	// Mount path where nginx.conf will be placed
	configMountPath = "/etc/nginx"

	// Default configuration filename of nginx
	configFileName = "nginx.conf"

	// Mount path where certificate and key pair will be placed
	certMountPath = configMountPath + "/certs"

	// Mount path where the additional files will be mounted on
	extraFilesMountPath = configMountPath + "/extra_files"

	// Annotation key used to stored the nginx that created the deployment
	generatedFromAnnotation = "nginx.tsuru.io/generated-from"
)

var nginxEntrypoint = []string{
	"/bin/sh",
	"-c",
	"while ! [ -f /tmp/done ]; do sleep 0.5; done && nginx -g 'daemon off;'",
}

var postStartCommand = []string{
	"/bin/sh",
	"-c",
	"nginx -t && touch /tmp/done",
}

// NewDeployment creates a deployment for a given Nginx resource.
func NewDeployment(n *v1alpha1.Nginx) (*appv1.Deployment, error) {
	n.Spec.Image = valueOrDefault(n.Spec.Image, defaultNginxImage)
	customSidecarContainerImage, _ := tsuruConfig.GetString("nginx-controller:sidecar:image")
	labels := mergeMap(n.Spec.PodTemplate.Labels, LabelsForNginx(n.Name))
	deployment := appv1.Deployment{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Deployment",
			APIVersion: "apps/v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      n.Name,
			Namespace: n.Namespace,
			OwnerReferences: []metav1.OwnerReference{
				*metav1.NewControllerRef(n, schema.GroupVersionKind{
					Group:   v1alpha1.SchemeGroupVersion.Group,
					Version: v1alpha1.SchemeGroupVersion.Version,
					Kind:    "Nginx",
				}),
			},
		},
		Spec: appv1.DeploymentSpec{
			Replicas: n.Spec.Replicas,
			Selector: &metav1.LabelSelector{
				MatchLabels: labels,
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Namespace:   n.Namespace,
					Annotations: n.Spec.PodTemplate.Annotations,
					Labels:      labels,
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Name:    "nginx",
							Image:   n.Spec.Image,
							Command: nginxEntrypoint,
							Ports: []corev1.ContainerPort{
								{
									Name:          defaultHTTPPortName,
									ContainerPort: defaultHTTPPort,
									Protocol:      corev1.ProtocolTCP,
								},
								{
									Name:          defaultHTTPSPortName,
									ContainerPort: defaultHTTPSPort,
									Protocol:      corev1.ProtocolTCP,
								},
							},
							Resources: n.Spec.Resources,
							ReadinessProbe: &corev1.Probe{
								Handler: corev1.Handler{
									HTTPGet: &corev1.HTTPGetAction{
										Path:   buildHealthcheckPath(n.Spec),
										Port:   intstr.FromInt(healthcheckPort),
										Scheme: corev1.URISchemeHTTP,
									},
								},
							},
							Lifecycle: &corev1.Lifecycle{
								PostStart: &corev1.Handler{
									Exec: &corev1.ExecAction{
										Command: postStartCommand,
									},
								},
							},
						},
						{
							Name:  healthcheckSidecarName,
							Image: valueOrDefault(customSidecarContainerImage, defaultSidecarContainerImage),
						},
					},
					Affinity: n.Spec.PodTemplate.Affinity,
				},
			},
		},
	}
	setupConfig(n.Spec.Config, &deployment)
	setupTLS(n.Spec.Certificates, &deployment)
	setupExtraFiles(n.Spec.ExtraFiles, &deployment)

	// This is done on the last step because n.Spec may have mutated during these methods
	if err := SetNginxSpec(&deployment.ObjectMeta, n.Spec); err != nil {
		return nil, err
	}

	return &deployment, nil
}

func mergeMap(a, b map[string]string) map[string]string {
	if a == nil {
		return b
	}
	for k, v := range b {
		a[k] = v
	}
	return a
}

// NewService assembles the ClusterIP service for the Nginx
func NewService(n *v1alpha1.Nginx) *corev1.Service {
	var labels, annotations map[string]string
	var lbIP string
	if n.Spec.Service != nil {
		labels = n.Spec.Service.Labels
		annotations = n.Spec.Service.Annotations
		lbIP = n.Spec.Service.LoadBalancerIP
	}
	service := corev1.Service{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Service",
			APIVersion: "v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      n.Name + "-service",
			Namespace: n.Namespace,
			OwnerReferences: []metav1.OwnerReference{
				*metav1.NewControllerRef(n, schema.GroupVersionKind{
					Group:   v1alpha1.SchemeGroupVersion.Group,
					Version: v1alpha1.SchemeGroupVersion.Version,
					Kind:    "Nginx",
				}),
			},
			Labels:      mergeMap(labels, LabelsForNginx(n.Name)),
			Annotations: annotations,
		},
		Spec: corev1.ServiceSpec{
			Ports: []corev1.ServicePort{
				{
					Name:       defaultHTTPPortName,
					Protocol:   corev1.ProtocolTCP,
					TargetPort: intstr.FromString(defaultHTTPPortName),
					Port:       int32(80),
				},
				{
					Name:       defaultHTTPSPortName,
					Protocol:   corev1.ProtocolTCP,
					TargetPort: intstr.FromString(defaultHTTPSPortName),
					Port:       int32(443),
				},
			},
			Selector:       LabelsForNginx(n.Name),
			LoadBalancerIP: lbIP,
			Type:           nginxService(n),
		},
	}
	return &service
}

func nginxService(n *v1alpha1.Nginx) corev1.ServiceType {
	if n == nil || n.Spec.Service == nil {
		return corev1.ServiceTypeClusterIP
	}
	return corev1.ServiceType(n.Spec.Service.Type)
}

// LabelsForNginx returns the labels for a Nginx CR with the given name
func LabelsForNginx(name string) map[string]string {
	return map[string]string{
		"nginx_cr": name,
		"app":      "nginx",
	}
}

// ExtractNginxSpec extracts the nginx used to create the object
func ExtractNginxSpec(o metav1.ObjectMeta) (v1alpha1.NginxSpec, error) {
	ann, ok := o.Annotations[generatedFromAnnotation]
	if !ok {
		return v1alpha1.NginxSpec{}, fmt.Errorf("missing %q annotation in deployment", generatedFromAnnotation)
	}
	var spec v1alpha1.NginxSpec
	if err := json.Unmarshal([]byte(ann), &spec); err != nil {
		return v1alpha1.NginxSpec{}, fmt.Errorf("failed to unmarshal nginx from annotation: %v", err)
	}
	return spec, nil
}

// SetNginxSpec sets the nginx spec into the object annotation to be later extracted
func SetNginxSpec(o *metav1.ObjectMeta, spec v1alpha1.NginxSpec) error {
	if o.Annotations == nil {
		o.Annotations = make(map[string]string)
	}
	origSpec, err := json.Marshal(spec)
	if err != nil {
		return err
	}
	o.Annotations[generatedFromAnnotation] = string(origSpec)
	return nil
}

func buildHealthcheckPath(spec v1alpha1.NginxSpec) string {
	httpURL := fmt.Sprintf("http://localhost:%d%s", defaultHTTPPort, spec.HealthcheckPath)

	query := url.Values{}
	query.Add("url", httpURL)

	if spec.Certificates != nil {
		httpsURL := fmt.Sprintf("https://localhost:%d%s", defaultHTTPSPort, spec.HealthcheckPath)
		query.Add("url", httpsURL)
	}

	return fmt.Sprintf("%s?%s", healthcheckPath, query.Encode())
}

func setupConfig(conf *v1alpha1.ConfigRef, dep *appv1.Deployment) {
	if conf == nil {
		return
	}
	dep.Spec.Template.Spec.Containers[0].VolumeMounts = append(dep.Spec.Template.Spec.Containers[0].VolumeMounts, corev1.VolumeMount{
		Name:      "nginx-config",
		MountPath: fmt.Sprintf("%s/%s", configMountPath, configFileName),
		SubPath:   configFileName,
	})
	switch conf.Kind {
	case v1alpha1.ConfigKindConfigMap:
		dep.Spec.Template.Spec.Volumes = append(dep.Spec.Template.Spec.Volumes, corev1.Volume{
			Name: "nginx-config",
			VolumeSource: corev1.VolumeSource{
				ConfigMap: &corev1.ConfigMapVolumeSource{
					LocalObjectReference: corev1.LocalObjectReference{
						Name: conf.Name,
					},
				},
			},
		})
	case v1alpha1.ConfigKindInline:
		// FIXME: inline content is being written out of order
		if dep.Spec.Template.Annotations == nil {
			dep.Spec.Template.Annotations = make(map[string]string)
		}
		dep.Spec.Template.Annotations[conf.Name] = conf.Value
		dep.Spec.Template.Spec.Volumes = append(dep.Spec.Template.Spec.Volumes, corev1.Volume{
			Name: "nginx-config",
			VolumeSource: corev1.VolumeSource{
				DownwardAPI: &corev1.DownwardAPIVolumeSource{
					Items: []corev1.DownwardAPIVolumeFile{
						{
							Path: "nginx.conf",
							FieldRef: &corev1.ObjectFieldSelector{
								FieldPath: fmt.Sprintf("metadata.annotations['%s']", conf.Name),
							},
						},
					},
				},
			},
		})
	}
}

// setupTLS appends an https port if TLS secrets are specified
func setupTLS(secret *v1alpha1.TLSSecret, dep *appv1.Deployment) {
	if secret == nil {
		return
	}

	dep.Spec.Template.Spec.Containers[0].VolumeMounts = append(dep.Spec.Template.Spec.Containers[0].VolumeMounts, corev1.VolumeMount{
		Name:      "nginx-certs",
		MountPath: certMountPath,
	})

	var items []corev1.KeyToPath
	for _, item := range secret.Items {
		items = append(items, corev1.KeyToPath{
			Key:  item.CertificateField,
			Path: valueOrDefault(item.CertificatePath, item.CertificateField),
		}, corev1.KeyToPath{
			Key:  item.KeyField,
			Path: valueOrDefault(item.KeyPath, item.KeyField),
		})
	}

	dep.Spec.Template.Spec.Volumes = append(dep.Spec.Template.Spec.Volumes, corev1.Volume{
		Name: "nginx-certs",
		VolumeSource: corev1.VolumeSource{
			Secret: &corev1.SecretVolumeSource{
				SecretName: secret.SecretName,
				Items:      items,
			},
		},
	})
}

// setupExtraFiles configures the volume source and mount into Deployment resource.
func setupExtraFiles(fRef *v1alpha1.FilesRef, dep *appv1.Deployment) {
	if fRef == nil {
		return
	}
	volumeMountName := "nginx-extra-files"
	dep.Spec.Template.Spec.Containers[0].VolumeMounts = append(dep.Spec.Template.Spec.Containers[0].VolumeMounts, corev1.VolumeMount{
		Name:      volumeMountName,
		MountPath: extraFilesMountPath,
	})
	var items []corev1.KeyToPath
	for key, path := range fRef.Files {
		items = append(items, corev1.KeyToPath{
			Key:  key,
			Path: path,
		})
	}
	// putting the items in a deterministic order to allow tests
	if items != nil {
		sort.Slice(items, func(i, j int) bool {
			return items[i].Key < items[j].Key
		})
	}
	dep.Spec.Template.Spec.Volumes = append(dep.Spec.Template.Spec.Volumes, corev1.Volume{
		Name: volumeMountName,
		VolumeSource: corev1.VolumeSource{
			ConfigMap: &corev1.ConfigMapVolumeSource{
				LocalObjectReference: corev1.LocalObjectReference{
					Name: fRef.Name,
				},
				Items: items,
			},
		},
	})
}

func valueOrDefault(value, def string) string {
	if value != "" {
		return value
	}
	return def
}
