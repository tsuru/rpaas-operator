// Copyright 2019 tsuru authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package util

import (
	"crypto/sha256"
	"fmt"
	"hash"

	"github.com/davecgh/go-spew/spew"
)

func SHA256(obj interface{}) string {
	hash := sha256.New()
	deepHashObject(hash, obj)
	return fmt.Sprintf("%x", hash.Sum(nil))
}

func deepHashObject(h hash.Hash, obj interface{}) {
	h.Reset()
	printer := spew.ConfigState{
		Indent:         " ",
		SortKeys:       true,
		DisableMethods: true,
		SpewKeys:       true,
	}
	printer.Fprintf(h, "%#v", obj)
}
