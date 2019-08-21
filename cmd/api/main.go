package main

import (
	"log"

	"github.com/google/gops/agent"
	"github.com/tsuru/rpaas-operator/api"
	"github.com/tsuru/rpaas-operator/config"
	"github.com/tsuru/rpaas-operator/pkg/apis"
)

func startup() error {
	if err := agent.Listen(agent.Options{}); err != nil {
		return err
	}
	defer agent.Close()

	err := config.Init()
	if err != nil {
		return err
	}
	manager, err := apis.NewManager()
	if err != nil {
		return err
	}
	a, err := api.New(manager)
	if err != nil {
		return err
	}
	if err = a.Start(); err != nil {
		return err
	}
	return nil
}

func main() {
	err := startup()
	if err != nil {
		log.Fatalf("unable to initialize application: %+v", err)
	}
}
