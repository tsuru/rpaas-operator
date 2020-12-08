// Copyright 2020 tsuru authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package registry

import (
	"crypto/tls"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/heroku/docker-registry-client/registry"
	"github.com/sirupsen/logrus"
)

const modulesLabel = "io.tsuru.nginx-modules"

type ImageMetadata interface {
	Modules(image string) ([]string, error)
}

type imageMetadataRetriever struct {
	mu            sync.Mutex
	labelsCache   map[string]map[string]string
	registryCache map[string]*registry.Registry
	insecure      bool
}

func NewImageMetadata() *imageMetadataRetriever {
	return &imageMetadataRetriever{
		labelsCache:   map[string]map[string]string{},
		registryCache: map[string]*registry.Registry{},
	}
}

func (r *imageMetadataRetriever) Modules(image string) ([]string, error) {
	labels, err := r.cachedLabels(image)
	if err != nil {
		return nil, err
	}
	if labels == nil {
		return nil, nil
	}
	rawModules := labels[modulesLabel]
	if rawModules == "" {
		return nil, nil
	}
	return strings.Split(labels[modulesLabel], ","), nil
}

func (r *imageMetadataRetriever) cachedLabels(image string) (map[string]string, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, ok := r.labelsCache[image]; ok {
		return r.labelsCache[image], nil
	}

	parts := parseImage(image)
	labels, err := r.imageLabels(parts)
	if err != nil {
		return nil, err
	}
	if !parts.isLatest() {
		r.labelsCache[image] = labels
	}

	return labels, nil
}

func (r *imageMetadataRetriever) imageLabels(image dockerImage) (map[string]string, error) {
	type historyEntry struct {
		Config struct {
			Labels map[string]string
		}
	}

	hub := r.registry(image.registry)
	manifest, err := hub.Manifest(image.image, image.tag)
	if err != nil {
		return nil, err
	}
	if len(manifest.History) == 0 {
		logrus.Errorf("No history found for image %s, caching empty labels", image)
		return nil, nil
	}
	data := manifest.History[0].V1Compatibility
	var entry historyEntry
	err = json.Unmarshal([]byte(data), &entry)
	if err != nil {
		logrus.Errorf("Unable to parse image labels for %s, caching empty labels", image)
		return nil, nil
	}
	return entry.Config.Labels, nil
}

func (r *imageMetadataRetriever) registry(registryHost string) *registry.Registry {
	if reg := r.registryCache[registryHost]; reg != nil {
		return reg
	}
	url := "https://" + registryHost
	transport := http.DefaultTransport
	if r.insecure {
		newTransport := http.DefaultTransport.(*http.Transport).Clone()
		newTransport.TLSClientConfig = &tls.Config{
			InsecureSkipVerify: true,
		}
		transport = newTransport
	}
	reg := &registry.Registry{
		URL: url,
		Client: &http.Client{
			Transport: registry.WrapTransport(transport, url, "", ""),
			Timeout:   30 * time.Second,
		},
		Logf: registry.Quiet,
	}
	r.registryCache[registryHost] = reg
	return reg
}

type dockerImage struct {
	registry string
	image    string
	tag      string
}

func (i dockerImage) isLatest() bool {
	return i.tag == "latest" || i.tag == "edge"
}

func (i dockerImage) String() string {
	return fmt.Sprintf("%s/%s:%s", i.registry, i.image, i.tag)
}

func parseImage(imageName string) dockerImage {
	img := dockerImage{
		registry: "registry-1.docker.io",
		tag:      "latest",
	}

	parts := strings.SplitN(imageName, "/", 3)
	switch len(parts) {
	case 1:
		img.image = imageName
	case 2:
		if strings.ContainsAny(parts[0], ":.") || parts[0] == "localhost" {
			img.registry = parts[0]
			img.image = parts[1]
			break
		}
		img.image = imageName
	case 3:
		img.registry = parts[0]
		img.image = strings.Join(parts[1:], "/")
	}
	parts = strings.SplitN(img.image, ":", 2)
	if len(parts) >= 2 {
		img.image = parts[0]
		img.tag = parts[1]
	}
	return img
}
