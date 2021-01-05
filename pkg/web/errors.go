package web

import (
	"fmt"
	"net/http"

	"github.com/labstack/echo/v4"
	"github.com/tsuru/rpaas-operator/internal/pkg/rpaas"
)

func ErrorMiddleware(next echo.HandlerFunc) echo.HandlerFunc {
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

func HTTPErrorHandler(err error, c echo.Context) {
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

	c.Logger().Error(err)

	if !c.Response().Committed {
		if c.Request().Method == http.MethodHead {
			err = c.NoContent(code)
		} else {
			err = c.JSON(code, msg)
		}
		if err != nil {
			c.Logger().Error(err)
		}
	}
}
