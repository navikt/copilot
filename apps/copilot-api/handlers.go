package main

import (
	"log/slog"
	"net/http"
	"strings"

	"go.opentelemetry.io/otel/trace"

	"github.com/navikt/copilot/apps/copilot-api/internal/auth"
	"github.com/navikt/copilot/apps/copilot-api/internal/config"
)

// healthHandler handles /health endpoint
func healthHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.Header().Set("Allow", http.MethodGet)
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	w.WriteHeader(http.StatusOK)
	if _, err := w.Write([]byte("OK")); err != nil {
		slog.Warn("Failed to write health response", "error", err)
	}
}

// readyHandler handles /ready endpoint
func readyHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.Header().Set("Allow", http.MethodGet)
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	if !auth.Ready() {
		w.WriteHeader(http.StatusServiceUnavailable)
		if _, err := w.Write([]byte("AUTH_UNAVAILABLE")); err != nil {
			slog.Warn("Failed to write ready response", "error", err)
		}
		return
	}
	w.WriteHeader(http.StatusOK)
	if _, err := w.Write([]byte("OK")); err != nil {
		slog.Warn("Failed to write ready response", "error", err)
	}
}

// tracePath reports whether a request may be traced. The SAML lookup never
// is: its route is internal to copilot-survey and has no use for a span.
func tracePath(r *http.Request) bool { return !strings.Contains(strings.ToLower(r.URL.Path), "/saml/") }

// loggingMiddleware logs HTTP requests
func loggingMiddleware(config *config.Config, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		shouldLog := false
		for endpoint := range config.LoggedEndpoints {
			if strings.HasPrefix(r.URL.Path, endpoint) {
				shouldLog = true
				break
			}
		}

		if shouldLog {
			args := []any{
				"method", r.Method,
				"path", auth.RedactPath(r.URL.Path),
				"remote_addr", r.RemoteAddr,
			}
			if sc := trace.SpanFromContext(r.Context()).SpanContext(); sc.IsValid() {
				args = append(args,
					"trace_id", sc.TraceID().String(),
					"span_id", sc.SpanID().String(),
				)
			}
			slog.Info("HTTP request", args...)
		}

		next.ServeHTTP(w, r)
	})
}
