package telemetry

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"testing"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
)

func TestLogIncludesTraceContext(t *testing.T) {
	exporter := tracetest.NewInMemoryExporter()
	provider := trace.NewTracerProvider(trace.WithSyncer(exporter))
	t.Cleanup(func() { require.NoError(t, provider.Shutdown(context.Background())) })
	ctx, span := provider.Tracer("test").Start(context.Background(), "request")
	defer span.End()
	var output bytes.Buffer
	logger := slog.New(logHandler{handler: slog.NewJSONHandler(&output, nil)}).With("service", "trip-service")
	logger.InfoContext(ctx, "test event")
	var record map[string]any
	require.NoError(t, json.Unmarshal(output.Bytes(), &record))
	require.Equal(t, span.SpanContext().TraceID().String(), record["trace_id"])
	require.Equal(t, span.SpanContext().SpanID().String(), record["span_id"])
	require.Equal(t, "trip-service", record["service"])
}
