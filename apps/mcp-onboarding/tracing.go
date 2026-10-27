package main

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"os"

	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
	"go.opentelemetry.io/otel"
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

// traceHTTP wraps h in otelhttp. Probes and /metrics get no span. Spans are
// named after method and path; otelhttp records url.path but neither the
// query string nor headers, so OAuth codes and bearer tokens stay out.
func traceHTTP(h http.Handler, opts ...otelhttp.Option) http.Handler {
	return otelhttp.NewHandler(h, "http", append([]otelhttp.Option{
		otelhttp.WithFilter(func(r *http.Request) bool {
			switch r.URL.Path {
			case "/health", "/ready", "/metrics":
				return false
			}
			return true
		}),
		otelhttp.WithSpanNameFormatter(func(_ string, r *http.Request) string {
			return r.Method + " " + r.URL.Path
		}),
	}, opts...)...)
}
