package api

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/labstack/echo"
	"github.com/labstack/echo/middleware"
	"github.com/tsuru/rpaas-operator/config"
	"github.com/tsuru/rpaas-operator/rpaas"
	"sigs.k8s.io/controller-runtime/pkg/manager"
)

type api struct {
	sync.Mutex

	// Address is the network address where the web server will listen on.
	// Defaults to `:9999`.
	Address string

	// ShutdownTimeout defines the max duration used to wait the web server
	// gracefully shutting down. Defaults to `30 * time.Second`.
	ShutdownTimeout time.Duration

	started      bool
	e            *echo.Echo
	mgr          manager.Manager
	rpaasManager rpaas.RpaasManager
	shutdown     chan struct{}
}

// New creates an api instance.
func New(mgr manager.Manager) (*api, error) {
	var rm rpaas.RpaasManager
	if mgr != nil {
		var err error
		rm, err = rpaas.NewK8S(mgr)
		if err != nil {
			return nil, err
		}
	}
	a := &api{
		Address:         `:9999`,
		ShutdownTimeout: 30 * time.Second,
		e:               newEcho(),
		mgr:             mgr,
		shutdown:        make(chan struct{}),
		rpaasManager:    rm,
	}
	a.e.Use(a.rpaasManagerInjector())
	return a, nil
}

// Start runs the web server.
func (a *api) Start() error {
	a.Lock()
	a.started = true
	a.Unlock()
	go a.handleSignals()
	go a.mgr.Start(a.shutdown)
	if err := a.e.Start(a.Address); err != http.ErrServerClosed {
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

func (a *api) Handler() http.Handler {
	return a.e
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

func (a *api) rpaasManagerInjector() echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(ctx echo.Context) error {
			setManager(ctx, a.rpaasManager)
			return next(ctx)
		}
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

func newEcho() *echo.Echo {
	e := echo.New()

	e.HideBanner = true

	e.Use(middleware.Recover())
	e.Use(middleware.Logger())
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

	e.GET("/healthcheck", healthcheck)
	e.POST("/resources", serviceCreate)
	e.GET("/resources/flavors", getServiceFlavors)
	e.GET("/resources/:instance/flavors", getInstanceFlavors)
	e.GET("/resources/plans", servicePlans)
	e.GET("/resources/:instance/plans", servicePlans)
	e.GET("/resources/:instance", serviceInfo)
	e.GET("/resources/:instance/node_status", serviceStatus)
	e.DELETE("/resources/:instance", serviceDelete)
	e.POST("/resources/:instance/bind-app", serviceBindApp)
	e.DELETE("/resources/:instance/bind-app", serviceUnbindApp)
	e.POST("/resources/:instance/bind", serviceBindUnit)
	e.DELETE("/resources/:instance/bind", serviceUnbindUnit)
	e.POST("/resources/:instance/scale", scale)
	e.POST("/resources/:instance/certificate", updateCertificate)
	e.GET("/resources/:instance/block", listBlocks)
	e.POST("/resources/:instance/block", updateBlock)
	e.DELETE("/resources/:instance/block/:block", deleteBlock)
	e.GET("/resources/:instance/files", listExtraFiles)
	e.GET("/resources/:instance/files/:name", getExtraFile)
	e.POST("/resources/:instance/files", addExtraFiles)
	e.PUT("/resources/:instance/files", updateExtraFiles)
	e.DELETE("/resources/:instance/files/:name", deleteExtraFile)
	e.DELETE("/resources/:instance/route", deleteRoute)
	e.GET("/resources/:instance/route", getRoutes)
	e.POST("/resources/:instance/route", updateRoute)
	e.POST("/resources/:instance/purge", cachePurge)

	return e
}
