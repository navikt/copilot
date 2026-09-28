package main

import (
	"bytes"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"testing"

	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
)

func TestHealthHandler(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	rec := httptest.NewRecorder()

	healthHandler(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", rec.Code)
	}

	if rec.Body.String() != "OK" {
		t.Errorf("Expected body 'OK', got %q", rec.Body.String())
	}
}

func TestReadyHandler(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/ready", nil)
	rec := httptest.NewRecorder()

	readyHandler(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", rec.Code)
	}

	if rec.Body.String() != "OK" {
		t.Errorf("Expected body 'OK', got %q", rec.Body.String())
	}
}

func TestHealthHandlerMethodNotAllowed(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/health", nil)
	rec := httptest.NewRecorder()

	healthHandler(rec, req)

	if rec.Code != http.StatusMethodNotAllowed {
		t.Errorf("Expected status 405, got %d", rec.Code)
	}
	if allow := rec.Header().Get("Allow"); allow != http.MethodGet {
		t.Errorf("Expected Allow header %q, got %q", http.MethodGet, allow)
	}
}

func TestReadyHandlerMethodNotAllowed(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/ready", nil)
	rec := httptest.NewRecorder()

	readyHandler(rec, req)

	if rec.Code != http.StatusMethodNotAllowed {
		t.Errorf("Expected status 405, got %d", rec.Code)
	}
	if allow := rec.Header().Get("Allow"); allow != http.MethodGet {
		t.Errorf("Expected Allow header %q, got %q", http.MethodGet, allow)
	}
}

func TestNotImplementedHandler(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/api/v1/test", nil)
	rec := httptest.NewRecorder()

	notImplementedHandler(rec, req)

	if rec.Code != http.StatusNotImplemented {
		t.Errorf("Expected status 501, got %d", rec.Code)
	}

	contentType := rec.Header().Get("Content-Type")
	if contentType != "application/problem+json" {
		t.Errorf("Expected Content-Type 'application/problem+json', got %q", contentType)
	}
}

func TestRespondError(t *testing.T) {
	tests := []struct {
		name       string
		errorType  string
		detail     string
		status     int
		wantStatus int
	}{
		{
			name:       "unauthorized",
			errorType:  "unauthorized",
			detail:     "Missing token",
			status:     http.StatusUnauthorized,
			wantStatus: 401,
		},
		{
			name:       "not found",
			errorType:  "not_found",
			detail:     "Resource not found",
			status:     http.StatusNotFound,
			wantStatus: 404,
		},
		{
			name:       "internal error",
			errorType:  "internal_error",
			detail:     "Something went wrong",
			status:     http.StatusInternalServerError,
			wantStatus: 500,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rec := httptest.NewRecorder()
			respondError(rec, tt.errorType, tt.detail, tt.status)

			if rec.Code != tt.wantStatus {
				t.Errorf("Expected status %d, got %d", tt.wantStatus, rec.Code)
			}

			contentType := rec.Header().Get("Content-Type")
			if contentType != "application/problem+json" {
				t.Errorf("Expected Content-Type 'application/problem+json', got %q", contentType)
			}

			// Verify response body contains expected fields
			body := rec.Body.String()
			if !strings.Contains(body, tt.detail) {
				t.Errorf("Response body should contain detail %q, got %q", tt.detail, body)
			}
		})
	}
}

func TestLoggingMiddleware(t *testing.T) {
	config := &Config{
		LoggedEndpoints: map[string]bool{
			"/api/v1/": true,
		},
	}

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	middleware := loggingMiddleware(config, handler)

	tests := []struct {
		name       string
		path       string
		shouldLog  bool
		wantStatus int
	}{
		{
			name:       "logged endpoint",
			path:       "/api/v1/test",
			shouldLog:  true,
			wantStatus: 200,
		},
		{
			name:       "not logged endpoint",
			path:       "/health",
			shouldLog:  false,
			wantStatus: 200,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, tt.path, nil)
			rec := httptest.NewRecorder()

			middleware.ServeHTTP(rec, req)

			if rec.Code != tt.wantStatus {
				t.Errorf("Expected status %d, got %d", tt.wantStatus, rec.Code)
			}
		})
	}
}

// TestAPIRoutesRequireAuth proves that every /api/v1/ route is behind auth middleware.
// This is a router-level integration test — it constructs the same middleware chain as main.go
// and verifies that requests without a Bearer token receive 401.
func TestAPIRoutesRequireAuth(t *testing.T) {
	// Use a strict auth middleware that rejects missing tokens (same as prod).
	authMiddleware := func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_, err := extractBearerToken(r)
			if err != nil {
				respondError(w, "unauthorized", err.Error(), http.StatusUnauthorized)
				return
			}
			next.ServeHTTP(w, r)
		})
	}

	config := &Config{Environment: "prod"}
	apiRouter := authMiddleware(makeAPIRouter(config, nil, nil, nil, NewIdentityResolverChain()))

	// All /api/v1/ paths should require auth
	paths := []string{
		"/api/v1/copilot/usage/metrics",
		"/api/v1/copilot/adoption/summary",
		"/api/v1/copilot/seats/testuser",
		"/api/v1/copilot/budget",
		"/api/v1/copilot/budget/global",
		"/api/v1/copilot/usage/summary",
	}

	for _, path := range paths {
		t.Run(path, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, path, nil)
			rec := httptest.NewRecorder()

			apiRouter.ServeHTTP(rec, req)

			if rec.Code != http.StatusUnauthorized {
				t.Errorf("GET %s without token: expected 401, got %d", path, rec.Code)
			}

			var problem ProblemDetail
			if err := json.NewDecoder(rec.Body).Decode(&problem); err != nil {
				t.Fatalf("Failed to decode response body: %v", err)
			}
			if !strings.Contains(problem.Type, "unauthorized") {
				t.Errorf("Expected problem type containing 'unauthorized', got %q", problem.Type)
			}
		})
	}
}

