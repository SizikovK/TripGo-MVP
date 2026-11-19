package middleware_test

import (
	"bytes"
	"context"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/metric/metricdata"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"

	"github.com/SizikovK/TripGo-MVP/internal/http/middleware"
)

func TestRequestSignals(t *testing.T) {
	for _, panics := range []bool{false, true} {
		ctx := context.Background()
		exporter := tracetest.NewInMemoryExporter()
		traces := sdktrace.NewTracerProvider(sdktrace.WithSyncer(exporter))
		t.Cleanup(func() { require.NoError(t, traces.Shutdown(ctx)) })
		reader := sdkmetric.NewManualReader()
		meters := sdkmetric.NewMeterProvider(sdkmetric.WithReader(reader))
		t.Cleanup(func() { require.NoError(t, meters.Shutdown(ctx)) })
		var logs bytes.Buffer
		logger := slog.New(slog.NewJSONHandler(&logs, nil))
		handler, err := middleware.Wrap(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			if panics {
				panic("secret panic")
			}
			w.WriteHeader(http.StatusCreated)
		}), logger, traces, meters, func(*http.Request) string { return "/trips/{id}" })
		require.NoError(t, err)
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, httptest.NewRequest("POST", "/trips/123", nil))
		status := http.StatusCreated
		if panics {
			status = http.StatusInternalServerError
			require.NotContains(t, response.Body.String(), "secret panic")
		}
		require.Equal(t, status, response.Code)
		spans := exporter.GetSpans()
		require.Len(t, spans, 1)
		require.Equal(t, "POST /trips/{id}", spans[0].Name)
		attrs := attribute.NewSet(spans[0].Attributes...)
		route, ok := attrs.Value("http.route")
		require.True(t, ok)
		require.Equal(t, "/trips/{id}", route.AsString())
		spanStatus, ok := attrs.Value("http.response.status_code")
		require.True(t, ok)
		require.EqualValues(t, status, spanStatus.AsInt64())
		if panics {
			require.Equal(t, codes.Error, spans[0].Status.Code)
		}

		require.Contains(t, logs.String(), `"route":"/trips/{id}"`)
		var result metricdata.ResourceMetrics
		require.NoError(t, reader.Collect(ctx, &result))
		found := false
		activeFound := false
		for _, scope := range result.ScopeMetrics {
			for _, signal := range scope.Metrics {
				if signal.Name == "http.server.request.duration" {
					found = true
					histogram, ok := signal.Data.(metricdata.Histogram[float64])
					require.True(t, ok)
					require.EqualValues(t, 1, histogram.DataPoints[0].Count)
					metricStatus, ok := histogram.DataPoints[0].Attributes.Value("http.response.status_code")
					require.True(t, ok)
					require.EqualValues(t, status, metricStatus.AsInt64())
				}
				if signal.Name == "http.server.active_requests" {
					activeFound = true
					active, ok := signal.Data.(metricdata.Sum[int64])
					require.True(t, ok)
					require.Len(t, active.DataPoints, 1)
					require.Zero(t, active.DataPoints[0].Value)
				}
			}
		}
		require.True(t, found)
		require.True(t, activeFound)
	}
}
