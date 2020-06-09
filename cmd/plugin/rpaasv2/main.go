// Copyright 2019 tsuru authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"fmt"
	"os"

	"github.com/tsuru/rpaas-operator/cmd/plugin/rpaasv2/cmd"
)

func main() {
	app := cmd.NewDefaultApp()

	if err := app.Run([]string{"rpaasv2", "exec", "-i", "my-instance", "--tty"}); err != nil {
		fmt.Fprintln(app.ErrWriter, err)
		os.Exit(1)
	}
}
