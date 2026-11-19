package middleware

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"runtime/debug"
	"time"

	chimiddleware "github.com/go-chi/chi/v5/middleware"
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/metric/noop"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/trace"

	"github.com/SizikovK/TripGo-MVP/internal/http/response"
)

type requestKey struct{}

type requestInfo struct {
	writer chimiddleware.WrapResponseWriter
	ctx    context.Context
	status int
	route  string
}

func Wrap(next http.Handler, logger *slog.Logger, traces trace.TracerProvider, meters metric.MeterProvider, route func(*http.Request) string) (http.Handler, error) {
	meter := meters.Meter("tripgo.http")
	duration, err := meter.Float64Histogram("http.server.request.duration", metric.WithUnit("s"), metric.WithExplicitBucketBoundaries(.005, .01, .025, .05, .1, .25, .5, 1, 2.5, 5, 10))
	if err != nil {
		return nil, err
	}
	active, err := meter.Int64UpDownCounter("http.server.active_requests", metric.WithUnit("{request}"))
	if err != nil {
		return nil, err
	}
	next = metrics(next, duration, active)
	next = logging(next, logger)
	next = captureContext(next)
	next = otelhttp.NewHandler(next, "http", otelhttp.WithTracerProvider(traces), otelhttp.WithMeterProvider(noop.NewMeterProvider()),
		otelhttp.WithPropagators(propagation.NewCompositeTextMapPropagator(propagation.TraceContext{}, propagation.Baggage{})),
		otelhttp.WithSpanNameFormatter(func(_ string, r *http.Request) string { return r.Method + " " + route(r) }))
	return recoverPanics(next, logger, route), nil
}

func recoverPanics(next http.Handler, logger *slog.Logger, route func(*http.Request) string) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		writer := chimiddleware.NewWrapResponseWriter(w, r.ProtoMajor)
		info := &requestInfo{writer: writer, ctx: r.Context(), route: route(r)}
		r = r.WithContext(context.WithValue(r.Context(), requestKey{}, info))
		defer func() {
			if value := recover(); value != nil {
				response.Error(writer, r.WithContext(info.ctx), logger.With("stack", string(debug.Stack())), fmt.Errorf("panic: %v", value))
			}
		}()
		next.ServeHTTP(writer, r)
	})
}

func captureContext(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		info := r.Context().Value(requestKey{}).(*requestInfo)
		info.ctx = r.Context()
		next.ServeHTTP(w, r)
	})
}

func logging(next http.Handler, logger *slog.Logger) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		started := time.Now()
		info := r.Context().Value(requestKey{}).(*requestInfo)
		defer func() {
			level := slog.LevelInfo
			if info.status >= http.StatusInternalServerError {
				level = slog.LevelError
			}
			logger.Log(r.Context(), level, "request handled", "method", r.Method, "route", info.route, "status", info.status,
				"duration_ms", float64(time.Since(started).Microseconds())/1000)
		}()
		next.ServeHTTP(w, r)
	})
}

func metrics(next http.Handler, duration metric.Float64Histogram, active metric.Int64UpDownCounter) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		started := time.Now()
		info := r.Context().Value(requestKey{}).(*requestInfo)
		attrs := []attribute.KeyValue{attribute.String("http.request.method", r.Method), attribute.String("http.route", info.route)}
		active.Add(r.Context(), 1, metric.WithAttributes(attrs...))
		finished := false
		defer func() {
			info.status = info.writer.Status()
			if info.status == 0 {
				info.status = http.StatusOK
			}
			if !finished {
				info.status = http.StatusInternalServerError
			}
			span := trace.SpanFromContext(r.Context())
			span.SetAttributes(attribute.String("http.route", info.route), attribute.Int("http.response.status_code", info.status))
			if info.status >= http.StatusInternalServerError {
				span.SetStatus(codes.Error, http.StatusText(info.status))
				span.SetAttributes(attribute.String("error.type", http.StatusText(info.status)))
			}
			active.Add(r.Context(), -1, metric.WithAttributes(attrs...))
			attrs = append(attrs, attribute.Int("http.response.status_code", info.status))
			duration.Record(r.Context(), time.Since(started).Seconds(), metric.WithAttributes(attrs...))
		}()
		next.ServeHTTP(w, r)
		finished = true
	})
}
