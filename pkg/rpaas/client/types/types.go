// Copyright 2019 tsuru authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package types

type Flavor struct {
	Name        string `json:"name"`
	Description string `json:"description"`
}

type Plan struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Default     bool   `json:"default"`
}
