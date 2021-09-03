// Copyright 2021 tsuru authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package certificates

import (
	"context"

	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/tsuru/rpaas-operator/api/v1alpha1"
)

func ReconcileDynamicCertificates(ctx context.Context, client client.Client, instance, instanceMergedWithFlavors *v1alpha1.RpaasInstance) error {
	// NOTE: for now, we've only a way to obtain automatic certificates but it can
	// be useful if we had more options in the future.
	return reconcileCertManager(ctx, client, instance, instanceMergedWithFlavors)
}