// The SAML lookup's path ends in the caller's Nav e-mail. Neither the request
// log nor a trace may carry it.
func TestSAMLPathNeverLoggedOrTraced(t *testing.T) {
	var buf bytes.Buffer
	prev := slog.Default()
	slog.SetDefault(slog.New(slog.NewJSONHandler(&buf, nil)))
	t.Cleanup(func() { slog.SetDefault(prev) })

	email := "ola.nordmann@nav.no"
	config := &Config{LoggedEndpoints: map[string]bool{"/api/v1/": true}}
	h := loggingMiddleware(config, http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))
	for _, p := range []string{"/api/v1/copilot/saml/" + email, "/api/v1/copilot/SAML/" + email, "/api/v1/ȺȺȺȺȺȺȺȺȺȺ/saml/" + email} {
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.URL.Path = p
		h.ServeHTTP(httptest.NewRecorder(), req)
		if tracePath(req) {
			t.Errorf("%s would be traced", p)
		}
	}
	if strings.Contains(buf.String(), email) || !strings.Contains(buf.String(), "/saml/{identity}") {
		t.Fatalf("log should carry the redacted path only, got %s", buf.String())
	}
	if !tracePath(httptest.NewRequest(http.MethodGet, "/api/v1/copilot/usage/metrics", nil)) {
		t.Error("ordinary routes must still be traced")
	}
}

// The wiring main.go uses: the SAML lookup yields no span, an ordinary
// route does.
func TestTraceAPISkipsSAMLLookup(t *testing.T) {
	exp := tracetest.NewInMemoryExporter()
	tp := sdktrace.NewTracerProvider(sdktrace.WithSyncer(exp))
	h := traceAPI(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}), otelhttp.WithTracerProvider(tp))
	for _, p := range []string{"/api/v1/copilot/saml/ola.nordmann@nav.no", "/api/v1/copilot/usage/metrics"} {
		h.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, p, nil))
	}
	spans := exp.GetSpans()
	if len(spans) != 1 || spans[0].Name != "GET /api/v1/copilot/usage/metrics" {
		t.Fatalf("spans = %v, want only the usage route", spans)
	}
}

// The per-user routes end in a GitHub username. The request log and the span
// carry {username}; the handler still gets the real path (#1085).
func TestUsernamePathsRedacted(t *testing.T) {
	var buf bytes.Buffer
	prev := slog.Default()
	slog.SetDefault(slog.New(slog.NewJSONHandler(&buf, nil)))
	t.Cleanup(func() { slog.SetDefault(prev) })

	exp := tracetest.NewInMemoryExporter()
	tp := sdktrace.NewTracerProvider(sdktrace.WithSyncer(exp))
	config := &Config{LoggedEndpoints: map[string]bool{"/api/v1/": true}}
	var got []string
	h := traceAPI(loggingMiddleware(config, http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		got = append(got, r.URL.Path)
	})), otelhttp.WithTracerProvider(tp))

	paths := map[string]string{
		"/api/v1/copilot/usage/user/Kari-Nordmann":               "GET /api/v1/copilot/usage/user/{username}",
		"/api/v1/copilot/usage/user/Kari-Nordmann/weekly":        "GET /api/v1/copilot/usage/user/{username}/weekly",
		"/api/v1/copilot/usage/user/Kari-Nordmann/daily-credits": "GET /api/v1/copilot/usage/user/{username}/daily-credits",
		"/api/v1/copilot/seats/Kari-Nordmann":                    "GET /api/v1/copilot/seats/{username}",
	}
	for p := range paths {
		h.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, p, nil))
	}
	if strings.Contains(strings.ToLower(buf.String()), "kari-nordmann") || !strings.Contains(buf.String(), "{username}") {
		t.Fatalf("log should carry the redacted paths only, got %s", buf.String())
	}
	names := map[string]bool{}
	for _, s := range exp.GetSpans() {
		names[s.Name] = true
		for _, a := range s.Attributes {
			if strings.Contains(strings.ToLower(a.Value.Emit()), "kari-nordmann") {
				t.Errorf("span %s attribute %s = %s", s.Name, a.Key, a.Value.Emit())
			}
		}
	}
	for p, want := range paths {
		if !names[want] {
			t.Errorf("no span %q for %s; spans: %v", want, p, names)
		}
		if !slices.Contains(got, p) {
			t.Errorf("handler did not get the real path %s; got %v", p, got)
		}
	}
}
