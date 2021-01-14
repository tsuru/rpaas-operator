package purge

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

	"github.com/tsuru/rpaas-operator/internal/pkg/rpaas"
	"github.com/tsuru/rpaas-operator/pkg/observability"
	"github.com/tsuru/rpaas-operator/pkg/web"
)

var metricsMiddleware = echoPrometheus.MetricsMiddleware()

type PurgeAPI struct {
	sync.Mutex

	lister       PodLister
	cacheManager rpaas.CacheManager

	Address string

	ShutdownTimeout time.Duration

	started  bool
	e        *echo.Echo
	Shutdown chan struct{}
}

type PodLister interface {
	ListPods(instance string) ([]rpaas.PodStatus, int32, error)
}

func NewAPI(l PodLister, n rpaas.CacheManager) (*PurgeAPI, error) {
	p := &PurgeAPI{
		lister:          l,
		cacheManager:    n,
		Address:         `:9990`,
		ShutdownTimeout: 30 * time.Second,
		e:               echo.New(),
		Shutdown:        make(chan struct{}),
	}
	p.setupEcho()
	return p, nil
}

func (p *PurgeAPI) startServer() error {
	return p.e.StartH2CServer(p.Address, &http2.Server{})
}

// Start runs the web server.
func (p *PurgeAPI) Start() error {
	p.Lock()
	p.started = true
	p.Unlock()
	go p.handleSignals()
	if err := p.startServer(); err != http.ErrServerClosed {
		fmt.Printf("problem to start the webserver: %+v", err)
		return err
	}
	fmt.Println("Shutting down the webserver...")
	return nil
}

// Stop shut down the web server.
func (p *PurgeAPI) Stop() error {
	p.Lock()
	defer p.Unlock()
	if !p.started {
		return fmt.Errorf("web server is already down")
	}
	if p.Shutdown == nil {
		return fmt.Errorf("shutdown channel is not defined")
	}
	close(p.Shutdown)
	ctx, cancel := context.WithTimeout(context.Background(), p.ShutdownTimeout)
	defer cancel()
	return p.e.Shutdown(ctx)
}

func (p *PurgeAPI) handleSignals() {
	quit := make(chan os.Signal, 2)
	signal.Notify(quit, os.Interrupt, syscall.SIGTERM)
	select {
	case <-quit:
		p.Stop()
	case <-p.Shutdown:
	}
}

func (p *PurgeAPI) setupEcho() {
	p.e.HideBanner = true
	p.e.HTTPErrorHandler = web.HTTPErrorHandler
	observability.Initialize()

	p.e.Use(middleware.Recover())
	p.e.Use(middleware.Logger())
	p.e.Use(metricsMiddleware)
	p.e.Use(observability.OpenTracingMiddleware)
	p.e.Use(web.ErrorMiddleware)

	p.e.GET("/metrics", echo.WrapHandler(promhttp.Handler()))
	p.e.GET("/healthcheck", healthcheck)
	p.e.GET("/", mainRoute)

	p.e.POST("/resources/:instance/purge", p.cachePurge)
	p.e.POST("/resources/:instance/purge/bulk", p.cachePurgeBulk)
}

func healthcheck(c echo.Context) error {
	return c.String(http.StatusOK, "OK")
}

func mainRoute(c echo.Context) error {
	return c.String(http.StatusOK, "RPaaS Purger")
}
