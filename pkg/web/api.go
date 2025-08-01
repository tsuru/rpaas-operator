// Copyright 2019 tsuru authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package web

import (
	"context"
	"fmt"
	"io"
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
	"github.com/tsuru/rpaas-operator/pkg/observability"
	"github.com/tsuru/rpaas-operator/pkg/web/target"
)

var metricsMiddleware = echoPrometheus.MetricsMiddleware()

type Api struct {
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

type APIServerStartOptions struct {
	DiscardLogging          bool
	ConfigEnableCertManager bool
}

// New creates an api instance.
func NewWithManager(manager rpaas.RpaasManager) (*Api, error) {
	localTargetFatory := target.NewLocalFactory(manager)
	return NewWithTargetFactoryWithDefaults(localTargetFatory)
}

func NewWithTargetFactoryWithDefaults(targetFactory target.Factory) (*Api, error) {
	return NewWithTargetFactory(targetFactory, `:9999`, `9993`, 30*time.Second, make(chan struct{}))
}

func NewWithTargetFactory(targetFactory target.Factory, address, addressTLS string, shutdownTimeout time.Duration, shutdownChan chan struct{}) (*Api, error) {
	return &Api{
		Address:         address,
		TLSAddress:      addressTLS,
		ShutdownTimeout: shutdownTimeout,
		e:               newEcho(targetFactory),
		shutdown:        shutdownChan,
	}, nil
}

func (a *Api) startServer() error {
	conf := config.Get()
	if conf.TLSCertificate != "" && conf.TLSKey != "" {
		return a.e.StartTLS(a.TLSAddress, conf.TLSCertificate, conf.TLSKey)
	}

	return a.e.StartH2CServer(a.Address, &http2.Server{})
}

// Start runs the web server.
func (a *Api) Start() error {
	a.Lock()
	a.started = true
	a.Unlock()
	go a.handleSignals()
	if err := a.startServer(); err != http.ErrServerClosed {
		a.e.Logger.Errorf("problem to start the webserver: %+v", err)
		return err
	}
	a.e.Logger.Info("Shutting down the webserver...")
	return nil
}

func (a *Api) StartWithOptions(options APIServerStartOptions) error {
	a.Lock()
	a.started = true
	a.Unlock()
	go a.handleSignals()
	if err := a.startServerWithOptions(options); err != http.ErrServerClosed {
		a.e.Logger.Errorf("problem to start the webserver: %+v", err)
		return err
	}
	a.e.Logger.Info("Shutting down the webserver...")
	return nil
}

func (a *Api) startServerWithOptions(options APIServerStartOptions) error {
	conf := config.Get()

	if options.DiscardLogging {
		a.e.Logger.SetOutput(io.Discard)
	}
	if options.ConfigEnableCertManager {
		conf.EnableCertManager = options.ConfigEnableCertManager
		config.Set(conf)
	}

	if conf.TLSCertificate != "" && conf.TLSKey != "" {
		return a.e.StartTLS(a.TLSAddress, conf.TLSCertificate, conf.TLSKey)
	}

	return a.e.StartH2CServer(a.Address, &http2.Server{})
}

// Stop shut down the web server.
func (a *Api) Stop() error {
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

func (a *Api) handleSignals() {
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
	e.HTTPErrorHandler = HTTPErrorHandler

	observability.Initialize()

	e.Use(middleware.Recover())
	e.Use(middleware.LoggerWithConfig(middleware.LoggerConfig{
		Skipper: func(c echo.Context) bool {
			if c.Path() == "/healthcheck" || c.Path() == "/metrics" {
				return true
			}
			return false
		},
		Format: `{"time":"${time_rfc3339_nano}","id":"${id}","remote_ip":"${remote_ip}",` +
			`"host":"${host}","method":"${method}","uri":"${uri}","user_agent":"${user_agent}",` +
			`"status":${status},"error":"${error}","latency":${latency},"latency_human":"${latency_human}"` +
			`,"bytes_in":${bytes_in},"bytes_out":${bytes_out}}` + "\n",
		CustomTimeFormat: "2006-01-02 15:04:05.00000",
	}))
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
	e.Use(ErrorMiddleware)

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
	group.POST("/:instance/autoscale", updateAutoscale)
	group.PUT("/:instance/autoscale", updateAutoscale)
	group.PATCH("/:instance/autoscale", updateAutoscale)
	group.DELETE("/:instance/autoscale", removeAutoscale)
	group.POST("/:instance/bind-app", serviceBindApp)
	group.DELETE("/:instance/bind-app", serviceUnbindApp)
	group.POST("/:instance/bind", serviceBindUnit)
	group.DELETE("/:instance/bind", serviceUnbindUnit)
	group.POST("/:instance/scale", scale)
	group.POST("/:instance/start", start)
	group.POST("/:instance/restart", restart)
	group.POST("/:instance/stop", stop)
	group.GET("/:instance/info", instanceInfo)
	group.POST("/:instance/certificate", updateCertificate)
	group.DELETE("/:instance/certificate/:name", deleteCertificate)
	group.DELETE("/:instance/certificate", deleteCertificate)
	group.GET("/:instance/cert-manager", listCertManagerRequests)
	group.POST("/:instance/cert-manager", updateCertManagerRequest)
	group.DELETE("/:instance/cert-manager", deleteCertManagerRequest)
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
	group.DELETE("/:instance/files", deleteExtraFiles)
	group.DELETE("/:instance/route", deleteRoute)
	group.GET("/:instance/route", getRoutes)
	group.POST("/:instance/route", updateRoute)
	group.POST("/:instance/purge", cachePurge)
	group.POST("/:instance/purge/bulk", cachePurgeBulk)
	group.Any("/:instance/exec", exec)
	group.Any("/:instance/debug", debug)
	group.GET("/:instance/acl", getUpstreams)
	group.POST("/:instance/acl", addUpstream)
	group.DELETE("/:instance/acl", deleteUpstream)
	group.GET("/:instance/log", log)
	group.GET("/:instance/metadata", getMetadata)
	group.POST("/:instance/metadata", setMetadata)
	group.DELETE("/:instance/metadata", unsetMetadata)

	return e
}
