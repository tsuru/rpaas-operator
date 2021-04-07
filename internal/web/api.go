// Copyright 2019 tsuru authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package web

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	echoPrometheus "github.com/globocom/echo-prometheus"
	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"golang.org/x/net/http2"

	"github.com/tsuru/rpaas-operator/internal/config"
	"github.com/tsuru/rpaas-operator/internal/pkg/rpaas"
	"github.com/tsuru/rpaas-operator/internal/web/target"
	"github.com/tsuru/rpaas-operator/pkg/observability"
	"github.com/tsuru/rpaas-operator/pkg/web"
)

var metricsMiddleware = echoPrometheus.MetricsMiddleware()

type api struct {
	sync.Mutex

	// Address is the network address where the web server will listen on.
	// Defaults to `:9999`.
	Address    string
	TLSAddress string

	// ShutdownTimeout defines the max duration used to wait the web server
	// gracefully shutting down. Defaults to `30 * time.Second`.
	ShutdownTimeout time.Duration

	started  bool
	e        *echo.Echo
	shutdown chan struct{}
}

// New creates an api instance.
func NewWithManager(manager rpaas.RpaasManager) (*api, error) {
	localTargetFatory := target.NewLocalFactory(manager)
	return NewWithTargetFactory(localTargetFatory)
}

func NewWithTargetFactory(targetFactory target.Factory) (*api, error) {
	return &api{
		Address:         `:9999`,
		TLSAddress:      `:9993`,
		ShutdownTimeout: 30 * time.Second,
		e:               newEcho(targetFactory),
		shutdown:        make(chan struct{}),
	}, nil
}

func (a *api) startServer() error {
	conf := config.Get()
	if conf.TLSCertificate != "" && conf.TLSKey != "" {
		return a.e.StartTLS(a.TLSAddress, conf.TLSCertificate, conf.TLSKey)
	}

	return a.e.StartH2CServer(a.Address, &http2.Server{})
}

// Start runs the web server.
func (a *api) Start() error {
	a.Lock()
	a.started = true
	a.Unlock()
	go a.handleSignals()
	if err := a.startServer(); err != http.ErrServerClosed {
		fmt.Printf("problem to start the webserver: %+v", err)
		return err
	}
	fmt.Println("Shutting down the webserver...")
	return nil
}

// Stop shut down the web server.
func (a *api) Stop() error {
	a.Lock()
	defer a.Unlock()
	if !a.started {
		return fmt.Errorf("web server is already down")
	}
	if a.shutdown == nil {
		return fmt.Errorf("shutdown channel is not defined")
	}
	close(a.shutdown)
	ctx, cancel := context.WithTimeout(context.Background(), a.ShutdownTimeout)
	defer cancel()
	return a.e.Shutdown(ctx)
}

func (a *api) handleSignals() {
	quit := make(chan os.Signal, 2)
	signal.Notify(quit, os.Interrupt, syscall.SIGTERM)
	select {
	case <-quit:
		a.Stop()
	case <-a.shutdown:
	}
}

func getManager(ctx context.Context) (rpaas.RpaasManager, error) {
	manager := rpaas.RpaasManagerFromContext(ctx)

	if manager == nil {
		return nil, fmt.Errorf("No manager found on request")
	}
	return manager, nil
}

func newEcho(targetFactory target.Factory) *echo.Echo {
	e := echo.New()
	e.HideBanner = true
	e.HTTPErrorHandler = web.HTTPErrorHandler

	observability.Initialize()

	e.Use(middleware.Recover())
	e.Use(middleware.Logger())
	e.Use(metricsMiddleware)
	e.Use(observability.OpenTracingMiddleware)
	e.Use(middleware.BasicAuthWithConfig(middleware.BasicAuthConfig{
		Skipper: func(c echo.Context) bool {
			conf := config.Get()
			return c.Path() == "/healthcheck" ||
				c.Path() == "/metrics" ||
				(conf.APIUsername == "" && conf.APIPassword == "")
		},
		Validator: func(user, pass string, c echo.Context) (bool, error) {
			conf := config.Get()
			return user == conf.APIUsername &&
				pass == conf.APIPassword, nil
		},
		Realm: "Restricted",
	}))
	e.Use(web.ErrorMiddleware)

	e.GET("/metrics", echo.WrapHandler(promhttp.Handler()))
	e.GET("/healthcheck", healthcheck)

	group := e.Group("/resources", func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(echoCtx echo.Context) error {
			req := echoCtx.Request()
			manager, err := targetFactory.Manager(req.Context(), req.Header)
			if err != nil {
				return err
			}
			ctx := rpaas.ContextWithRpaasManager(req.Context(), manager)
			req = req.WithContext(ctx)
			echoCtx.SetRequest(req)
			return next(echoCtx)
		}
	})

	group.POST("", serviceCreate)
	group.GET("/flavors", getServiceFlavors)
	group.GET("/:instance/flavors", getInstanceFlavors)
	group.GET("/plans", servicePlans)
	group.GET("/:instance/plans", servicePlans)
	group.GET("/:instance", serviceInfo)
	group.PUT("/:instance", serviceUpdate)
	group.GET("/:instance/status", serviceStatus)
	group.GET("/:instance/node_status", serviceNodeStatus)
	group.DELETE("/:instance", serviceDelete)
	group.GET("/:instance/autoscale", getAutoscale)
	group.POST("/:instance/autoscale", createAutoscale)
	group.PATCH("/:instance/autoscale", updateAutoscale)
	group.DELETE("/:instance/autoscale", removeAutoscale)
	group.POST("/:instance/bind-app", serviceBindApp)
	group.DELETE("/:instance/bind-app", serviceUnbindApp)
	group.POST("/:instance/bind", serviceBindUnit)
	group.DELETE("/:instance/bind", serviceUnbindUnit)
	group.POST("/:instance/scale", scale)
	group.GET("/:instance/info", instanceInfo)
	group.POST("/:instance/certificate", updateCertificate)
	group.DELETE("/:instance/certificate/:name", deleteCertificate)
	group.DELETE("/:instance/certificate", deleteCertificate)
	group.GET("/:instance/certificate", getCertificates)
	group.GET("/:instance/block", listBlocks)
	group.POST("/:instance/block", updateBlock)
	group.DELETE("/:instance/block/:block", deleteBlock)
	group.DELETE("/:instance/lua", deleteLuaBlock)
	group.GET("/:instance/lua", listLuaBlocks)
	group.POST("/:instance/lua", updateLuaBlock)
	group.GET("/:instance/files", listExtraFiles)
	group.GET("/:instance/files/:name", getExtraFile)
	group.POST("/:instance/files", addExtraFiles)
	group.PUT("/:instance/files", updateExtraFiles)
	group.DELETE("/:instance/files/:name", deleteExtraFile)
	group.DELETE("/:instance/route", deleteRoute)
	group.GET("/:instance/route", getRoutes)
	group.POST("/:instance/route", updateRoute)
	group.POST("/:instance/purge", cachePurge)
	group.POST("/:instance/purge/bulk", cachePurgeBulk)
	group.Any("/:instance/exec", exec)
	group.GET("/:instance/upstream", getAllowedUpstreams)
	group.POST("/:instance/upstream", addAllowedUpstream)
	group.DELETE("/:instance/upstream", deleteAllowedUpstream)

	return e
}
