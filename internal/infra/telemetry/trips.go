package telemetry

import (
	"context"
	"log/slog"
	"time"

	"go.opentelemetry.io/otel/metric"
)

type Trips struct {
	created   metric.Int64Counter
	completed metric.Int64Counter
}

func NewTrips(meter metric.Meter, countActive func(context.Context) (int64, error), timeout time.Duration, logger *slog.Logger) (*Trips, error) {
	created, err := meter.Int64Counter("tripgo.trips.created", metric.WithUnit("{trip}"))
	if err != nil {
		return nil, err
	}
	completed, err := meter.Int64Counter("tripgo.trips.completed", metric.WithUnit("{trip}"))
	if err != nil {
		return nil, err
	}
	_, err = meter.Int64ObservableGauge("tripgo.trips.active", metric.WithUnit("{trip}"), metric.WithInt64Callback(func(ctx context.Context, observer metric.Int64Observer) error {
		queryCtx, cancel := context.WithTimeout(ctx, timeout)
		defer cancel()
		count, err := countActive(queryCtx)
		if err != nil {
			logger.WarnContext(ctx, "cannot collect active trips", "error", err)
			return nil
		}
		observer.Observe(count)
		return nil
	}))
	if err != nil {
		return nil, err
	}
	return &Trips{created: created, completed: completed}, nil
}

func (t *Trips) Created(ctx context.Context)   { t.created.Add(ctx, 1) }
func (t *Trips) Completed(ctx context.Context) { t.completed.Add(ctx, 1) }
