package positions_test

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"sync/atomic"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel/codes"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
	"go.uber.org/goleak"
	"go.uber.org/mock/gomock"

	"github.com/SizikovK/TripGo-MVP/internal/config"
	"github.com/SizikovK/TripGo-MVP/internal/model"
	"github.com/SizikovK/TripGo-MVP/internal/worker/positions"
)

func TestMain(m *testing.M) { goleak.VerifyTestMain(m) }

type fixture struct {
	pool       *positions.Pool
	repository *Mockrepository
	push       *MockpushGateway
	metrics    *Mockmetrics
	ticks      chan time.Time
	results    chan string
	processed  chan string
	exporter   *tracetest.InMemoryExporter
	queue      atomic.Int64
	active     atomic.Int64
	ctx        context.Context
	cancel     context.CancelFunc
	stopped    chan struct{}
}

func newFixture(t *testing.T, workers, size int32) *fixture {
	t.Helper()
	ctrl := gomock.NewController(t)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	t.Cleanup(cancel)
	f := &fixture{
		repository: NewMockrepository(ctrl), push: NewMockpushGateway(ctrl), metrics: NewMockmetrics(ctrl),
		ticks: make(chan time.Time, 10), results: make(chan string, 100), processed: make(chan string, 100),
		exporter: tracetest.NewInMemoryExporter(), ctx: ctx, cancel: cancel, stopped: make(chan struct{}),
	}
	ticker := NewMockticker(ctrl)
	ticker.EXPECT().Ticks().Return((<-chan time.Time)(f.ticks)).AnyTimes()
	ticker.EXPECT().Stop().Do(func() { close(f.stopped) })
	f.metrics.EXPECT().Queued(gomock.Any(), gomock.Any()).Do(func(_ context.Context, result string) { f.results <- result }).AnyTimes()
	f.metrics.EXPECT().Processed(gomock.Any(), gomock.Any(), gomock.Any()).Do(func(_ context.Context, result string, _ time.Duration) { f.processed <- result }).AnyTimes()
	f.metrics.EXPECT().QueueChanged(gomock.Any()).Do(func(delta int64) { f.queue.Add(delta) }).AnyTimes()
	f.metrics.EXPECT().ActiveChanged(gomock.Any()).Do(func(delta int64) { f.active.Add(delta) }).AnyTimes()

	traces := sdktrace.NewTracerProvider(sdktrace.WithSyncer(f.exporter))
	t.Cleanup(func() { require.NoError(t, traces.Shutdown(context.Background())) })
	cfg := &config.Config{PositionWorkers: workers, PositionQueueSize: size, PositionStaleAfter: time.Minute, DatabaseQueryTimeout: time.Second, PushRequestTimeout: 5 * time.Second}
	f.pool = positions.New(cfg, f.repository, f.push, ticker, f.metrics, slog.New(slog.NewTextHandler(io.Discard, nil)), traces.Tracer("test"))
	f.pool.Start(ctx)
	t.Cleanup(func() {
		cleanup, stop := context.WithTimeout(context.Background(), time.Second)
		defer stop()
		require.NoError(t, f.pool.Shutdown(cleanup))
		require.Zero(t, f.queue.Load())
		require.Zero(t, f.active.Load())
	})

	return f
}

func take[T any](t *testing.T, ctx context.Context, ch <-chan T) T {
	t.Helper()
	select {
	case value := <-ch:
		return value
	case <-ctx.Done():
		t.Fatal(ctx.Err())
		var zero T
		return zero
	}
}

func request() model.PositionRequest {
	return model.PositionRequest{TripID: uuid.New(), DriverID: uuid.New()}
}

func (f *fixture) scan(requests ...model.PositionRequest) {
	f.repository.EXPECT().StaleTrips(gomock.Any(), time.Minute).Return(requests, nil)
	f.ticks <- time.Now()
}

