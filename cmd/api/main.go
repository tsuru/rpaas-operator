// Copyright 2019 tsuru authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"log"

	"github.com/google/gops/agent"
	"github.com/spf13/viper"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"

	"github.com/tsuru/rpaas-operator/api/v1alpha1"
	"github.com/tsuru/rpaas-operator/internal/config"
	"github.com/tsuru/rpaas-operator/pkg/web"
	"github.com/tsuru/rpaas-operator/pkg/web/target"
)

func main() {
	if err := config.Init(); err != nil {
		log.Fatalf("could not initialize RPaaS configurations: %v", err)
	}

	if err := agent.Listen(agent.Options{}); err != nil {
		log.Fatalf("could not initialize gops agent: %v", err)
	}
	defer agent.Close()

	cfg := config.Get()
	isFakeServerAPI := viper.GetBool("fake-api")

	var targetFactory target.Factory
	var err error
	if cfg.MultiCluster {
		targetFactory = target.NewMultiClustersFactory(cfg.Clusters)
	} else if isFakeServerAPI {
		log.Println("Starting a Fake API Server (without K8s)...")
		targetFactory, err = target.NewFakeServerFactory(fakeRuntimeObjects())
		if err != nil {
			log.Fatalf("could not initialize fake cluster target: %v", err)
		}
	} else {
		targetFactory, err = target.NewKubeConfigFactory()
		if err != nil {
			log.Fatalf("could not initialize cluster target: %v", err)
		}
	}

	a, err := web.NewWithTargetFactoryWithDefaults(targetFactory)
	if err != nil {
		log.Fatalf("could not create RPaaS API: %v", err)
	}

	if err := a.Start(); err != nil {
		log.Fatalf("could not start the RPaaS API server: %v", err)
	}
}

func fakeRuntimeObjects() []runtime.Object {
	return []runtime.Object{
		&v1alpha1.RpaasPlan{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "my-plan",
				Namespace: "rpaasv2",
			},
		},
		&v1alpha1.RpaasInstance{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "my-rpaas",
				Namespace: "rpaasv2",
			},
		},
	}
}
