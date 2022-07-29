package web

import (
	"errors"
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

		internal := errors.Unwrap(err)

		if rpaas.IsNotModifiedError(err) {
			return c.NoContent(http.StatusNoContent)
		}

		if rpaas.IsValidationError(err) {
			return &echo.HTTPError{Code: http.StatusBadRequest, Message: err, Internal: internal}
		}

		if rpaas.IsConflictError(err) {
			return &echo.HTTPError{Code: http.StatusConflict, Message: err, Internal: internal}
		}

		if rpaas.IsNotFoundError(err) {
			return &echo.HTTPError{Code: http.StatusNotFound, Message: err, Internal: internal}
		}

		return err
	}
}

func HTTPErrorHandler(err error, c echo.Context) {
	var (
		code     int         = http.StatusInternalServerError
		msg      interface{} = err.Error()
		internal error       = errors.Unwrap(err)
	)

	var he *echo.HTTPError
	if errors.As(err, &he) {
		code, msg = he.Code, he.Message
	}

	if _, ok := msg.(string); ok {
		msg = echo.Map{"message": msg}
	}

	if c.Response().Committed {
		return
	}

	c.Logger().Error("Final error: %s; Wrapped error: %s", err, internal)

	if c.Request().Method == http.MethodHead {
		err = c.NoContent(code)
	} else {
		err = c.JSON(code, msg)
	}

	c.Logger().Errorf("failed to submit response: %s", err)
}
