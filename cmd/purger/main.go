package main

import (
	"log"

	"github.com/google/gops/agent"
	"github.com/tsuru/rpaas-operator/internal/pkg/rpaas/nginx"
	"github.com/tsuru/rpaas-operator/internal/purge"
)

func main() {
	if err := agent.Listen(agent.Options{}); err != nil {
		log.Fatalf("could not initialize gops agent: %v", err)
	}
	defer agent.Close()

	k, err := purge.NewK8S()
	if err != nil {
		log.Fatalf("could not initialize kubernetes interface: %v", err)
	}

	w, err := purge.NewWatcher(k)
	if err != nil {
		log.Fatalf("could not create pods watcher: %v", err)
	}

	w.Watch()
	defer w.Stop()

	n := nginx.NewNginxManager()

	a, err := purge.NewAPI(w, n)
	if err != nil {
		log.Fatalf("could not create Purge API: %v", err)
	}

	if err := a.Start(); err != nil {
		log.Fatalf("could not start the Purge API server: %v", err)
	}
}
