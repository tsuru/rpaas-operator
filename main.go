// Copyright 2020 tsuru authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"flag"
	"log"
	"net/http"
	"os"
	"text/template"
	"time"

	_ "k8s.io/client-go/plugin/pkg/client/auth/gcp"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"

	"github.com/tsuru/rpaas-operator/controllers"
	"github.com/tsuru/rpaas-operator/pkg/controllerapi"
	extensionsruntime "github.com/tsuru/rpaas-operator/pkg/runtime"
	// +kubebuilder:scaffold:imports
)

var setupLog = ctrl.Log.WithName("setup")

type configOpts struct {
	metricsAddr                    string
	healthAddr                     string
	internalAPIAddr                string
	leaderElection                 bool
	leaderElectionNamespace        string
	leaderElectionResourceName     string
	namespace                      string
	syncPeriod                     time.Duration
	autoscalingController          string
	kedaPrometheusServerAddress    string
	kedaPrometheusRPSQueryTemplate string
}

func (o *configOpts) bindFlags(fs *flag.FlagSet) {
	// Following the standard of flags on Kubernetes.
	// See more: https://github.com/kubernetes-sigs/kubebuilder/issues/1839
	fs.StringVar(&o.metricsAddr, "metrics-bind-address", ":8080", "The TCP address that controller should bind to for serving Prometheus metrics. It can be set to \"0\" to disable the metrics serving.")
	fs.StringVar(&o.healthAddr, "health-probe-bind-address", ":8081", "The TCP address that controller should bind to for serving health probes.")
	fs.StringVar(&o.internalAPIAddr, "internal-api-address", ":8082", "The TCP address that controller should bind to for internal controller API.")

	fs.BoolVar(&o.leaderElection, "leader-elect", true, "Start a leader election client and gain leadership before executing the main loop. Enable this when running replicated components for high availability.")
	fs.StringVar(&o.leaderElectionResourceName, "leader-elect-resource-name", "rpaas-operator-lock", "The name of resource object that is used for locking during leader election.")
	fs.StringVar(&o.leaderElectionNamespace, "leader-elect-resource-namespace", "", "The namespace of resource object that is used for locking during leader election.")

	fs.DurationVar(&o.syncPeriod, "sync-period", 10*time.Hour, "The resync period for reconciling manager resources.")
	fs.StringVar(&o.namespace, "namespace", "", "Limit the observed RpaasInstance resources from specific namespace (empty means all namespaces)")

	fs.StringVar(&o.autoscalingController, "autoscaling-controller", "native", "Which controller should use to configure pod autoscaling features on RpaasInstances (available options: \"native\" or \"keda\")")
	fs.StringVar(&o.kedaPrometheusServerAddress, "keda-prometheus-server-address", "", "Address of Prometheus server used to configure KEDA ScaledObjects")
	fs.StringVar(&o.kedaPrometheusRPSQueryTemplate, "keda-prometheus-rps-query-template", `sum(rate(nginx_vts_server_requests_total{nginx_tsuru_io_resource_name="{{ .Name }}", namespace="{{ .Namespace }}"}[5m]))`, "Query template (go template) used to generate the RPS query on KEDA ScaledObjects' Prometheus trigger")
}

func main() {
	var opts configOpts
	opts.bindFlags(flag.CommandLine)

	zapOpts := zap.Options{}
	zapOpts.BindFlags(flag.CommandLine)
	flag.Parse()

	ctrl.SetLogger(zap.New(zap.UseFlagOptions(&zapOpts)))

	mgr, err := ctrl.NewManager(ctrl.GetConfigOrDie(), ctrl.Options{
		Scheme:                     extensionsruntime.NewScheme(),
		MetricsBindAddress:         opts.metricsAddr,
		LeaderElectionResourceLock: "leases",
		LeaderElection:             opts.leaderElection,
		LeaderElectionID:           opts.leaderElectionResourceName,
		LeaderElectionNamespace:    opts.leaderElectionNamespace,
		SyncPeriod:                 &opts.syncPeriod,
		HealthProbeBindAddress:     opts.healthAddr,
	})
	if err != nil {
		setupLog.Error(err, "unable to start manager")
		os.Exit(1)
	}

	if err = (&controllers.RpaasInstanceReconciler{
		Client:        mgr.GetClient(),
		Log:           mgr.GetLogger().WithName("controllers").WithName("RpaasInstance"),
		EventRecorder: mgr.GetEventRecorderFor("rpaas-operator"),
		KEDAOptions: controllers.KEDAOptions{
			Enabled:                 opts.autoscalingController == "keda",
			PrometheusServerAddress: opts.kedaPrometheusServerAddress,
			PrometheusRPSQuery:      template.Must(template.New("prometheus-query").Parse(opts.kedaPrometheusRPSQueryTemplate)),
		},
	}).SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "RpaasInstance")
		os.Exit(1)
	}
	// +kubebuilder:scaffold:builder

	// controllerapi
	go func() {
		setupLog.Info("starting internalapi", "addr", opts.internalAPIAddr)
		log.Fatal(http.ListenAndServe(opts.internalAPIAddr, controllerapi.New(mgr.GetClient())))
	}()

	setupLog.Info("starting manager")
	if err := mgr.Start(ctrl.SetupSignalHandler()); err != nil {
		setupLog.Error(err, "problem running manager")
		os.Exit(1)
	}
}
