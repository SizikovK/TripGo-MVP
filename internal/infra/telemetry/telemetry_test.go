package telemetry

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel/sdk/metric/metricdata"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"

	"github.com/SizikovK/TripGo-MVP/internal/config"
)

func TestHTTPExportPaths(t *testing.T) {
	paths := make(chan string, 2)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || (r.URL.Path != "/v1/traces" && r.URL.Path != "/v1/metrics") {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		paths <- r.URL.Path
		w.Header().Set("Content-Type", "application/x-protobuf")
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	traces, meters, err := exporters(ctx, &config.Config{OTelProtocol: "http/protobuf", OTelEndpoint: server.URL})
	require.NoError(t, err)
	t.Cleanup(func() {
		require.NoError(t, traces.Shutdown(context.Background()))
		require.NoError(t, meters.Shutdown(context.Background()))
	})
	span := tracetest.SpanStub{Name: "test"}.Snapshot()
	require.NoError(t, traces.ExportSpans(ctx, []sdktrace.ReadOnlySpan{span}))
	require.NoError(t, meters.Export(ctx, &metricdata.ResourceMetrics{}))
	require.Len(t, paths, 2)
	require.Equal(t, "/v1/traces", <-paths)
	require.Equal(t, "/v1/metrics", <-paths)
}

func TestTraceSampler(t *testing.T) {
	for _, name := range []string{"always_on", "always_off", "parentbased_always_on", "parentbased_always_off", "traceidratio", "parentbased_traceidratio"} {
		sampler, err := traceSampler(name, "0.5")
		require.NoError(t, err)
		require.NotNil(t, sampler)
	}
	for _, arg := range []string{"-1", "2", "bad", "NaN", "+Inf"} {
		_, err := traceSampler("traceidratio", arg)
		require.Error(t, err)
	}
	_, err := traceSampler("bad", "")
	require.Error(t, err)
}
