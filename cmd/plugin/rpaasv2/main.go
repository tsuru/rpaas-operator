// Copyright 2019 tsuru authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"log"
	"os"

	"github.com/tsuru/rpaas-operator/cmd/plugin/rpaasv2/cmd"
)

func main() {
	app := cmd.NewApp()

	err := app.Run(os.Args)
	if err != nil {
		log.Fatal(err)
	}
}
