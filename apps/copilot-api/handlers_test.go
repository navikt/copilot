package main

import (
	"bytes"
	"context"
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

	"github.com/navikt/copilot/apps/copilot-api/internal/auth"
	"github.com/navikt/copilot/apps/copilot-api/internal/config"
	"github.com/navikt/copilot/apps/copilot-api/internal/problem"
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
	// Dev mode marks the auth middleware ready; this ran as a side effect of
	// the auth tests before they moved to internal/auth.
	auth.Middleware(auth.Config{Environment: "local"})
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
	config := &config.Config{
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
			_, err := auth.ExtractBearerToken(r)
			if err != nil {
				respondError(w, "unauthorized", err.Error(), http.StatusUnauthorized)
				return
			}
			next.ServeHTTP(w, r)
		})
	}

	config := &config.Config{Environment: "prod"}
	apiRouter := authMiddleware(makeAPIRouter(config, nil, nil, nil, nil, auth.NewIdentityResolverChain(), nil))

	// All /api/v1/ paths should require auth
	paths := []string{
		"/api/v1/copilot/usage/metrics",
		"/api/v1/copilot/usage/quality",
		"/api/v1/copilot/adoption/summary",
		"/api/v1/copilot/seats/testuser",
		"/api/v1/copilot/budget",
		"/api/v1/copilot/budget/global",
	}

	for _, path := range paths {
		t.Run(path, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, path, nil)
			rec := httptest.NewRecorder()

			apiRouter.ServeHTTP(rec, req)

			if rec.Code != http.StatusUnauthorized {
				t.Errorf("GET %s without token: expected 401, got %d", path, rec.Code)
			}

			var problem problem.Detail
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
	config := &config.Config{LoggedEndpoints: map[string]bool{"/api/v1/": true}}
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
	config := &config.Config{LoggedEndpoints: map[string]bool{"/api/v1/": true}}
	var got []string
	h := traceAPI(loggingMiddleware(config, http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		got = append(got, r.URL.Path)
	})), otelhttp.WithTracerProvider(tp))

	paths := map[string]string{
		"/api/v1/copilot/usage/user/Kari-Nordmann":               "GET /api/v1/copilot/usage/user/{username}",
		"/api/v1/copilot/usage/user/Kari-Nordmann/weekly":        "GET /api/v1/copilot/usage/user/{username}/weekly",
		"/api/v1/copilot/usage/user/Kari-Nordmann/daily-credits": "GET /api/v1/copilot/usage/user/{username}/daily-credits",
		"/api/v1/copilot/seats/Kari-Nordmann":                    "GET /api/v1/copilot/seats/{username}",
		"/api/v1/copilot/seats//Kari-Nordmann":                   "GET /api/v1/copilot/seats/{username}",
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
			if strings.Contains(strings.ToLower(a.Value.String()), "kari-nordmann") {
				t.Errorf("span %s attribute %s = %s", s.Name, a.Key, a.Value.String())
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

// Every allowlisted pattern must be one the API router registers, or the
// allowlist silently stops matching after a route is renamed.
func TestOnBehalfOfRoutesAreRegistered(t *testing.T) {
	mux := makeAPIRouter(&config.Config{}, nil, nil, nil, nil, auth.NewIdentityResolverChain(), nil).(*http.ServeMux)
	for _, pattern := range auth.OnBehalfOfRoutePatterns() {
		method, path, _ := strings.Cut(pattern, " ")
		req := httptest.NewRequest(method, strings.ReplaceAll(path, "{username}", "ola"), nil)
		if _, got := mux.Handler(req); got != pattern {
			t.Errorf("%q routes to %q", pattern, got)
		}
	}
}

// cliApp is copilot-cli's M2M token as copilot-api sees it.
var cliApp = &auth.User{AZP: "copilot-cli-client-id", Idtyp: "app"}

// TestRequireOwnershipIntegration exercises the full IdentityMiddleware +
// requireOwnership path used by handleUserMetrics/handleUserWeeklyTrends/
// handleUserDailyCredits (Phase 3 of the auth architecture migration — see
// identity.go, identity_middleware.go). Unit-level coverage of the
// individual resolvers and requireOwnership itself lives in identity_test.go;
// this test verifies they compose correctly through the middleware chain.
func TestRequireOwnershipIntegration(t *testing.T) {
	newRequest := func(user *auth.User, onBehalfOf string) *http.Request {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/copilot/usage/user/hans", nil)
		req.Pattern = "GET /api/v1/copilot/usage/user/{username}"
		if user != nil {
			req = req.WithContext(context.WithValue(req.Context(), auth.UserContextKey, user))
		}
		if onBehalfOf != "" {
			req.Header.Set("X-On-Behalf-Of", onBehalfOf)
		}
		return req
	}

	handler := func(w http.ResponseWriter, r *http.Request) {
		if auth.RequireOwnership(w, r, "hans") {
			w.WriteHeader(http.StatusOK)
		}
	}

	t.Run("copilot-cli azp with matching X-On-Behalf-Of succeeds", func(t *testing.T) {
		chain := auth.NewIdentityResolverChain(auth.NewOnBehalfOfIdentityResolver(map[string]bool{"copilot-cli-client-id": true}))
		mw := auth.IdentityMiddleware(chain, true)

		rec := httptest.NewRecorder()
		mw(http.HandlerFunc(handler)).ServeHTTP(rec, newRequest(cliApp, "hans"))

		if rec.Code != http.StatusOK {
			t.Fatalf("status: got %d, want %d, body: %s", rec.Code, http.StatusOK, rec.Body.String())
		}
	})

	t.Run("copilot-cli azp with mismatched X-On-Behalf-Of is denied", func(t *testing.T) {
		chain := auth.NewIdentityResolverChain(auth.NewOnBehalfOfIdentityResolver(map[string]bool{"copilot-cli-client-id": true}))
		mw := auth.IdentityMiddleware(chain, true)

		rec := httptest.NewRecorder()
		mw(http.HandlerFunc(handler)).ServeHTTP(rec, newRequest(cliApp, "someone-else"))

		if rec.Code != http.StatusForbidden {
			t.Errorf("status: got %d, want %d", rec.Code, http.StatusForbidden)
		}
	})

	t.Run("copilot-cli azp with missing X-On-Behalf-Of is rejected", func(t *testing.T) {
		chain := auth.NewIdentityResolverChain(auth.NewOnBehalfOfIdentityResolver(map[string]bool{"copilot-cli-client-id": true}))
		mw := auth.IdentityMiddleware(chain, true)

		rec := httptest.NewRecorder()
		mw(http.HandlerFunc(handler)).ServeHTTP(rec, newRequest(cliApp, ""))

		if rec.Code != http.StatusUnauthorized {
			t.Errorf("status: got %d, want %d", rec.Code, http.StatusUnauthorized)
		}
	})

	// A user token that carries X-On-Behalf-Of is refused, not resolved
	// through SAML with the header ignored. A user never asserts someone else.
	t.Run("user token with X-On-Behalf-Of is refused", func(t *testing.T) {
		chain := auth.NewIdentityResolverChain(
			auth.NewOnBehalfOfIdentityResolver(map[string]bool{"copilot-cli-client-id": true}),
			auth.NewSAMLIdentityResolver(&mockGitHubClient{samlUsername: "hans"}),
		)
		mw := auth.IdentityMiddleware(chain, true)

		req := newRequest(&auth.User{AZP: "my-copilot-client-id", Email: "hans@nav.no"}, "attacker")
		rec := httptest.NewRecorder()
		mw(http.HandlerFunc(handler)).ServeHTTP(rec, req)

		if rec.Code != http.StatusUnauthorized {
			t.Fatalf("status: got %d, want %d, body: %s", rec.Code, http.StatusUnauthorized, rec.Body.String())
		}
	})

	t.Run("non-copilot-cli azp falls back to SAML lookup", func(t *testing.T) {
		chain := auth.NewIdentityResolverChain(
			auth.NewOnBehalfOfIdentityResolver(map[string]bool{"copilot-cli-client-id": true}),
			auth.NewSAMLIdentityResolver(&mockGitHubClient{samlUsername: "hans"}),
		)
		mw := auth.IdentityMiddleware(chain, true)

		req := newRequest(&auth.User{AZP: "my-copilot-client-id", Email: "hans@nav.no"}, "")
		rec := httptest.NewRecorder()
		mw(http.HandlerFunc(handler)).ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("status: got %d, want %d, body: %s", rec.Code, http.StatusOK, rec.Body.String())
		}
	})
}
