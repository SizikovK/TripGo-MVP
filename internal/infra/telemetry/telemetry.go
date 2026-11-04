package telemetry

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"math"
	"strconv"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetricgrpc"
	"go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetrichttp"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"

	"github.com/SizikovK/TripGo-MVP/internal/config"
)

type Providers struct {
	Traces *sdktrace.TracerProvider
	Meters *sdkmetric.MeterProvider
}

func New(ctx context.Context, cfg *config.Config, logger *slog.Logger) (*Providers, error) {
	sampler, err := traceSampler(cfg.OTelSampler, cfg.OTelSamplerArg)
	if err != nil {
		return nil, err
	}
	res, err := resource.New(ctx,
		resource.WithAttributes(attribute.String("service.namespace", "tripgo"), attribute.String("deployment.environment", "local")),
		resource.WithFromEnv(),
		resource.WithAttributes(attribute.String("service.name", cfg.ServiceName), attribute.String("service.version", cfg.ServiceVersion)),
	)
	if err != nil {
		return nil, fmt.Errorf("telemetry resource: %w", err)
	}
	traceExporter, meterExporter, err := exporters(ctx, cfg)
	if err != nil {
		return nil, fmt.Errorf("create OTLP exporters: %w", err)
	}
	traces := sdktrace.NewTracerProvider(sdktrace.WithResource(res), sdktrace.WithSampler(sampler), sdktrace.WithBatcher(traceExporter, sdktrace.WithExportTimeout(cfg.DatabaseQueryTimeout)))
	meters := sdkmetric.NewMeterProvider(sdkmetric.WithResource(res), sdkmetric.WithReader(sdkmetric.NewPeriodicReader(meterExporter,
		sdkmetric.WithInterval(cfg.OTelMetricInterval), sdkmetric.WithTimeout(cfg.DatabaseQueryTimeout))))
	logger.InfoContext(ctx, "telemetry initialized", "protocol", cfg.OTelProtocol)
	return &Providers{Traces: traces, Meters: meters}, nil
}

func exporters(ctx context.Context, cfg *config.Config) (sdktrace.SpanExporter, sdkmetric.Exporter, error) {
	if cfg.OTelProtocol == "grpc" {
		t, err := otlptracegrpc.New(ctx, otlptracegrpc.WithEndpointURL(cfg.OTelEndpoint))
		if err != nil {
			return nil, nil, err
		}
		m, err := otlpmetricgrpc.New(ctx, otlpmetricgrpc.WithEndpointURL(cfg.OTelEndpoint))
		return t, m, err
	}
	t, err := otlptracehttp.New(ctx, otlptracehttp.WithEndpointURL(cfg.OTelEndpoint), otlptracehttp.WithURLPath("/v1/traces"))
	if err != nil {
		return nil, nil, err
	}
	m, err := otlpmetrichttp.New(ctx, otlpmetrichttp.WithEndpointURL(cfg.OTelEndpoint), otlpmetrichttp.WithURLPath("/v1/metrics"))
	return t, m, err
}

func traceSampler(name, arg string) (sdktrace.Sampler, error) {
	switch name {
	case "always_on":
		return sdktrace.AlwaysSample(), nil
	case "always_off":
		return sdktrace.NeverSample(), nil
	case "parentbased_always_on":
		return sdktrace.ParentBased(sdktrace.AlwaysSample()), nil
	case "parentbased_always_off":
		return sdktrace.ParentBased(sdktrace.NeverSample()), nil
	case "traceidratio", "parentbased_traceidratio":
		ratio, err := strconv.ParseFloat(arg, 64)
		if err != nil || math.IsNaN(ratio) || math.IsInf(ratio, 0) || ratio < 0 || ratio > 1 {
			return nil, errors.New("OTEL_TRACES_SAMPLER_ARG must be between 0 and 1")
		}
		sampler := sdktrace.TraceIDRatioBased(ratio)
		if name == "parentbased_traceidratio" {
			return sdktrace.ParentBased(sampler), nil
		}
		return sampler, nil
	default:
		return nil, fmt.Errorf("unsupported OTEL_TRACES_SAMPLER: %s", name)
	}
}

func (p *Providers) Shutdown(ctx context.Context) error {
	return errors.Join(p.Traces.Shutdown(ctx), p.Meters.Shutdown(ctx))
}
