package controllerapi

import (
	"encoding/json"
	"net/http"

	"github.com/tsuru/nginx-operator/api/v1alpha1"
	sigsk8sclient "sigs.k8s.io/controller-runtime/pkg/client"
)

const healthcheckPath = "/_nginx_healthcheck"

type prometheusDiscoverHandler struct {
	client sigsk8sclient.Client
}

func rpaasTargetGroups(nginxInstance *v1alpha1.Nginx) []TargetGroup {
	targetGroups := []TargetGroup{}
	ips := []string{}

	for _, service := range nginxInstance.Status.Services {
		ips = append(ips, service.IPs...)
	}

	for _, ingress := range nginxInstance.Status.Ingresses {
		ips = append(ips, ingress.IPs...)
	}

	for _, ip := range ips {
		namespace := nginxInstance.ObjectMeta.Namespace
		serviceInstance := nginxInstance.Labels["rpaas.extensions.tsuru.io/instance-name"]
		service := nginxInstance.Labels["rpaas.extensions.tsuru.io/service-name"]
		teamOwner := nginxInstance.Labels["rpaas.extensions.tsuru.io/team-owner"]

		targetGroups = append(targetGroups, TargetGroup{
			Targets: []string{
				"http://" + ip + healthcheckPath,
			},
			Labels: map[string]string{
				"namespace":        namespace,
				"service_instance": serviceInstance,
				"service":          service,
				"team_owner":       teamOwner,
			},
		})

		for _, tls := range nginxInstance.Spec.TLS {
			for _, host := range tls.Hosts {
				targetGroups = append(targetGroups, TargetGroup{
					Targets: []string{
						"https://" + ip + healthcheckPath,
					},
					Labels: map[string]string{
						"namespace":        namespace,
						"service_instance": serviceInstance,
						"service":          service,
						"servername":       host,
						"team_owner":       teamOwner,
					},
				})
			}
		}
	}

	return targetGroups
}

// TargetGroup is a collection of related hosts that prometheus monitors
type TargetGroup struct {
	Targets []string          `json:"targets"`
	Labels  map[string]string `json:"labels"`
}

func (h *prometheusDiscoverHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	allNginx := &v1alpha1.NginxList{}

	err := h.client.List(ctx, allNginx, &sigsk8sclient.ListOptions{})
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	targetGroups := []TargetGroup{}
	for _, nginxInstance := range allNginx.Items {
		targetGroups = append(targetGroups, rpaasTargetGroups(&nginxInstance)...)
	}

	w.Header().Set("Content-Type", "application/json")
	encoder := json.NewEncoder(w)
	encoder.SetIndent(" ", " ")
	err = encoder.Encode(targetGroups)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func New(client sigsk8sclient.Client) http.Handler {
	mux := http.NewServeMux()
	mux.Handle("/v1/prometheus/discover", &prometheusDiscoverHandler{client: client})

	return mux
}
