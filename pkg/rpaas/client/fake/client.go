// Copyright 2019 tsuru authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package fake

import (
	"context"
	"net/http"

	"github.com/tsuru/rpaas-operator/pkg/rpaas/client"
	"github.com/tsuru/rpaas-operator/pkg/rpaas/client/types"
)

type FakeClient struct {
	FakeGetPlans          func(instance string) ([]types.Plan, *http.Response, error)
	FakeGetFlavors        func(instance string) ([]types.Flavor, *http.Response, error)
	FakeScale             func(args client.ScaleArgs) (*http.Response, error)
	FakeUpdateCertificate func(args client.UpdateCertificateArgs) (*http.Response, error)
}

var _ client.Client = &FakeClient{}

func (f *FakeClient) GetPlans(ctx context.Context, instance string) ([]types.Plan, *http.Response, error) {
	if f.FakeGetPlans != nil {
		return f.FakeGetPlans(instance)
	}

	return nil, nil, nil
}

func (f *FakeClient) GetFlavors(ctx context.Context, instance string) ([]types.Flavor, *http.Response, error) {
	if f.FakeGetFlavors != nil {
		return f.FakeGetFlavors(instance)
	}

	return nil, nil, nil
}

func (f *FakeClient) Scale(ctx context.Context, args client.ScaleArgs) (*http.Response, error) {
	if f.FakeScale != nil {
		return f.FakeScale(args)
	}

	return nil, nil
}

func (f *FakeClient) UpdateCertificate(ctx context.Context, args client.UpdateCertificateArgs) (*http.Response, error) {
	if f.FakeUpdateCertificate != nil {
		return f.FakeUpdateCertificate(args)
	}

	return nil, nil
}
