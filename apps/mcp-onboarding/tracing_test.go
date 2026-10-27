package main

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
)

// Probes yield no span; a traced route keeps its query string and
// Authorization header out of the span.
func TestTraceHTTP(t *testing.T) {
	exp := tracetest.NewInMemoryExporter()
	tp := sdktrace.NewTracerProvider(sdktrace.WithSyncer(exp))
	h := traceHTTP(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}), otelhttp.WithTracerProvider(tp))
	for _, p := range []string{"/health", "/ready", "/metrics", "/oauth/callback?code=fake-code&state=fake-state"} {
		r := httptest.NewRequest(http.MethodGet, p, nil)
		r.Header.Set("Authorization", "Bearer fake-token")
		h.ServeHTTP(httptest.NewRecorder(), r)
	}
	spans := exp.GetSpans()
	if len(spans) != 1 || spans[0].Name != "GET /oauth/callback" {
		t.Fatalf("spans = %v, want only GET /oauth/callback", spans)
	}
	for _, kv := range spans[0].Attributes {
		if v := kv.Value.String(); strings.Contains(v, "fake-") {
			t.Errorf("attribute %s leaks %q", kv.Key, v)
		}
	}
}
