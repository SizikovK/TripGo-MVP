package positions

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/google/uuid"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"

	"github.com/SizikovK/TripGo-MVP/internal/config"
	"github.com/SizikovK/TripGo-MVP/internal/model"
)

type task struct {
	request model.PositionRequest
	origin  trace.SpanContext
}

type Pool struct {
	cfg         *config.Config
	repository  repository
	push        pushGateway
	ticker      ticker
	metrics     metrics
	logger      *slog.Logger
	tracer      trace.Tracer
	queue       chan *task
	mu          sync.Mutex
	pending     map[uuid.UUID]*task
	workers     sync.WaitGroup
	stopScanner context.CancelFunc
	stopWorkers context.CancelFunc
	done        chan struct{}
}

func New(cfg *config.Config, repository repository, push pushGateway, ticker ticker, metrics metrics, logger *slog.Logger, tracer trace.Tracer) *Pool {
	return &Pool{
		cfg: cfg, repository: repository, push: push, ticker: ticker, metrics: metrics, logger: logger, tracer: tracer,
		queue: make(chan *task, int(cfg.PositionQueueSize)), pending: make(map[uuid.UUID]*task), done: make(chan struct{}),
	}
}

func (p *Pool) Start(ctx context.Context) {
	scannerCtx, stopScanner := context.WithCancel(ctx)
	workerCtx, stopWorkers := context.WithCancel(context.WithoutCancel(ctx))
	p.stopScanner, p.stopWorkers = stopScanner, stopWorkers
	p.logger.InfoContext(ctx, "position workers started", "count", p.cfg.PositionWorkers)

	for range int(p.cfg.PositionWorkers) {
		p.workers.Add(1)
		go func() {
			defer p.workers.Done()
			p.work(workerCtx)
		}()
	}

	go p.run(scannerCtx)
}

func (p *Pool) Shutdown(ctx context.Context) error {
	if p.stopScanner == nil {
		p.ticker.Stop()
		return nil
	}

	p.stopScanner()
	select {
	case <-p.done:
		return nil
	case <-ctx.Done():
		p.stopWorkers()
		return fmt.Errorf("stop position workers: %w", ctx.Err())
	}
}

func (p *Pool) Cancel(id uuid.UUID) {
	p.mu.Lock()
	delete(p.pending, id)
	p.mu.Unlock()
}

func (p *Pool) enqueue(ctx context.Context, request model.PositionRequest) string {
	p.mu.Lock()
	defer p.mu.Unlock()

	if _, exists := p.pending[request.TripID]; exists {
		return "duplicate"
	}

	item := &task{request: request, origin: trace.SpanContextFromContext(ctx)}
	p.pending[request.TripID] = item
	p.metrics.QueueChanged(1)
	select {
	case p.queue <- item:
		return "queued"
	default:
		delete(p.pending, request.TripID)
		p.metrics.QueueChanged(-1)
		return "dropped"
	}
}

func (p *Pool) work(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		case item, ok := <-p.queue:
			if !ok {
				return
			}

			p.metrics.QueueChanged(-1)
			p.process(ctx, item)
		}
	}
}

func (p *Pool) process(ctx context.Context, item *task) {
	p.mu.Lock()
	valid := p.pending[item.request.TripID] == item
	p.mu.Unlock()
	if !valid {
		p.metrics.Processed(ctx, "cancelled", 0)
		return
	}

	ctx, cancel := context.WithTimeout(ctx, p.cfg.PushRequestTimeout)
	defer cancel()
	opts := []trace.SpanStartOption{trace.WithNewRoot()}
	if item.origin.IsValid() {
		opts = append(opts, trace.WithLinks(trace.Link{SpanContext: item.origin}))
	}
	ctx, span := p.tracer.Start(ctx, "positions.request", opts...)
	defer span.End()

	p.metrics.ActiveChanged(1)
	started := time.Now()
	err := p.push.RequestPosition(ctx, item.request)
	elapsed := time.Since(started)
	p.metrics.ActiveChanged(-1)
	result := "success"
	if err != nil {
		result = "error"
		span.RecordError(err)
		span.SetStatus(codes.Error, "position request failed")
		span.SetAttributes(attribute.String("error.type", fmt.Sprintf("%T", err)))
		p.logger.WarnContext(ctx, "push service unavailable", "trip_id", item.request.TripID, "error", err)
	}
	p.metrics.Processed(ctx, result, elapsed)

	p.mu.Lock()
	if p.pending[item.request.TripID] == item {
		delete(p.pending, item.request.TripID)
	}
	p.mu.Unlock()
}
