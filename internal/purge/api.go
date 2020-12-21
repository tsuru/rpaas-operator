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
	"github.com/tsuru/rpaas-operator/internal/pkg/rpaas/nginx"
	"golang.org/x/net/http2"
)

var metricsMiddleware = echoPrometheus.MetricsMiddleware()

type purge struct {
	sync.Mutex

	watcher      *Watcher
	cacheManager nginx.NginxManager

	Address string

	ShutdownTimeout time.Duration

	started  bool
	e        *echo.Echo
	shutdown chan struct{}
}

func New(w *Watcher) (*purge, error) {
	p := &purge{
		watcher:         w,
		cacheManager:    nginx.NewNginxManager(),
		Address:         `:9990`,
		ShutdownTimeout: 30 * time.Second,
		e:               echo.New(),
		shutdown:        make(chan struct{}),
	}
	p.setupEcho(p.e)
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

func (p *purge) setupEcho(e *echo.Echo) {
	e.HideBanner = true
	e.HTTPErrorHandler = func(err error, c echo.Context) {
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

		e.Logger.Error(err)

		if !c.Response().Committed {
			if c.Request().Method == http.MethodHead {
				err = c.NoContent(code)
			} else {
				err = c.JSON(code, msg)
			}
			if err != nil {
				e.Logger.Error(err)
			}
		}
	}

	e.Use(middleware.Recover())
	e.Use(middleware.Logger())
	e.Use(metricsMiddleware)
	//e.Use(OpenTracingMiddleware)
	//e.Use(errorMiddleware)

	e.GET("/metrics", echo.WrapHandler(promhttp.Handler()))
	e.GET("/healthcheck", healthcheck)

	e.POST("/resource/:instance/purge", p.cachePurge)
	e.POST("/resource/:instance/purge/bulk", p.cachePurgeBulk)
}

func healthcheck(c echo.Context) error {
	return c.String(http.StatusOK, "OK")
}
