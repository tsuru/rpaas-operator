// Copyright 2019 tsuru authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package rpaas

import (
	"context"
	"crypto/sha256"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"sort"
	"strings"

	nginxv1alpha1 "github.com/tsuru/nginx-operator/api/v1alpha1"
	osb "sigs.k8s.io/go-open-service-broker-client/v2"

	"github.com/tsuru/rpaas-operator/api/v1alpha1"
	clientTypes "github.com/tsuru/rpaas-operator/pkg/rpaas/client/types"
)

type ConfigurationBlock struct {
	Name    string `form:"block_name" json:"block_name"`
	Content string `form:"content" json:"content"`
}

// ConfigurationBlockHandler defines some functions to handle the custom
// configuration blocks from an instance.
type ConfigurationBlockHandler interface {
	// DeleteBlock removes the configuration block named by blockName. It returns
	// a nil error meaning it was successful, otherwise a non-nil one which
	// describes the reached problem.
	DeleteBlock(ctx context.Context, instanceName, blockName string) error

	// ListBlocks returns all custom configuration blocks from instance (which
	// name is instanceName). It returns a nil error meaning it was successful,
	// otherwise a non-nil one which describes the reached problem.
	ListBlocks(ctx context.Context, instanceName string) ([]ConfigurationBlock, error)

	// UpdateBlock overwrites the older configuration block content with the one.
	// Whether the configuration block entry does not exist, it will already be
	// created with the new content. It returns a nil error meaning it was
	// successful, otherwise a non-nil one which describes the reached problem.
	UpdateBlock(ctx context.Context, instanceName string, block ConfigurationBlock) error
}

type File struct {
	Name    string
	Content []byte
}

func (f File) SHA256() string {
	return fmt.Sprintf("%x", sha256.Sum256(f.Content))
}

func (f File) MarshalJSON() ([]byte, error) {
	return json.Marshal(&map[string]interface{}{
		"name":    f.Name,
		"content": f.Content,
		"sha256":  f.SHA256(),
	})
}

type ExtraFileHandler interface {
	CreateExtraFiles(ctx context.Context, instanceName string, files ...File) error
	DeleteExtraFiles(ctx context.Context, instanceName string, filenames ...string) error
	GetExtraFiles(ctx context.Context, instanceName string) ([]File, error)
	UpdateExtraFiles(ctx context.Context, instanceName string, files ...File) error
}

type Route struct {
	Path        string `json:"path" form:"path"`
	Destination string `json:"destination" form:"destination"`
	Content     string `json:"content" form:"content"`
	HTTPSOnly   bool   `json:"https_only" form:"https_only"`
}

type RouteHandler interface {
	DeleteRoute(ctx context.Context, instanceName, path string) error
	GetRoutes(ctx context.Context, instanceName string) ([]Route, error)
	UpdateRoute(ctx context.Context, instanceName string, route Route) error
}

type CreateArgs struct {
	Name        string                 `form:"name"`
	Team        string                 `form:"team"`
	Plan        string                 `form:"plan"`
	Description string                 `form:"description"`
	Tags        []string               `form:"tags"`
	Parameters  map[string]interface{} `form:"parameters"`
}

func (args CreateArgs) Flavors() []string {
	return getFlavors(args.Parameters, args.Tags)
}

func (args CreateArgs) IP() string {
	return getIP(args.Parameters, args.Tags)
}

func (args CreateArgs) LoadBalancerName() string {
	return getLoadBalancerName(args.Parameters)
}

func (args CreateArgs) PlanOverride() string {
	return getPlanOverride(args.Parameters, args.Tags)
}

type UpdateInstanceArgs struct {
	Team        string                 `form:"team"`
	Description string                 `form:"description"`
	Plan        string                 `form:"plan"`
	Tags        []string               `form:"tags"`
	Parameters  map[string]interface{} `form:"parameters"`
}

func (args UpdateInstanceArgs) Flavors() []string {
	return getFlavors(args.Parameters, args.Tags)
}

func (args UpdateInstanceArgs) IP() string {
	return getIP(args.Parameters, args.Tags)
}

func (args UpdateInstanceArgs) LoadBalancerName() string {
	return getLoadBalancerName(args.Parameters)
}

func (args UpdateInstanceArgs) PlanOverride() string {
	return getPlanOverride(args.Parameters, args.Tags)
}

type PodStatusMap map[string]PodStatus

type PodStatus struct {
	Running bool   `json:"running"`
	Status  string `json:"status"`
	Address string `json:"address"`
}

type BindAppArgs struct {
	AppName          string   `form:"app-name"`
	AppHosts         []string `form:"app-hosts"`
	AppInternalHosts []string `form:"app-internal-hosts"`
	AppClusterName   string   `form:"app-cluster-name"`
	User             string   `form:"user"`
	EventID          string   `form:"eventid"`
}

type CacheManager interface {
	PurgeCache(host, path string, port int32, preservePath bool) error
}

type PurgeCacheArgs struct {
	Path         string `json:"path" form:"path"`
	PreservePath bool   `json:"preserve_path" form:"preserve_path"`
}

type PurgeCacheBulkResult struct {
	Path            string `json:"path"`
	InstancesPurged int    `json:"instances_purged,omitempty"`
	Error           string `json:"error,omitempty"`
}

