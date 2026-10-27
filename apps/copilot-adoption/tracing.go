package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"sync"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.27.0"
)

// initTracer sets up the global OTel TracerProvider and W3C propagator.
// It reads OTEL_EXPORTER_OTLP_ENDPOINT from the environment (injected by Nais
// when observability.autoInstrumentation.runtime: sdk is configured).
// Returns a shutdown function that must be called on application exit.
func initTracer(ctx context.Context, serviceName string) (func(context.Context) error, error) {
	endpoint := os.Getenv("OTEL_EXPORTER_OTLP_ENDPOINT")
	if endpoint == "" {
		// No tracing configured — use no-op (e.g. local dev without Nais)
		return func(context.Context) error { return nil }, nil
	}

	exporter, err := otlptracegrpc.New(ctx)
	if err != nil {
		return nil, fmt.Errorf("create OTLP trace exporter: %w", err)
	}

	res, err := resource.New(ctx,
		resource.WithSchemaURL(semconv.SchemaURL),
		resource.WithAttributes(semconv.ServiceName(serviceName)),
		resource.WithFromEnv(), // picks up OTEL_SERVICE_NAME, OTEL_RESOURCE_ATTRIBUTES
	)
	if err != nil {
		return nil, fmt.Errorf("create OTel resource: %w", err)
	}

	tp := sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(exporter),
		sdktrace.WithResource(res),
		sdktrace.WithSampler(sdktrace.AlwaysSample()),
	)

	otel.SetTracerProvider(tp)
	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(
		propagation.TraceContext{},
		propagation.Baggage{},
	))

	slog.Info("OTel tracer initialized", "endpoint", endpoint, "service", serviceName)

	return tp.Shutdown, nil
}

// finishRun ends the root span of this run and flushes the tracer. It is a
// no-op until startRun has run, and runs at most once.
var finishRun = func(int) {}

// startRun starts the tracer and a root span covering the whole job run.
// main calls exit, not os.Exit, so the span is ended and flushed on every
// path: os.Exit skips deferred calls and the batch exporter would drop it.
func startRun(service string) {
	ctx := context.Background()
	shutdown, err := initTracer(ctx, service)
	if err != nil {
		slog.Warn("OTel tracer initialization failed — tracing disabled", "error", err)
		return
	}
	_, span := otel.Tracer(service).Start(ctx, service+".run")
	var once sync.Once
	finishRun = func(code int) {
		once.Do(func() {
			if code != 0 {
				span.SetStatus(codes.Error, fmt.Sprintf("exit code %d", code))
			}
			span.End()
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()
			if err := shutdown(ctx); err != nil {
				slog.Warn("OTel tracer shutdown error", "error", err)
			}
		})
	}
}

// exit ends and flushes the run span, then exits with code.
func exit(code int) {
	finishRun(code)
	os.Exit(code)
}
