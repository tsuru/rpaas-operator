package api

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/google/gops/agent"
	"github.com/labstack/echo"
	"github.com/labstack/echo/middleware"
	"github.com/sirupsen/logrus"
	"github.com/tsuru/rpaas-operator/rpaas"
)

func handleSignals(e *echo.Echo) {
	quit := make(chan os.Signal)
	signal.Notify(quit, syscall.SIGTERM, syscall.SIGINT)
	<-quit
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	if err := e.Shutdown(ctx); err != nil {
		logrus.Fatal(err)
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

func rpaasManagerInjector(next echo.HandlerFunc) echo.HandlerFunc {
	return func(c echo.Context) error {
		manager := rpaas.NewK8S(rpaas.K8SOptions{
			Cli: cli,
			Ctx: c.Request().Context(),
		})
		setManager(c, manager)
		return next(c)
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

func Start() error {
	err := setup()
	if err != nil {
		logrus.Fatal(err)
		return err
	}

	if err = agent.Listen(agent.Options{}); err != nil {
		return err
	}
	defer agent.Close()

	e := configEcho()
	go handleSignals(e)

	err = e.Start(":9999")
	logrus.Infof("Shutting down server: %v", err)
	return err
}

func configEcho() *echo.Echo {
	e := echo.New()
	e.Use(middleware.Logger())
	e.Use(rpaasManagerInjector)
	e.Use(errorMiddleware)
	configHandlers(e)
	return e
}

func configHandlers(e *echo.Echo) {
	e.POST("/resources", serviceCreate)
	e.GET("/resources/plans", servicePlans)
	e.GET("/resources/:instance", serviceInfo)
	e.DELETE("/resources/:instance", serviceDelete)
	e.POST("/resources/:instance/bind-app", serviceBindApp)
	e.DELETE("/resources/:instance/bind-app", serviceUnbindApp)

	e.POST("/resources/:instance/scale", scale)
	e.POST("/resources/:instance/certificate", updateCertificate)
	e.POST("/resources/:instance/block", updateBlock)
	e.DELETE("/resources/:instance/block/:block", deleteBlock)
}