func TestDuplicateAndCancellation(t *testing.T) {
	f := newFixture(t, 1, 2)
	first, second := request(), request()
	started, release := make(chan struct{}), make(chan struct{})
	f.push.EXPECT().RequestPosition(gomock.Any(), first).DoAndReturn(func(ctx context.Context, _ model.PositionRequest) error {
		close(started)
		select {
		case <-release:
			return nil
		case <-ctx.Done():
			return ctx.Err()
		}
	})

	f.scan(first)
	require.Equal(t, "queued", take(t, f.ctx, f.results))
	take(t, f.ctx, started)
	f.scan(first, second)
	require.Equal(t, "duplicate", take(t, f.ctx, f.results))
	require.Equal(t, "queued", take(t, f.ctx, f.results))
	f.pool.Cancel(second.TripID)
	close(release)
	require.Equal(t, "success", take(t, f.ctx, f.processed))
	require.Equal(t, "cancelled", take(t, f.ctx, f.processed))
}

func TestQueueOverflowDoesNotBlockScanner(t *testing.T) {
	f := newFixture(t, 1, 1)
	first, second, third := request(), request(), request()
	started, release := make(chan struct{}), make(chan struct{})
	f.push.EXPECT().RequestPosition(gomock.Any(), first).DoAndReturn(func(ctx context.Context, _ model.PositionRequest) error {
		close(started)
		select {
		case <-release:
			return nil
		case <-ctx.Done():
			return ctx.Err()
		}
	})
	f.push.EXPECT().RequestPosition(gomock.Any(), second).Return(nil)

	f.scan(first)
	require.Equal(t, "queued", take(t, f.ctx, f.results))
	take(t, f.ctx, started)
	f.scan(second, third)
	require.Equal(t, "queued", take(t, f.ctx, f.results))
	require.Equal(t, "dropped", take(t, f.ctx, f.results))
	f.scan(second)
	require.Equal(t, "duplicate", take(t, f.ctx, f.results))
	close(release)
	require.Equal(t, "success", take(t, f.ctx, f.processed))
	require.Equal(t, "success", take(t, f.ctx, f.processed))
}

func TestFixedNumberOfWorkers(t *testing.T) {
	const count = 12
	const workers = 3
	f := newFixture(t, workers, count)
	started, release := make(chan struct{}, count), make(chan struct{})
	var active, maximum atomic.Int32
	f.push.EXPECT().RequestPosition(gomock.Any(), gomock.Any()).DoAndReturn(func(ctx context.Context, _ model.PositionRequest) error {
		current := active.Add(1)
		for old := maximum.Load(); current > old; old = maximum.Load() {
			if maximum.CompareAndSwap(old, current) {
				break
			}
		}
		defer active.Add(-1)
		started <- struct{}{}
		select {
		case <-release:
			return nil
		case <-ctx.Done():
			return ctx.Err()
		}
	}).Times(count)

	requests := make([]model.PositionRequest, count)
	for i := range requests {
		requests[i] = request()
	}
	f.scan(requests...)
	for range workers {
		take(t, f.ctx, started)
	}
	require.EqualValues(t, workers, active.Load())
	close(release)
	for range count {
		require.Equal(t, "success", take(t, f.ctx, f.processed))
	}
	require.EqualValues(t, workers, maximum.Load())
}

func TestErrorDoesNotStopWorkerAndSpanIsLinked(t *testing.T) {
	f := newFixture(t, 1, 2)
	first, second := request(), request()
	f.push.EXPECT().RequestPosition(gomock.Any(), first).Return(errors.New("unavailable"))
	f.push.EXPECT().RequestPosition(gomock.Any(), second).Return(nil)
	f.scan(first, second)
	require.Equal(t, "error", take(t, f.ctx, f.processed))
	require.Equal(t, "success", take(t, f.ctx, f.processed))
	require.NoError(t, f.pool.Shutdown(f.ctx))

	spans := f.exporter.GetSpans()
	var linked int
	for _, span := range spans {
		if span.Name != "positions.request" {
			continue
		}

		require.False(t, span.Parent.IsValid())
		require.Len(t, span.Links, 1)
		for _, scan := range spans {
			if scan.Name == "positions.scan" {
				require.Equal(t, scan.SpanContext, span.Links[0].SpanContext)
				require.NotEqual(t, scan.SpanContext.TraceID(), span.SpanContext.TraceID())
			}
		}
		if span.Status.Code == codes.Error {
			linked++
		}
	}
	require.Equal(t, 1, linked)
}

