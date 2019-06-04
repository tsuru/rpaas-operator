package v1alpha1

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// NginxSpec defines the desired state of Nginx
type NginxSpec struct {
	// Number of desired pods. This is a pointer to distinguish between explicit
	// zero and not specified. Defaults to the default deployment replicas value.
	// +optional
	Replicas *int32 `json:"replicas"`
	// Docker image name. Defaults to "nginx:latest".
	// +optional
	Image string `json:"image"`
	// Reference to the nginx config object.
	Config *ConfigRef `json:"configRef"`
	// Certificates refers to a Secret containing one or more certificate-key
	// pairs.
	// +optional
	Certificates *TLSSecret `json:"certificates,omitempty"`
	// Template used to configure the nginx pod.
	// +optional
	PodTemplate NginxPodTemplateSpec `json:"podTemplate,omitempty"`
	// Service to expose the nginx pod
	// +optional
	Service *NginxService `json:"service,omitempty"`
	// ExtraFiles references to additional files into a object in the cluster.
	// These additional files will be mounted on `/etc/nginx/extra_files`.
	// +optional
	ExtraFiles *FilesRef `json:"extraFiles,omitempty"`
	// HealthcheckPath defines the endpoint used to check whether instance is
	// working or not.
	// +optional
	HealthcheckPath string `json:"healthcheckPath,omitempty"`
}

type NginxPodTemplateSpec struct {
	// Resources requirements to be set on the nginx container.
	// +optional
	Resources corev1.ResourceRequirements `json:"resources,omitempty"`
	// Affinity to be set on the nginx pod.
	// +optional
	Affinity *corev1.Affinity `json:"affinity,omitempty"`
}

// NginxStatus defines the observed state of Nginx
type NginxStatus struct {
	Pods     []PodStatus     `json:"pods,omitempty"`
	Services []ServiceStatus `json:"services,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// Nginx is the Schema for the nginxes API
// +k8s:openapi-gen=true
type Nginx struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   NginxSpec   `json:"spec,omitempty"`
	Status NginxStatus `json:"status,omitempty"`
}

type PodStatus struct {
	// Name is the name of the POD running nginx
	Name string `json:"name"`
	// PodIP is the IP if the POD
	PodIP string `json:"podIP"`
}

type ServiceStatus struct {
	// Name is the name of the Service created by nginx
	Name string `json:"name"`
}

type NginxService struct {
	// Type is the type of the service
	Type corev1.ServiceType `json:"type"`
	// LoadBalancerIP is an optional load balancer IP for the service
	LoadBalancerIP string `json:"loadBalancerIP"`
	// Labels are extra labels for the service
	Labels map[string]string `json:"labels"`
	// Annotations are extra annotations for the service
	Annotations map[string]string `json:"annotations"`
}

// ConfigRef is a reference to a config object.
type ConfigRef struct {
	// Name of the config object.
	Name string `json:"name"`
	// Kind of the config object. Defaults to ConfigKindConfigMap.
	Kind ConfigKind `json:"kind"`
	// Optional value used by some ConfigKinds.
	Value string `json:"value"`
}

type ConfigKind string

const (
	// ConfigKindConfigMap is a Kind of configuration that points to a configmap
	ConfigKindConfigMap = ConfigKind("ConfigMap")
	// ConfigKindInline is a kinda of configuration that is setup as a annotation on the Pod
	// and is inject as a file on the container using the Downward API.
	ConfigKindInline = ConfigKind("Inline")
)

// TLSSecret is a reference to TLS certificate and key pairs stored in a Secret.
type TLSSecret struct {
	// SecretName refers to the Secret holding the certificates and keys pairs.
	SecretName string `json:"secretName"`
	// Items maps the key and path where the certificate-key pairs should be
	// mounted on nginx container.
	Items []TLSSecretItem `json:"items"`
}

// TLSSecretItem maps each certificate and key pair against a key-value data
// from a Secret object.
type TLSSecretItem struct {
	// CertificateField is the field name that contains the certificate.
	CertificateField string `json:"certificateField"`
	// CertificatePath holds the path where the certificate should be stored
	// inside the nginx container. Defaults to same as CertificatedField.
	// +optional
	CertificatePath string `json:"certificatePath,omitempty"`
	// KeyField is the field name that contains the key.
	KeyField string `json:"keyField"`
	// KeyPath holds the path where the key should be store inside the nginx
	// container. Defaults to same as KeyField.
	// +optional
	KeyPath string `json:"keyPath,omitempty"`
}

// FilesRef is a reference to arbitrary files stored into a ConfigMap in the
// cluster.
type FilesRef struct {
	// Name points to a ConfigMap resource (in the same namespace) which holds
	// the files.
	Name string `json:"name"`
	// Files maps each key entry from the ConfigMap to its relative location on
	// the nginx filesystem.
	// +optional
	Files map[string]string `json:"files,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// NginxList contains a list of Nginx
type NginxList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Nginx `json:"items"`
}

func init() {
	SchemeBuilder.Register(&Nginx{}, &NginxList{})
}
