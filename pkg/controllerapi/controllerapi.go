package controllerapi

import (
	"context"
	"encoding/json"
	"net/http"

	"github.com/tsuru/nginx-operator/api/v1alpha1"
	coreV1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/labels"
	sigsk8sclient "sigs.k8s.io/controller-runtime/pkg/client"
)

const healthcheckPath = "/_nginx_healthcheck"

type prometheusDiscoverHandler struct {
	client sigsk8sclient.Client
}

func (h *prometheusDiscoverHandler) svcMap(ctx context.Context) (map[sigsk8sclient.ObjectKey]*coreV1.Service, error) {
	svcMap := map[sigsk8sclient.ObjectKey]*coreV1.Service{}
	allNginxServices := &coreV1.ServiceList{}
	err := h.client.List(ctx, allNginxServices, &sigsk8sclient.ListOptions{
		LabelSelector: labels.SelectorFromSet(labels.Set{
			"nginx.tsuru.io/app": "nginx",
		}),
	})

	if err != nil {
		return nil, err
	}

	for i, svc := range allNginxServices.Items {
		svcMap[sigsk8sclient.ObjectKeyFromObject(&svc)] = &allNginxServices.Items[i]
	}

	return svcMap, nil
}

func rpaasTargetGroups(svcMap map[sigsk8sclient.ObjectKey]*coreV1.Service, nginxInstance *v1alpha1.Nginx) []TargetGroup {
	targetGroups := []TargetGroup{}

	for _, service := range nginxInstance.Status.Services {

		svc := svcMap[sigsk8sclient.ObjectKey{
			Namespace: nginxInstance.Namespace,
			Name:      service.Name,
		}]

		if svc == nil {
			continue
		}

		if len(svc.Status.LoadBalancer.Ingress) == 0 {
			continue
		}

		svcIP := svc.Status.LoadBalancer.Ingress[0].IP

		namespace := svc.ObjectMeta.Namespace
		serviceInstance := svc.Labels["rpaas.extensions.tsuru.io/instance-name"]
		service := svc.Labels["rpaas.extensions.tsuru.io/service-name"]
		teamOwner := svc.Labels["rpaas.extensions.tsuru.io/team-owner"]

		targetGroups = append(targetGroups, TargetGroup{
			Targets: []string{
				"http://" + svcIP + healthcheckPath,
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
						"https://" + svcIP + healthcheckPath,
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
	svcMap, err := h.svcMap(ctx)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	allNginx := &v1alpha1.NginxList{}

	err = h.client.List(ctx, allNginx, &sigsk8sclient.ListOptions{})
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	targetGroups := []TargetGroup{}
	for _, nginxInstance := range allNginx.Items {
		targetGroups = append(targetGroups, rpaasTargetGroups(svcMap, &nginxInstance)...)
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