func TestCancellationDoesNotInvalidateNewTask(t *testing.T) {
	f := newFixture(t, 1, 3)
	first, second := request(), request()
	started, release := make(chan struct{}), make(chan struct{})
	f.push.EXPECT().RequestPosition(gomock.Any(), first).DoAndReturn(func(ctx context.Context, _ model.PositionRequest) error {
		close(started)
		select {
		case <-release:
			return nil
		case <-ctx.Done():
			return ctx.Err()
		}
	})
	f.push.EXPECT().RequestPosition(gomock.Any(), second).Return(nil)
	f.scan(first)
	take(t, f.ctx, started)
	take(t, f.ctx, f.results)
	f.scan(second)
	take(t, f.ctx, f.results)
	f.pool.Cancel(second.TripID)
	f.scan(second)
	require.Equal(t, "queued", take(t, f.ctx, f.results))
	close(release)
	require.Equal(t, "success", take(t, f.ctx, f.processed))
	require.Equal(t, "cancelled", take(t, f.ctx, f.processed))
	require.Equal(t, "success", take(t, f.ctx, f.processed))
}

func TestShutdownDrainsQueue(t *testing.T) {
	f := newFixture(t, 1, 2)
	first, second := request(), request()
	started, release := make(chan struct{}), make(chan struct{})
	f.push.EXPECT().RequestPosition(gomock.Any(), first).DoAndReturn(func(ctx context.Context, _ model.PositionRequest) error {
		close(started)
		select {
		case <-release:
			return nil
		case <-ctx.Done():
			return ctx.Err()
		}
	})
	f.push.EXPECT().RequestPosition(gomock.Any(), second).Return(nil)
	f.scan(first)
	take(t, f.ctx, started)
	take(t, f.ctx, f.results)
	f.scan(second)
	take(t, f.ctx, f.results)

	finished := make(chan error, 1)
	go func() { finished <- f.pool.Shutdown(f.ctx) }()
	take(t, f.ctx, f.stopped)
	require.EqualValues(t, 1, f.active.Load())
	close(release)
	require.NoError(t, take(t, f.ctx, finished))
	require.Equal(t, "success", take(t, f.ctx, f.processed))
	require.Equal(t, "success", take(t, f.ctx, f.processed))
}

func TestContextCancellationStopsScanner(t *testing.T) {
	f := newFixture(t, 2, 2)
	f.cancel()
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	require.NoError(t, f.pool.Shutdown(ctx))
}

func TestShutdownDeadlineCancelsPush(t *testing.T) {
	f := newFixture(t, 1, 1)
	started := make(chan struct{})
	f.push.EXPECT().RequestPosition(gomock.Any(), gomock.Any()).DoAndReturn(func(ctx context.Context, _ model.PositionRequest) error {
		close(started)
		<-ctx.Done()
		return ctx.Err()
	})
	f.scan(request())
	take(t, f.ctx, started)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	require.ErrorIs(t, f.pool.Shutdown(ctx), context.Canceled)
	require.Equal(t, "error", take(t, f.ctx, f.processed))
}

func TestScannerContinuesAfterRepositoryError(t *testing.T) {
	f := newFixture(t, 1, 1)
	failed := make(chan struct{})
	f.repository.EXPECT().StaleTrips(gomock.Any(), time.Minute).DoAndReturn(func(context.Context, time.Duration) ([]model.PositionRequest, error) {
		close(failed)
		return nil, errors.New("database unavailable")
	})
	f.ticks <- time.Now()
	take(t, f.ctx, failed)

	next := request()
	f.push.EXPECT().RequestPosition(gomock.Any(), next).Return(nil)
	f.scan(next)
	require.Equal(t, "success", take(t, f.ctx, f.processed))
}

func TestPushHasDeadline(t *testing.T) {
	f := newFixture(t, 1, 1)
	deadlines := make(chan bool, 1)
	f.push.EXPECT().RequestPosition(gomock.Any(), gomock.Any()).DoAndReturn(func(ctx context.Context, _ model.PositionRequest) error {
		deadline, ok := ctx.Deadline()
		deadlines <- ok && time.Until(deadline) <= 5*time.Second
		return nil
	})
	f.scan(request())
	require.True(t, take(t, f.ctx, deadlines))
	require.Equal(t, "success", take(t, f.ctx, f.processed))
}
