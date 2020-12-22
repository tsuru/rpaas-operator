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
	"github.com/tsuru/rpaas-operator/internal/pkg/rpaas/nginx"
	"github.com/tsuru/rpaas-operator/pkg/observability"
)

var metricsMiddleware = echoPrometheus.MetricsMiddleware()

type purge struct {
	sync.Mutex

	lister       PodLister
	cacheManager nginx.NginxManager

	Address string

	ShutdownTimeout time.Duration

	started  bool
	e        *echo.Echo
	shutdown chan struct{}
}

type PodLister interface {
	ListPods(instance string) ([]rpaas.PodStatus, int32, error)
}

func NewAPI(l PodLister, n nginx.NginxManager) (*purge, error) {
	p := &purge{
		lister:          l,
		cacheManager:    n,
		Address:         `:9990`,
		ShutdownTimeout: 30 * time.Second,
		e:               echo.New(),
		shutdown:        make(chan struct{}),
	}
	p.setupEcho()
	return p, nil
}

func (p *purge) startServer() error {
	return p.e.StartH2CServer(p.Address, &http2.Server{})
}

// Start runs the web server.
func (p *purge) Start() error {
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
func (p *purge) Stop() error {
	p.Lock()
	defer p.Unlock()
	if !p.started {
		return fmt.Errorf("web server is already down")
	}
	if p.shutdown == nil {
		return fmt.Errorf("shutdown channel is not defined")
	}
	close(p.shutdown)
	ctx, cancel := context.WithTimeout(context.Background(), p.ShutdownTimeout)
	defer cancel()
	return p.e.Shutdown(ctx)
}

func (p *purge) handleSignals() {
	quit := make(chan os.Signal, 2)
	signal.Notify(quit, os.Interrupt, syscall.SIGTERM)
	select {
	case <-quit:
		p.Stop()
	case <-p.shutdown:
	}
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

func (p *purge) setupEcho() {
	p.e.HideBanner = true
	p.e.HTTPErrorHandler = func(err error, c echo.Context) {
		var (
			code = http.StatusInternalServerError
			msg  interface{}
		)

		if he, ok := err.(*echo.HTTPError); ok {
			code = he.Code
			msg = he.Message
			if he.Internal != nil {
				msg = fmt.Sprintf("%v, %v", err, he.Internal)
			}
		} else {
			msg = err.Error()
		}
		if _, ok := msg.(string); ok {
			msg = echo.Map{"message": msg}
		}

		p.e.Logger.Error(err)

		if !c.Response().Committed {
			if c.Request().Method == http.MethodHead {
				err = c.NoContent(code)
			} else {
				err = c.JSON(code, msg)
			}
			if err != nil {
				p.e.Logger.Error(err)
			}
		}
	}

	observability.Initialize()

	p.e.Use(middleware.Recover())
	p.e.Use(middleware.Logger())
	p.e.Use(metricsMiddleware)
	p.e.Use(observability.OpenTracingMiddleware)
	p.e.Use(errorMiddleware)

	p.e.GET("/metrics", echo.WrapHandler(promhttp.Handler()))
	p.e.GET("/healthcheck", healthcheck)

	p.e.POST("/resource/:instance/purge", p.cachePurge)
	p.e.POST("/resource/:instance/purge/bulk", p.cachePurgeBulk)
}

func healthcheck(c echo.Context) error {
	return c.String(http.StatusOK, "OK")
}
