// Copyright 2019 tsuru authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"log"

	"github.com/google/gops/agent"

	"github.com/tsuru/rpaas-operator/api"
	"github.com/tsuru/rpaas-operator/internal/config"
)

func main() {
	if err := config.Init(); err != nil {
		log.Fatalf("could not initialize RPaaS configurations: %v", err)
	}

	if err := agent.Listen(agent.Options{}); err != nil {
		log.Fatalf("could not initialize gops agent: %v", err)
	}
	defer agent.Close()

	a, err := api.New(nil)
	if err != nil {
		log.Fatalf("could not create RPaaS API: %v", err)
	}

	if err := a.Start(); err != nil {
		log.Fatalf("could not start the RPaaS API server: %v", err)
	}
}
