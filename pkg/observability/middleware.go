// Copyright 2020 tsuru authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package observability

import (
	"log"
	"net/http"

	"github.com/labstack/echo/v4"
	"github.com/opentracing/opentracing-go"
	opentracingExt "github.com/opentracing/opentracing-go/ext"
	jaegerConfig "github.com/uber/jaeger-client-go/config"
	"github.com/uber/jaeger-client-go/zipkin"
)

func Initialize() {
	// We decided to use B3 Format, in the future plan to move to W3C context propagation
	// https://github.com/w3c/trace-context
	zipkinPropagator := zipkin.NewZipkinB3HTTPHeaderPropagator()

	// setup opentracing
	cfg, err := jaegerConfig.FromEnv()
	if err != nil {
		log.Fatal(err)
	}
	if cfg.ServiceName == "" {
		return
	}

	tracer, _, err := cfg.NewTracer(
		jaegerConfig.Injector(opentracing.HTTPHeaders, zipkinPropagator),
		jaegerConfig.Extractor(opentracing.HTTPHeaders, zipkinPropagator),
		jaegerConfig.Injector(opentracing.TextMap, zipkinPropagator),
		jaegerConfig.Extractor(opentracing.TextMap, zipkinPropagator),
	)
	if err == nil {
		opentracing.SetGlobalTracer(tracer)
	} else {
		log.Printf("Could not initialize jaeger tracer: %s", err.Error())
	}
}

func OpenTracingMiddleware(next echo.HandlerFunc) echo.HandlerFunc {
	return func(c echo.Context) error {
		tracer := opentracing.GlobalTracer()
		var span opentracing.Span
		r := c.Request()

		path := c.Path()
		opName := r.Method + " " + path

		tags := []opentracing.StartSpanOption{
			opentracingExt.SpanKindRPCServer,
			opentracing.Tag{Key: "component", Value: "echo"},
			opentracing.Tag{Key: "request_id", Value: r.Header.Get("X-Request-ID")},
			opentracing.Tag{Key: "http.method", Value: r.Method},
			opentracing.Tag{Key: "http.url", Value: r.URL.RequestURI()},
		}

		wireContext, err := tracer.Extract(
			opentracing.HTTPHeaders,
			opentracing.HTTPHeadersCarrier(r.Header))

		if err == nil {
			tags = append(tags, opentracing.ChildOf(wireContext))
		}

		span = tracer.StartSpan(opName, tags...)
		defer span.Finish()

		ctx := opentracing.ContextWithSpan(r.Context(), span)
		newR := r.WithContext(ctx)

		c.SetRequest(newR)
		err = next(c)

		if err != nil {
			c.Error(err)
		}

		statusCode := c.Response().Status
		span.SetTag("http.status_code", statusCode)
		if statusCode >= http.StatusInternalServerError {
			opentracingExt.Error.Set(span, true)
		}

		return err
	}
}
