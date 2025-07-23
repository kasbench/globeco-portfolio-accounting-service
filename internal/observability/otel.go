package observability

import (
	"context"
	"fmt"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetricgrpc"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/resource"
	"go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.21.0"
)

// OTelProvider holds shutdown functions for tracing and metrics
type OTelProvider struct {
	Shutdown func(ctx context.Context) error
}

// InitOTel sets up OpenTelemetry tracing and metrics with OTLP exporters
func InitOTel(ctx context.Context, serviceName, endpoint string, sampleRate float64) (*OTelProvider, error) {
	res, err := resource.New(ctx,
		resource.WithAttributes(
			semconv.ServiceName(serviceName),
			semconv.ServiceVersion("1.0.0"),
			semconv.ServiceNamespace("globeco"),
		),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create OTel resource: %w", err)
	}

	// Set up OTLP gRPC exporters for traces and metrics
	traceExp, err := otlptracegrpc.New(ctx,
		otlptracegrpc.WithEndpoint(endpoint),
		otlptracegrpc.WithInsecure(),
		otlptracegrpc.WithHeaders(map[string]string{
			"service.name": serviceName,
		}),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create OTLP trace exporter: %w", err)
	}

	metricExp, err := otlpmetricgrpc.New(ctx,
		otlpmetricgrpc.WithEndpoint(endpoint),
		otlpmetricgrpc.WithInsecure(),
		otlpmetricgrpc.WithHeaders(map[string]string{
			"service.name": serviceName,
		}),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create OTLP metric exporter: %w", err)
	}

	// Tracer provider with batcher and sampling
	tracerProvider := trace.NewTracerProvider(
		trace.WithBatcher(traceExp),
		trace.WithResource(res),
		trace.WithSampler(trace.ParentBased(trace.TraceIDRatioBased(sampleRate))),
	)
	otel.SetTracerProvider(tracerProvider)

	// Set up propagation for trace context (CRITICAL for receiving traces from other services)
	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(
		propagation.TraceContext{},
		propagation.Baggage{},
	))

	// Meter provider with periodic reader
	meterProvider := metric.NewMeterProvider(
		metric.WithResource(res),
		metric.WithReader(metric.NewPeriodicReader(
			metricExp,
			metric.WithInterval(10*time.Second),
		)),
	)
	otel.SetMeterProvider(meterProvider)

	shutdown := func(ctx context.Context) error {
		var err1, err2 error
		err1 = tracerProvider.Shutdown(ctx)
		err2 = meterProvider.Shutdown(ctx)
		if err1 != nil {
			return err1
		}
		return err2
	}

	return &OTelProvider{Shutdown: shutdown}, nil
}
