// Copyright 2019 tsuru authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package apis

import (
	nginxApis "github.com/tsuru/nginx-operator/pkg/apis"
	"k8s.io/apimachinery/pkg/runtime"
)

// AddToSchemes may be used to add all resources defined in the project to a Scheme
var AddToSchemes runtime.SchemeBuilder

// AddToScheme adds all Resources to the Scheme
func AddToScheme(s *runtime.Scheme) error {
	return AddToSchemes.AddToScheme(s)
}

func NewScheme() (*runtime.Scheme, error) {
	s := runtime.NewScheme()
	if err := AddToScheme(s); err != nil {
		return nil, err
	}

	if err := nginxApis.AddToScheme(s); err != nil {
		return nil, err
	}

	return s, nil
}
