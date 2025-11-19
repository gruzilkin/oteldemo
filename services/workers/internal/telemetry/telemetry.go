package telemetry

import (
	"context"
	"log"
	logsdk "log/slog"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/exporters/otlp/otlplog/otlploggrpc"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	sdklog "go.opentelemetry.io/otel/sdk/log"
	semconv "go.opentelemetry.io/otel/semconv/v1.21.0"
	"go.opentelemetry.io/contrib/bridges/otelslog"

	"github.com/oteldemo/workers/internal/config"
)

// InitTracer initializes the OpenTelemetry tracer
func InitTracer(cfg *config.Config) (func(context.Context) error, error) {
	ctx := context.Background()

	// Create resource
	res, err := resource.New(ctx,
		resource.WithAttributes(
			semconv.ServiceName(cfg.ServiceName),
			semconv.ServiceVersion("1.0.0"),
			semconv.DeploymentEnvironment("demo"),
		),
		resource.WithAttributes(
			semconv.HostName(cfg.Location),
		),
	)
	if err != nil {
		return nil, err
	}

	// Create OTLP trace exporter
	traceExporter, err := otlptracegrpc.New(ctx,
		otlptracegrpc.WithEndpoint(cfg.OtelCollectorEndpoint),
		otlptracegrpc.WithInsecure(),
	)
	if err != nil {
		return nil, err
	}

	// Create batch span processor
	bsp := sdktrace.NewBatchSpanProcessor(traceExporter)

	// Create tracer provider
	tracerProvider := sdktrace.NewTracerProvider(
		sdktrace.WithSampler(sdktrace.AlwaysSample()),
		sdktrace.WithResource(res),
		sdktrace.WithSpanProcessor(bsp),
	)

	// Set global tracer provider
	otel.SetTracerProvider(tracerProvider)

	// Set global propagator to tracecontext (the default is no-op).
	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(
		propagation.TraceContext{},
		propagation.Baggage{},
	))

	log.Printf("OpenTelemetry tracer initialized for service: %s", cfg.ServiceName)

	// Return shutdown function
	return func(ctx context.Context) error {
		ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
		defer cancel()
		return tracerProvider.Shutdown(ctx)
	}, nil
}

// InitLogger initializes the OpenTelemetry logger
func InitLogger(cfg *config.Config) (func(context.Context) error, *logsdk.Logger, error) {
	ctx := context.Background()

	// Create resource (same as tracer)
	res, err := resource.New(ctx,
		resource.WithAttributes(
			semconv.ServiceName(cfg.ServiceName),
			semconv.ServiceVersion("1.0.0"),
			semconv.DeploymentEnvironment("demo"),
		),
		resource.WithAttributes(
			semconv.HostName(cfg.Location),
		),
	)
	if err != nil {
		return nil, nil, err
	}

	// Create OTLP log exporter
	logExporter, err := otlploggrpc.New(ctx,
		otlploggrpc.WithEndpoint(cfg.OtelCollectorEndpoint),
		otlploggrpc.WithInsecure(),
	)
	if err != nil {
		return nil, nil, err
	}

	// Create batch log processor
	logProcessor := sdklog.NewBatchProcessor(logExporter)

	// Create logger provider
	loggerProvider := sdklog.NewLoggerProvider(
		sdklog.WithResource(res),
		sdklog.WithProcessor(logProcessor),
	)

	// Create slog handler that bridges to OTEL
	otelHandler := otelslog.NewHandler(cfg.ServiceName, otelslog.WithLoggerProvider(loggerProvider))

	// Create slog logger with OTEL handler
	logger := logsdk.New(otelHandler)

	// Set as default slog logger
	logsdk.SetDefault(logger)

	log.Printf("OpenTelemetry logger initialized for service: %s", cfg.ServiceName)

	// Return shutdown function and logger
	return func(ctx context.Context) error {
		ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
		defer cancel()
		return loggerProvider.Shutdown(ctx)
	}, logger, nil
}
