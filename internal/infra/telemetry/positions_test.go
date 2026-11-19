package telemetry

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/metric/metricdata"
)

func TestPositionMetrics(t *testing.T) {
	ctx := context.Background()
	reader := sdkmetric.NewManualReader()
	provider := sdkmetric.NewMeterProvider(sdkmetric.WithReader(reader))
	t.Cleanup(func() { require.NoError(t, provider.Shutdown(ctx)) })
	metrics, err := NewPositions(provider.Meter("test"))
	require.NoError(t, err)

	metrics.Saved(ctx)
	metrics.Queued(ctx, "queued")
	metrics.Processed(ctx, "success", 500*time.Millisecond)
	metrics.Processed(ctx, "cancelled", 0)
	metrics.QueueChanged(2)
	metrics.ActiveChanged(1)

	for _, busy := range []bool{true, false} {
		if !busy {
			metrics.QueueChanged(-2)
			metrics.ActiveChanged(-1)
		}

		var result metricdata.ResourceMetrics
		require.NoError(t, reader.Collect(ctx, &result))
		found := make(map[string]bool)
		for _, scope := range result.ScopeMetrics {
			for _, signal := range scope.Metrics {
				found[signal.Name] = true
				switch data := signal.Data.(type) {
				case metricdata.Gauge[int64]:
					require.Len(t, data.DataPoints, 1)
					expected := int64(0)
					if busy {
						expected = 1
						if signal.Name == "tripgo.position_requests.queue.size" {
							expected = 2
						}
					}
					require.Equal(t, expected, data.DataPoints[0].Value)
				case metricdata.Histogram[float64]:
					require.Len(t, data.DataPoints, 1)
					require.EqualValues(t, 1, data.DataPoints[0].Count)
					require.InDelta(t, .5, data.DataPoints[0].Sum, .000001)
				}
			}
		}
		require.Len(t, found, 6)
	}
}
