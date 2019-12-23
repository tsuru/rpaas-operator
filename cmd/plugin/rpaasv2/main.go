// Copyright 2019 tsuru authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"github.com/tsuru/rpaas-operator/cmd/plugin/rpaasv2/cmd"
)

func main() {
	cmd.NewApp().RunAndExitOnError()
}
