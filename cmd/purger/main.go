package main

import (
	"context"
	"flag"
	"log"
	"time"

	"github.com/google/gops/agent"
	ctrl "sigs.k8s.io/controller-runtime"

	"github.com/tsuru/rpaas-operator/internal/pkg/rpaas/nginx"
	"github.com/tsuru/rpaas-operator/internal/purge"
	extensionsruntime "github.com/tsuru/rpaas-operator/pkg/runtime"
)

type configOpts struct {
	metricsAddr string
	syncPeriod  time.Duration
}

func (o *configOpts) bindFlags(fs *flag.FlagSet) {
	fs.StringVar(&o.metricsAddr, "metrics-addr", ":8080", "The address the metric endpoint binds to.")
	fs.DurationVar(&o.syncPeriod, "reconcile-sync", time.Minute, "Resync frequency of Nginx resources.")
}

func main() {
	var opts configOpts
	opts.bindFlags(flag.CommandLine)

	if err := agent.Listen(agent.Options{}); err != nil {
		log.Fatalf("could not initialize gops agent: %v", err)
	}
	defer agent.Close()

	mgr, err := ctrl.NewManager(ctrl.GetConfigOrDie(), ctrl.Options{
		Scheme:             extensionsruntime.NewScheme(),
		MetricsBindAddress: opts.metricsAddr,
		Port:               9443,
		SyncPeriod:         &opts.syncPeriod,
	})
	if err != nil {
		log.Fatalf("unable to start manager: %v", err)
	}

	w, err := purge.NewWatcher(mgr.GetClient())
	if err != nil {
		log.Fatalf("could not create pods watcher: %v", err)
	}

	n := nginx.NewNginxManager()

	a, err := purge.NewAPI(w, n)
	if err != nil {
		log.Fatalf("could not create Purge API: %v", err)
	}

	go func() {
		mgr.Start(context.Background())
	}()

	if err := a.Start(); err != nil {
		log.Fatalf("could not start the Purge API server: %v", err)
	}
}
