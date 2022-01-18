// Copyright 2020 tsuru authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package registry

import (
	"context"
	"fmt"
	"strings"
	"sync"

	"github.com/google/go-containerregistry/pkg/authn"
	"github.com/google/go-containerregistry/pkg/name"
	"github.com/google/go-containerregistry/pkg/v1/google"
	"github.com/google/go-containerregistry/pkg/v1/remote"
)

const modulesLabel = "io.tsuru.nginx-modules"

var keychain = authn.NewMultiKeychain(
	authn.DefaultKeychain,
	google.Keychain,
	// maybe in the future, add more plugins here...
)

type ImageMetadata interface {
	Modules(ctx context.Context, image string) ([]string, error)
}

type imageMetadataRetriever struct {
	mu          sync.Mutex
	labelsCache map[string]map[string]string
}

func NewImageMetadata() *imageMetadataRetriever {
	return &imageMetadataRetriever{
		labelsCache: map[string]map[string]string{},
	}
}

func (r *imageMetadataRetriever) Modules(ctx context.Context, image string) ([]string, error) {
	labels, err := r.cachedLabels(ctx, image)
	if err != nil {
		return nil, err
	}

	return strings.Split(labels[modulesLabel], ","), nil
}

func (r *imageMetadataRetriever) cachedLabels(ctx context.Context, image string) (map[string]string, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if cachedLabels, ok := r.labelsCache[image]; ok {
		return cachedLabels, nil
	}

	ref, err := name.ParseReference(image)
	if err != nil {
		return nil, fmt.Errorf("failed to parse the image reference %q: %w", image, err)
	}

	labels, err := r.imageLabels(ctx, ref)
	if err != nil {
		return nil, err
	}

	if !isLatest(ref.Identifier()) {
		r.labelsCache[image] = labels
	}

	return labels, nil
}

func (r *imageMetadataRetriever) imageLabels(ctx context.Context, ref name.Reference) (map[string]string, error) {
	desc, err := remote.Get(ref, remote.WithContext(ctx), remote.WithAuthFromKeychain(keychain))
	if err != nil {
		return nil, fmt.Errorf("failed to fetch manifest of %q from container registry: %w", ref, err)
	}

	img, err := desc.Image()
	if err != nil {
		return nil, fmt.Errorf("failed to fetch config from manifest: %w", err)
	}

	cfg, err := img.ConfigFile()
	if err != nil {
		return nil, err
	}

	return cfg.Config.Labels, nil
}

func isLatest(tag string) bool {
	return tag == "latest" || tag == "edge"
}
