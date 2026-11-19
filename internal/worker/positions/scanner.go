package positions

import (
	"context"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
)

func (p *Pool) run(ctx context.Context) {
	defer close(p.done)
	defer p.stopWorkers()
	p.logger.InfoContext(ctx, "position scanner started", "workers", p.cfg.PositionWorkers)

	for running := true; running; {
		select {
		case <-ctx.Done():
			running = false
		case <-p.ticker.Ticks():
			p.scan(ctx)
		}
	}

	p.ticker.Stop()
	p.logger.InfoContext(ctx, "position scanner stopped")
	close(p.queue)
	p.workers.Wait()
	p.logger.InfoContext(ctx, "position workers stopped")
}

func (p *Pool) scan(ctx context.Context) {
	ctx, cancel := context.WithTimeout(ctx, p.cfg.DatabaseQueryTimeout)
	defer cancel()
	ctx, span := p.tracer.Start(ctx, "positions.scan")
	defer span.End()

	requests, err := p.repository.StaleTrips(ctx, p.cfg.PositionStaleAfter)
	if err != nil {
		if ctx.Err() == context.Canceled {
			return
		}

		span.RecordError(err)
		span.SetStatus(codes.Error, "scan failed")
		span.SetAttributes(attribute.String("error.type", "repository"))
		p.logger.WarnContext(ctx, "cannot scan stale trips", "error", err)
		return
	}

	for _, request := range requests {
		if ctx.Err() != nil {
			return
		}

		result := p.enqueue(ctx, request)
		p.metrics.Queued(ctx, result)
		if result == "dropped" {
			p.logger.WarnContext(ctx, "position queue full", "trip_id", request.TripID)
		}
	}
}
