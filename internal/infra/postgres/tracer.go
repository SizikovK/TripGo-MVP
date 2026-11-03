package postgres

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/exaring/otelpgx"
	"github.com/jackc/pgx/v5"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/metric/noop"
	"go.opentelemetry.io/otel/trace"
)

type queryKey struct{}

type queryInfo struct {
	started    time.Time
	operation  string
	collection string
}

type queryTracer struct {
	*otelpgx.Tracer
	duration metric.Float64Histogram
}

func newQueryTracer(traces trace.TracerProvider, meters metric.MeterProvider) (*queryTracer, error) {
	duration, err := meters.Meter("tripgo.postgres").Float64Histogram("db.client.operation.duration", metric.WithUnit("s"))
	if err != nil {
		return nil, err
	}
	base := otelpgx.NewTracer(otelpgx.WithTracerProvider(traces), otelpgx.WithMeterProvider(noop.NewMeterProvider()), otelpgx.WithDisableAcquireTracer())
	return &queryTracer{Tracer: base, duration: duration}, nil
}

func (t *queryTracer) TraceQueryStart(ctx context.Context, conn *pgx.Conn, data pgx.TraceQueryStartData) context.Context {
	ctx = t.Tracer.TraceQueryStart(ctx, conn, data)
	info := queryInfo{started: time.Now(), operation: otelpgx.SQLOperationName(ctx, data.SQL), collection: collectionName(data.SQL)}
	trace.SpanFromContext(ctx).SetAttributes(attribute.String("db.collection.name", info.collection))
	return context.WithValue(ctx, queryKey{}, info)
}

func (t *queryTracer) TraceQueryEnd(ctx context.Context, conn *pgx.Conn, data pgx.TraceQueryEndData) {
	info, ok := ctx.Value(queryKey{}).(queryInfo)
	if ok {
		errorType := ""
		if data.Err != nil {
			errorType = fmt.Sprintf("%T", data.Err)
			trace.SpanFromContext(ctx).SetAttributes(attribute.String("error.type", errorType))
		}
		t.duration.Record(ctx, time.Since(info.started).Seconds(), metric.WithAttributes(
			attribute.String("db.operation.name", info.operation), attribute.String("db.collection.name", info.collection), attribute.String("error.type", errorType),
		))
	}
	t.Tracer.TraceQueryEnd(ctx, conn, data)
}

func collectionName(query string) string {
	fields := strings.Fields(query)
	for i, field := range fields {
		if i+1 >= len(fields) {
			break
		}
		switch strings.ToUpper(field) {
		case "FROM", "INTO", "UPDATE":
			return strings.Trim(fields[i+1], "\"()")
		}
	}
	return ""
}
