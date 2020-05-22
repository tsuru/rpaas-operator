// Copyright 2019 tsuru authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package api

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"regexp"
	"sync"
	"syscall"
	"time"

	sigsk8sclient "sigs.k8s.io/controller-runtime/pkg/client"
	sigsk8sconfig "sigs.k8s.io/controller-runtime/pkg/client/config"

	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
	"github.com/tsuru/rpaas-operator/config"
	"github.com/tsuru/rpaas-operator/internal/pkg/rpaas"
	"github.com/tsuru/rpaas-operator/pkg/apis"
	"golang.org/x/net/http2"
	"golang.org/x/net/http2/h2c"
)

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
func New(manager rpaas.RpaasManager) (*api, error) {
	if manager == nil {
		k8sClient, err := newKubernetesClient()
		if err != nil {
			return nil, err
		}

		manager = rpaas.NewK8S(k8sClient)
	}

	return &api{
		Address:         `:9999`,
		TLSAddress:      `:9993`,
		ShutdownTimeout: 30 * time.Second,
		e:               newEcho(rm),
		mgr:             mgr,
		shutdown:        make(chan struct{}),
		rpaasManager:    rm,
	}
	// a.e.Use(a.rpaasManagerInjector())
	return a, nil
}

func (a *api) startServer() error {
	conf := config.Get()
	if conf.TLSCertificate != "" && conf.TLSKey != "" {
		return a.e.StartTLS(a.TLSAddress, conf.TLSCertificate, conf.TLSKey)
	}
	return a.e.Start(a.Address)
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

func setManager(c echo.Context, manager rpaas.RpaasManager) {
	c.Set("manager", manager)
}

func getManager(c echo.Context) (rpaas.RpaasManager, error) {
	manager, ok := c.Get("manager").(rpaas.RpaasManager)
	if !ok {
		return nil, fmt.Errorf("invalid manager state: %#v", c.Get("manager"))
	}
	return manager, nil
}

func errorMiddleware(next echo.HandlerFunc) echo.HandlerFunc {
	return func(c echo.Context) error {
		err := next(c)
		if err == nil {
			return nil
		}
		if rpaas.IsValidationError(err) {
			return &echo.HTTPError{Code: http.StatusBadRequest, Message: err}
		}
		if rpaas.IsConflictError(err) {
			return &echo.HTTPError{Code: http.StatusConflict, Message: err}
		}
		if rpaas.IsNotFoundError(err) {
			return &echo.HTTPError{Code: http.StatusNotFound, Message: err}
		}
		return err
	}
}

func getInstance(urlPath string) string {
	re := regexp.MustCompile(`.*\/resources\/(.*)\/`)
	s := re.FindStringSubmatch(urlPath)
	return s[1]
}

func newEcho(mgr rpaas.RpaasManager) *echo.Echo {
	e := echo.New()
	e.HideBanner = true

	e.Use(middleware.Recover())
	e.Use(middleware.Logger())
	e.Use(
		func(next echo.HandlerFunc) echo.HandlerFunc {
			return func(ctx echo.Context) error {
				setManager(ctx, mgr)
				return next(ctx)
			}
		})
	e.Use(middleware.BasicAuthWithConfig(middleware.BasicAuthConfig{
		Skipper: func(c echo.Context) bool {
			conf := config.Get()
			return c.Path() == "/healthcheck" ||
				(conf.APIUsername == "" && conf.APIPassword == "")
		},
		Validator: func(user, pass string, c echo.Context) (bool, error) {
			conf := config.Get()
			return user == conf.APIUsername &&
				pass == conf.APIPassword, nil
		},
		Realm: "Restricted",
	}))
	e.Use(errorMiddleware)
	e.Use(func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(ctx echo.Context) error {
			setManager(ctx, manager)
			return next(ctx)
		}
	})

	e.GET("/healthcheck", healthcheck)
	e.POST("/resources", serviceCreate)
	e.GET("/resources/flavors", getServiceFlavors)
	e.GET("/resources/:instance/flavors", getInstanceFlavors)
	e.GET("/resources/plans", servicePlans)
	e.GET("/resources/:instance/plans", servicePlans)
	e.GET("/resources/:instance", serviceInfo)
	e.PUT("/resources/:instance", serviceUpdate)
	e.GET("/resources/:instance/status", serviceStatus)
	e.GET("/resources/:instance/node_status", serviceNodeStatus)
	e.DELETE("/resources/:instance", serviceDelete)
	e.GET("/resources/:instance/autoscale", getAutoscale)
	e.POST("/resources/:instance/autoscale", createAutoscale)
	e.PATCH("/resources/:instance/autoscale", updateAutoscale)
	e.DELETE("/resources/:instance/autoscale", removeAutoscale)
	e.POST("/resources/:instance/bind-app", serviceBindApp)
	e.DELETE("/resources/:instance/bind-app", serviceUnbindApp)
	e.POST("/resources/:instance/bind", serviceBindUnit)
	e.DELETE("/resources/:instance/bind", serviceUnbindUnit)
	e.POST("/resources/:instance/scale", scale)
	e.GET("/resources/:instance/info", instanceInfo)
	e.POST("/resources/:instance/certificate", updateCertificate)
	e.DELETE("resources/:instance/certificate/:name", deleteCertificate)
	e.DELETE("resources/:instance/certificate", deleteCertificate)
	e.GET("/resources/:instance/certificate", getCertificates)
	e.GET("/resources/:instance/block", listBlocks)
	e.POST("/resources/:instance/block", updateBlock)
	e.DELETE("/resources/:instance/block/:block", deleteBlock)
	e.DELETE("/resources/:instance/lua", deleteLuaBlock)
	e.GET("/resources/:instance/lua", listLuaBlocks)
	e.POST("/resources/:instance/lua", updateLuaBlock)
	e.GET("/resources/:instance/files", listExtraFiles)
	e.GET("/resources/:instance/files/:name", getExtraFile)
	e.POST("/resources/:instance/files", addExtraFiles)
	e.PUT("/resources/:instance/files", updateExtraFiles)
	e.DELETE("/resources/:instance/files/:name", deleteExtraFile)
	e.DELETE("/resources/:instance/route", deleteRoute)
	e.GET("/resources/:instance/route", getRoutes)
	e.POST("/resources/:instance/route", updateRoute)
	e.POST("/resources/:instance/purge", cachePurge)

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var useTty bool
		if tty := r.FormValue("tty"); tty == "true" {
			useTty = true
		}
		if r.URL == nil {
			w.Write([]byte("missing URL"))
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		instanceName := getInstance(r.URL.Path)
		err := mgr.Exec(context.TODO(), instanceName, rpaas.ExecArgs{
			Stdin:          nil,
			Stdout:         w,
			Stderr:         w,
			Tty:            useTty,
			Command:        []string{"uptime"},
			TerminalWidth:  r.FormValue("w"),
			TerminalHeight: r.FormValue("r"),
		})
		if err != nil {
			fmt.Printf("error: %#v\n\n", err)
			w.WriteHeader(http.StatusInternalServerError)
			w.Write([]byte(err.Error()))
		}
	})
	h2s := &http2.Server{}
	e.Any("/resources/:instance/exec", echo.WrapHandler(h2c.NewHandler(handler, h2s)))

	return e
}

func newKubernetesClient() (sigsk8sclient.Client, error) {
	cfg, err := sigsk8sconfig.GetConfig()
	if err != nil {
		return nil, err
	}

	scheme, err := apis.NewScheme()
	if err != nil {
		return nil, err
	}

	c, err := sigsk8sclient.New(cfg, sigsk8sclient.Options{Scheme: scheme})
	if err != nil {
		return nil, err
	}

	return c, nil
}
