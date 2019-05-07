package main

import (
	"log"

	"github.com/google/gops/agent"
	"github.com/tsuru/rpaas-operator/api"
	"github.com/tsuru/rpaas-operator/pkg/apis"
)

func main() {
	if err := agent.Listen(agent.Options{}); err != nil {
		log.Fatalf("could not start gops: %+v\n", err)
	}
	defer agent.Close()

	manager, err := apis.NewManager()
	if err != nil {
		log.Fatalf("could not initialize kubernetes client manager: %+v\n", err)
	}

	if err = api.New(manager).Start(); err != nil {
		log.Fatalf("something went wrong: %+v", err)
	}
}
