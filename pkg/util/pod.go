// Copyright 2019 tsuru authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package util

import (
	corev1 "k8s.io/api/core/v1"
)

func PortByName(ports []corev1.ContainerPort, name string) int32 {
	for _, port := range ports {
		if port.Name == name {
			return port.ContainerPort
		}
	}
	return 0
}
