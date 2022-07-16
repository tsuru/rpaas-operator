// Copyright 2020 tsuru authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"flag"
	"fmt"
	"os"
	"time"

	_ "k8s.io/client-go/plugin/pkg/client/auth/gcp"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"

	"github.com/tsuru/rpaas-operator/controllers"
	"github.com/tsuru/rpaas-operator/internal/registry"
	extensionsruntime "github.com/tsuru/rpaas-operator/pkg/runtime"
	// +kubebuilder:scaffold:imports
)

var setupLog = ctrl.Log.WithName("setup")

type configOpts struct {
	metricsAddr                string
	healthAddr                 string
	leaderElection             bool
	leaderElectionNamespace    string
	leaderElectionResourceName string
	namespace                  string
	syncPeriod                 time.Duration
	portRangeMin               int
	portRangeMax               int
	enableRollout              bool
}

func (o *configOpts) bindFlags(fs *flag.FlagSet) {
	// Following the standard of flags on Kubernetes.
	// See more: https://github.com/kubernetes-sigs/kubebuilder/issues/1839
	fs.StringVar(&o.metricsAddr, "metrics-bind-address", ":8080", "The TCP address that controller should bind to for serving Prometheus metrics. It can be set to \"0\" to disable the metrics serving.")
	fs.StringVar(&o.healthAddr, "health-probe-bind-address", ":8081", "The TCP address that controller should bind to for serving health probes.")

	fs.BoolVar(&o.leaderElection, "leader-elect", true, "Start a leader election client and gain leadership before executing the main loop. Enable this when running replicated components for high availability.")
	fs.StringVar(&o.leaderElectionResourceName, "leader-elect-resource-name", "rpaas-operator-lock", "The name of resource object that is used for locking during leader election.")
	fs.StringVar(&o.leaderElectionNamespace, "leader-elect-resource-namespace", "", "The namespace of resource object that is used for locking during leader election.")

	fs.DurationVar(&o.syncPeriod, "sync-period", 10*time.Hour, "The resync period for reconciling manager resources.")
	fs.StringVar(&o.namespace, "namespace", "", "Limit the observed RpaasInstance resources from specific namespace (empty means all namespaces)")

	fs.BoolVar(&o.enableRollout, "enable-rollout", true, "Enable automatic rollout of nginx objects on rpaas-instance change.")
	fs.IntVar(&o.portRangeMin, "port-range-min", 20000, "Allocated port range start")
	fs.IntVar(&o.portRangeMax, "port-range-max", 30000, "Allocated port range end")
}

func (o *configOpts) validate() error {
	if o.portRangeMin >= o.portRangeMax {
		return fmt.Errorf("invalid port range, min: %d, max: %d", o.portRangeMin, o.portRangeMax)
	}
	return nil
}

func main() {
	var opts configOpts
	opts.bindFlags(flag.CommandLine)

	zapOpts := zap.Options{}
	zapOpts.BindFlags(flag.CommandLine)
	flag.Parse()

	err := opts.validate()
	if err != nil {
		setupLog.Error(err, "invalid args")
		os.Exit(1)
	}

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
		Client:              mgr.GetClient(),
		Log:                 ctrl.Log.WithName("controllers").WithName("RpaasInstance"),
		Scheme:              mgr.GetScheme(),
		RolloutNginxEnabled: opts.enableRollout,
		PortRangeMin:        int32(opts.portRangeMin),
		PortRangeMax:        int32(opts.portRangeMax),
		ImageMetadata:       registry.NewImageMetadata(),
	}).SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "RpaasInstance")
		os.Exit(1)
	}
	// +kubebuilder:scaffold:builder

	setupLog.Info("starting manager")
	if err := mgr.Start(ctrl.SetupSignalHandler()); err != nil {
		setupLog.Error(err, "problem running manager")
		os.Exit(1)
	}
}
