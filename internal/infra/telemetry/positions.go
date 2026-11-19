package telemetry

import (
	"context"
	"sync/atomic"
	"time"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
)

type Positions struct {
	saved    metric.Int64Counter
	checks   metric.Int64Counter
	requests metric.Int64Counter
	duration metric.Float64Histogram
	queue    atomic.Int64
	active   atomic.Int64
}

func NewPositions(meter metric.Meter) (*Positions, error) {
	p := &Positions{}
	var err error
	p.saved, err = meter.Int64Counter("tripgo.positions.saved")
	if err != nil {
		return nil, err
	}

	p.checks, err = meter.Int64Counter("tripgo.stale_checks")
	if err != nil {
		return nil, err
	}

	p.requests, err = meter.Int64Counter("tripgo.position_requests")
	if err != nil {
		return nil, err
	}

	p.duration, err = meter.Float64Histogram("tripgo.position_requests.duration", metric.WithUnit("s"),
		metric.WithExplicitBucketBoundaries(.005, .01, .025, .05, .1, .25, .5, 1, 2.5, 5))
	if err != nil {
		return nil, err
	}

	for _, gauge := range []struct {
		name  string
		value *atomic.Int64
	}{
		{name: "tripgo.position_requests.queue.size", value: &p.queue},
		{name: "tripgo.position_requests.workers.active", value: &p.active},
	} {
		_, err := meter.Int64ObservableGauge(gauge.name, metric.WithInt64Callback(func(_ context.Context, observer metric.Int64Observer) error {
			observer.Observe(gauge.value.Load())
			return nil
		}))
		if err != nil {
			return nil, err
		}
	}

	return p, nil
}

func (p *Positions) Saved(ctx context.Context) { p.saved.Add(ctx, 1) }
func (p *Positions) QueueChanged(delta int64)  { p.queue.Add(delta) }
func (p *Positions) ActiveChanged(delta int64) { p.active.Add(delta) }

func (p *Positions) Queued(ctx context.Context, result string) {
	p.checks.Add(ctx, 1, metric.WithAttributes(attribute.String("result", result)))
}

func (p *Positions) Processed(ctx context.Context, result string, duration time.Duration) {
	attrs := metric.WithAttributes(attribute.String("result", result))
	p.requests.Add(ctx, 1, attrs)
	if result != "cancelled" {
		p.duration.Record(ctx, duration.Seconds(), attrs)
	}
}
