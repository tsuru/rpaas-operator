// Copyright 2020 tsuru authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package registry

import (
	"encoding/json"
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
	mu          sync.Mutex
	labelsCache map[string]map[string]string
}

func NewImageMetadata() *imageMetadataRetriever {
	return &imageMetadataRetriever{
		labelsCache: map[string]map[string]string{},
	}
}

func (r *imageMetadataRetriever) Modules(image string) ([]string, error) {
	labels, err := r.getLabels(image)
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

func (r *imageMetadataRetriever) getLabels(image string) (map[string]string, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, ok := r.labelsCache[image]; ok {
		return r.labelsCache[image], nil
	}

	labels, err := getImageLabels(image)
	if err != nil {
		return nil, err
	}
	r.labelsCache[image] = labels

	return labels, nil
}

func getImageLabels(image string) (map[string]string, error) {
	type historyEntry struct {
		Config struct {
			Labels map[string]string
		}
	}

	parts := parseImage(image)
	hub := newRegistry(parts.registry)
	manifest, err := hub.Manifest(parts.image, parts.tag)
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

type dockerImage struct {
	registry string
	image    string
	tag      string
}

func parseImage(imageName string) dockerImage {
	img := dockerImage{
		registry: "registry-1.docker.io",
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

func newRegistry(registryHost string) *registry.Registry {
	url := "https://" + registryHost
	return &registry.Registry{
		URL: url,
		Client: &http.Client{
			Transport: registry.WrapTransport(http.DefaultTransport, url, "", ""),
			Timeout:   30 * time.Second,
		},
		Logf: registry.Quiet,
	}
}
