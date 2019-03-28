package api

import (
	"context"
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

func Start() error {
	err := setup()
	if err != nil {
		logrus.Fatal(err)
		return err
	}

	rpaas.SetRpaasManager(rpaas.NewK8SRpaasManager(cli))

	if err = agent.Listen(agent.Options{}); err != nil {
		return err
	}
	defer agent.Close()

	e := echo.New()
	go handleSignals(e)
	e.Use(middleware.Logger())
	configHandlers(e)

	err = e.Start(":9999")
	logrus.Infof("Shutting down server: %v", err)
	return err
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
}
