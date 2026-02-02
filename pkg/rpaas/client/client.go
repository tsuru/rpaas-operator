// Copyright 2019 tsuru authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package client

import (
	"context"
	"io"
	"time"

	"github.com/gorilla/websocket"

	"github.com/tsuru/rpaas-operator/pkg/rpaas/client/types"
)

type ScaleArgs struct {
	Instance string
	Replicas int32
}

type ExtraFilesArgs struct {
	Instance string
	Files    []types.RpaasFile
}

type DeleteExtraFilesArgs struct {
	Instance string
	Files    []string
}

type GetExtraFileArgs struct {
	Instance string
	FileName string
}

type ListExtraFilesArgs struct {
	Instance    string
	ShowContent bool
}
type UpdateCertificateArgs struct {
	Instance    string
	Name        string
	Certificate string
	Key         string

	boundary string
}

type DeleteCertificateArgs struct {
	Instance string
	Name     string
}

type UpdateBlockArgs struct {
	Instance   string
	Name       string
	ServerName string
	Content    string
	Extend     bool
}

type DeleteBlockArgs struct {
	Instance   string
	Name       string
	ServerName string
}

type ListBlocksArgs struct {
	Instance string
}

type DeleteRouteArgs struct {
	Instance   string
	ServerName string
	Path       string
}

type ListRoutesArgs struct {
	Instance string
}

type UpdateRouteArgs struct {
	Instance    string
	ServerName  string
	Path        string
	Destination string
	HTTPSOnly   bool
	Content     string
}

type InfoArgs struct {
	Instance string
	Raw      bool
}

type ExecArgs struct {
	In             io.Reader
	Command        []string
	Instance       string
	Pod            string
	Container      string
	TerminalWidth  uint16
	TerminalHeight uint16
	Interactive    bool
	TTY            bool
}

type DebugArgs struct {
	In             io.Reader
	Command        []string
	Instance       string
	Pod            string
	Container      string
	Image          string
	TerminalWidth  uint16
	TerminalHeight uint16
	TTY            bool
	Interactive    bool
}

type LogArgs struct {
	Out       io.Writer
	Instance  string
	Pod       string
	Container string
	Since     time.Duration
	Lines     int
	Follow    bool
	Color     bool
}

type UpdateCertManagerArgs struct {
	types.CertManager
	Instance string
}

type ListUpstreamOptionsArgs struct {
	Instance string
}

type UpstreamOptionsArgs struct {
	Instance             string
	PrimaryBind          string
	CanaryBinds          []string
	TrafficShapingPolicy TrafficShapingPolicy
	LoadBalance          string
	LoadBalanceHashKey   string
}

type TrafficShapingPolicy struct {
	Weight        int    `json:"weight,omitempty"`
	WeightTotal   int    `json:"weightTotal,omitempty"`
	Header        string `json:"header,omitempty"`
	HeaderValue   string `json:"headerValue,omitempty"`
	HeaderPattern string `json:"headerPattern,omitempty"`
	Cookie        string `json:"cookie,omitempty"`
}

type DeleteUpstreamOptionsArgs struct {
	Instance    string
	PrimaryBind string
}

type PurgeCacheArgs struct {
	Instance     string
	Path         string
	PreservePath bool
	ExtraHeaders map[string][]string
}

type PurgeCacheBulkArgs struct {
	Instance string
	Items    []PurgeCacheItem
}

type PurgeCacheItem struct {
	Path         string
	PreservePath bool
	ExtraHeaders map[string][]string
}

type PurgeBulkResult struct {
	Path            string `json:"path"`
	InstancesPurged int    `json:"instances_purged,omitempty"`
	Error           string `json:"error,omitempty"`
}

type Client interface {
	GetPlans(ctx context.Context, instance string) ([]types.Plan, error)
	GetFlavors(ctx context.Context, instance string) ([]types.Flavor, error)
	Scale(ctx context.Context, args ScaleArgs) error
	Restart(ctx context.Context, instance string) error
	Start(ctx context.Context, instance string) error
	Stop(ctx context.Context, instance string) error
	Info(ctx context.Context, args InfoArgs) (*types.InstanceInfo, error)
	UpdateCertificate(ctx context.Context, args UpdateCertificateArgs) error
	DeleteCertificate(ctx context.Context, args DeleteCertificateArgs) error
	UpdateBlock(ctx context.Context, args UpdateBlockArgs) error
	DeleteBlock(ctx context.Context, args DeleteBlockArgs) error
	ListBlocks(ctx context.Context, args ListBlocksArgs) ([]types.Block, error)
	DeleteRoute(ctx context.Context, args DeleteRouteArgs) error
	ListRoutes(ctx context.Context, args ListRoutesArgs) ([]types.Route, error)
	UpdateRoute(ctx context.Context, args UpdateRouteArgs) error
	Exec(ctx context.Context, args ExecArgs) (*websocket.Conn, error)
	Debug(ctx context.Context, args DebugArgs) (*websocket.Conn, error)
	Log(ctx context.Context, args LogArgs) error
	AddExtraFiles(ctx context.Context, args ExtraFilesArgs) error
	UpdateExtraFiles(ctx context.Context, args ExtraFilesArgs) error
	DeleteExtraFiles(ctx context.Context, args DeleteExtraFilesArgs) error
	ListExtraFiles(ctx context.Context, args ListExtraFilesArgs) ([]types.RpaasFile, error)
	GetExtraFile(ctx context.Context, args GetExtraFileArgs) (types.RpaasFile, error)

	AddAccessControlList(ctx context.Context, instance, host string, port int) error
	ListAccessControlList(ctx context.Context, instance string) ([]types.AllowedUpstream, error)
	RemoveAccessControlList(ctx context.Context, instance, host string, port int) error

	SetService(service string) (Client, error)

	ListCertManagerRequests(ctx context.Context, instance string) ([]types.CertManager, error)
	UpdateCertManager(ctx context.Context, args UpdateCertManagerArgs) error
	DeleteCertManagerByName(ctx context.Context, instance, name string) error
	DeleteCertManagerByIssuer(ctx context.Context, instance, issuer string) error

	GetMetadata(ctx context.Context, instance string) (*types.Metadata, error)
	SetMetadata(ctx context.Context, instance string, metadata *types.Metadata) error
	UnsetMetadata(ctx context.Context, instance string, metadata *types.Metadata) error

	ListUpstreamOptions(ctx context.Context, args ListUpstreamOptionsArgs) ([]types.UpstreamOptions, error)
	AddUpstreamOptions(ctx context.Context, args UpstreamOptionsArgs) error
	UpdateUpstreamOptions(ctx context.Context, args UpstreamOptionsArgs) error
	DeleteUpstreamOptions(ctx context.Context, args DeleteUpstreamOptionsArgs) error

	PurgeCache(ctx context.Context, args PurgeCacheArgs) (int, error)
	PurgeCacheBulk(ctx context.Context, args PurgeCacheBulkArgs) ([]PurgeBulkResult, error)
}

type wsWriter struct {
	*websocket.Conn
}

func (w *wsWriter) Write(p []byte) (int, error) {
	return len(p), w.Conn.WriteMessage(websocket.BinaryMessage, p)
}
