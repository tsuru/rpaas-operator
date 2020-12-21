package main

import (
	"log"

	"github.com/google/gops/agent"
	"github.com/tsuru/rpaas-operator/internal/purge"
)

func main() {
	if err := agent.Listen(agent.Options{}); err != nil {
		log.Fatalf("could not initialize gops agent: %v", err)
	}
	defer agent.Close()

	k, err := purge.NewK8S()
	if err != nil {
		log.Fatalf("could not create RPaaS API: %v", err)
	}

	w, err := purge.NewWithClient(k)
	if err != nil {
		log.Fatalf("could not create RPaaS API: %v", err)
	}

	w.Watch()

	a, err := purge.New(w)
	if err != nil {
		log.Fatalf("could not create RPaaS API: %v", err)
	}

	if err := a.Start(); err != nil {
		log.Fatalf("could not start the RPaaS API server: %v", err)
	}
}
