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
	metricsAddr             string
	leaderElectionNamespace string
	syncPeriod              time.Duration
	portRangeMin            int
	portRangeMax            int
	enableRollout           bool
	enableLeaderElection    bool
}

func (o *configOpts) bindFlags(fs *flag.FlagSet) {
	fs.StringVar(&o.metricsAddr, "metrics-addr", ":8080", "The address the metric endpoint binds to.")
	fs.BoolVar(&o.enableRollout, "enable-rollout", true, "Enable automatic rollout of nginx objects on rpaas-instance change.")
	fs.BoolVar(&o.enableLeaderElection, "enable-leader-election", true,
		"Enable leader election for controller manager. "+
			"Enabling this will ensure there is only one active controller manager.")
	fs.StringVar(&o.leaderElectionNamespace, "leader-election-namespace", "",
		"Namespace where the leader election object will be created.")
	fs.DurationVar(&o.syncPeriod, "reconcile-sync", time.Minute, "Resync frequency of Nginx resources.")
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
		Scheme:                  extensionsruntime.NewScheme(),
		MetricsBindAddress:      opts.metricsAddr,
		Port:                    9443,
		LeaderElection:          opts.enableLeaderElection,
		LeaderElectionNamespace: opts.leaderElectionNamespace,
		LeaderElectionID:        "rpaas-operator-lock",
		SyncPeriod:              &opts.syncPeriod,
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