type Plan struct {
	Name        string       `json:"name"`
	Description string       `json:"description"`
	Schemas     *osb.Schemas `json:"schemas,omitempty"`
}

type Flavor struct {
	Name        string `json:"name"`
	Description string `json:"description"`
}

type AutoscaleHandler interface {
	GetAutoscale(ctx context.Context, name string) (*clientTypes.Autoscale, error)
	CreateAutoscale(ctx context.Context, instanceName string, autoscale *clientTypes.Autoscale) error
	UpdateAutoscale(ctx context.Context, instanceName string, autoscale *clientTypes.Autoscale) error
	DeleteAutoscale(ctx context.Context, name string) error
}

type ExecArgs struct {
	Command        []string
	Pod            string
	Container      string
	TerminalWidth  uint16
	TerminalHeight uint16
	TTY            bool
	Interactive    bool

	Stdin  io.Reader
	Stdout io.Writer
	Stderr io.Writer
}

type RpaasManager interface {
	ConfigurationBlockHandler
	ExtraFileHandler
	RouteHandler
	AutoscaleHandler

	UpdateCertificate(ctx context.Context, instance, name string, cert tls.Certificate) error
	DeleteCertificate(ctx context.Context, instance, name string) error
	GetCertificates(ctx context.Context, instanceName string) ([]CertificateData, error)
	CreateInstance(ctx context.Context, args CreateArgs) error
	DeleteInstance(ctx context.Context, name string) error
	UpdateInstance(ctx context.Context, name string, args UpdateInstanceArgs) error
	GetInstance(ctx context.Context, name string) (*v1alpha1.RpaasInstance, error)
	GetInstanceAddress(ctx context.Context, name string) (string, error)
	GetInstanceStatus(ctx context.Context, name string) (*nginxv1alpha1.Nginx, PodStatusMap, error)
	Scale(ctx context.Context, name string, replicas int32) error
	GetPlans(ctx context.Context) ([]Plan, error)
	GetFlavors(ctx context.Context) ([]Flavor, error)
	BindApp(ctx context.Context, instanceName string, args BindAppArgs) error
	UnbindApp(ctx context.Context, instanceName, appName string) error
	PurgeCache(ctx context.Context, instanceName string, args PurgeCacheArgs) (int, error)
	GetInstanceInfo(ctx context.Context, instanceName string) (*clientTypes.InstanceInfo, error)
	Exec(ctx context.Context, instanceName string, args ExecArgs) error

	AddAllowedUpstream(ctx context.Context, instanceName string, upstream v1alpha1.RpaasAllowedUpstream) error
}

type CertificateData struct {
	Name        string `json:"name"`
	Certificate string `json:"certificate"`
	Key         string `json:"key"`
}

func getFlavors(params map[string]interface{}, tags []string) (flavors []string) {
	p, found := params["flavors"]
	if !found {
		return legacyGetFlavors(tags)
	}

	flavorsString, ok := p.(string)
	if ok {
		flavors = strings.Split(flavorsString, ",")
		return
	}

	flavorsParams, ok := p.(map[string]interface{})
	if !ok {
		return
	}

	var sortedKeys []string
	for key := range flavorsParams {
		sortedKeys = append(sortedKeys, key)
	}

	sort.Strings(sortedKeys)

	for _, key := range sortedKeys {
		flavors = append(flavors, flavorsParams[key].(string))
	}

	return
}

func legacyGetFlavors(tags []string) (flavors []string) {
	values := extractTagValues([]string{"flavor:", "flavor=", "flavors:", "flavors="}, tags)
	if len(values) == 0 {
		return nil
	}

	return strings.Split(values[0], ",")
}

func getIP(params map[string]interface{}, tags []string) string {
	p, found := params["ip"]
	if !found {
		return legacyGetIP(tags)
	}

	ip, ok := p.(string)
	if !ok {
		return ""
	}

	return ip
}

func legacyGetIP(tags []string) string {
	values := extractTagValues([]string{"ip:", "ip="}, tags)
	if len(values) == 0 {
		return ""
	}

	return values[0]
}

func getPlanOverride(params map[string]interface{}, tags []string) string {
	p, found := params["plan-override"]
	if !found {
		return legacyGetPlanOverride(tags)
	}

	override, ok := p.(string)
	if !ok {
		return ""
	}

	return override
}

func legacyGetPlanOverride(tags []string) string {
	values := extractTagValues([]string{"plan-override:", "plan-override="}, tags)
	if len(values) == 0 {
		return ""
	}

	return values[0]
}

func getLoadBalancerName(params map[string]interface{}) string {
	p, found := params["lb-name"]
	if !found {
		return ""
	}

	if lbName, ok := p.(string); ok {
		return lbName
	}
	return ""
}

func extractTagValues(prefixes, tags []string) []string {
	for _, t := range tags {
		for _, p := range prefixes {
			if !strings.HasPrefix(t, p) {
				continue
			}

			separator := string(p[len(p)-1])
			parts := strings.SplitN(t, separator, 2)
			if len(parts) == 1 {
				return nil
			}

			return parts[1:]
		}
	}

	return nil
}
