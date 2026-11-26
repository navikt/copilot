package main

import (
	"cmp"
	"context"
	"errors"
	"log/slog"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/navikt/copilot/apps/copilot-api/internal/api/admin"
	"github.com/navikt/copilot/apps/copilot-api/internal/api/copilot"
	"github.com/navikt/copilot/apps/copilot-api/internal/api/public"
	bqstore "github.com/navikt/copilot/apps/copilot-api/internal/bigquery"

	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"

	"github.com/navikt/copilot/apps/copilot-api/internal/auth"

	"github.com/navikt/copilot/apps/copilot-api/internal/budget"
	"github.com/navikt/copilot/apps/copilot-api/internal/config"
	"github.com/navikt/copilot/apps/copilot-api/internal/github"
)

func main() {
	config := config.Load()

	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: config.LogLevel,
	}))
	slog.SetDefault(logger)

	slog.Info("Starting Copilot API server",
		"port", config.Port,
		"environment", config.Environment,
		"log_level", config.LogLevel.String(),
	)

	// Initialize OTel tracing — must happen before any instrumented handlers are set up.
	// Reads OTEL_EXPORTER_OTLP_ENDPOINT injected by Nais (runtime: sdk).
	ctx := context.Background()
	shutdownTracer, err := initTracer(ctx, "copilot-api")
	if err != nil {
		slog.Warn("OTel tracer initialization failed — tracing disabled", "error", err)
	} else {
		defer func() {
			if err := shutdownTracer(ctx); err != nil {
				slog.Warn("OTel tracer shutdown error", "error", err)
			}
		}()
	}

	// Initialize GitHub client (optional - metrics will show zeros if not configured)
	var githubClient *github.Client
	var ghHandlers *GitHubHandlers
	if config.GitHubAppID != "" && config.GitHubAppPrivateKey != "" && config.GitHubInstallationID != "" {
		var err error
		githubClient, err = github.New(config.GitHubOrg, config.GitHubAppID, config.GitHubAppPrivateKey, config.GitHubInstallationID)
		if err != nil {
			slog.Warn("GitHub client initialization failed - metrics will be unavailable", "error", err)
		} else {
			ghHandlers = newGitHubHandlers(githubClient)
			slog.Info("GitHub client initialized successfully")
		}
	} else {
		slog.Warn("GitHub App credentials not configured - metrics will show zeros")
	}

	// Start background metrics collector
	startMetricsCollector(config, githubClient)

	// Initialize BigQuery client (optional - endpoints will error if not configured)
	var cachedBQClient *bqstore.CachedBigQueryClient
	var rawBQClient *bqstore.BigQueryClient
	if config.GCPProjectID != "" {
		bqClient, err := bqstore.NewBigQueryClient(bqstore.Config{
			GCPProjectID:           config.GCPProjectID,
			CopilotMetricsDataset:  config.CopilotMetricsDataset,
			CopilotMetricsTable:    config.CopilotMetricsTable,
			CopilotAdoptionDataset: config.CopilotAdoptionDataset,
			CopilotMimirDataset:    config.CopilotMimirDataset,
		})
		if err != nil {
			slog.Warn("BigQuery client initialization failed - data endpoints will be unavailable", "error", err)
		} else {
			rawBQClient = bqClient
			cacheTTL := time.Duration(config.CacheTTLHours) * time.Hour
			cachedBQClient = bqstore.NewCachedBigQueryClient(bqClient, cacheTTL)
			slog.Info("BigQuery client initialized successfully", "cache_ttl", cacheTTL)
		}
	} else {
		slog.Warn("GCP_TEAM_PROJECT_ID not configured - BigQuery endpoints will be unavailable")
	}
	if cachedBQClient != nil {
		defer cachedBQClient.Close()
	}

	// Initialize budget client (optional - requires the BUDGET_GITHUB_APP_* enterprise app)
	var budgetHandlers *budget.BudgetHandlers
	// Stays a nil interface without a budget client: a typed nil would not.
	var globalBudget budget.GlobalBudgetGetter
	billingToken, billingErr := budget.NewBillingTokenSource(config)
	switch {
	case billingErr != nil:
		slog.Error("Budget app unavailable - budget endpoint will be unavailable", "error", billingErr)
	case billingToken == nil:
		slog.Warn("BUDGET_GITHUB_APP_* not configured - budget endpoint will be unavailable")
	case githubClient == nil:
		slog.Warn("GitHub client unavailable - budget endpoint will be unavailable (see GitHub client initialization error above)")
	default:
		budgetClient := budget.NewBudgetClient(billingToken, config.GitHubEnterprise)
		budgetHandlers = budget.NewBudgetHandlers(budgetClient, func() int64 {
			metricsCollector.mu.RLock()
			defer metricsCollector.mu.RUnlock()
			return metricsCollector.githubSeatsActive
		})
		globalBudget = budgetClient
		slog.Info("Budget client initialized successfully")
	}

	var bqHandlers *copilot.Handlers
	var adminHandlers *admin.Handlers
	var publicHandlers *public.Handlers
	if cachedBQClient != nil {
		bqHandlers = copilot.New(copilot.Deps{
			BQ:     cachedBQClient,
			Budget: globalBudget,
			Seats: func() int64 {
				metricsCollector.mu.RLock()
				defer metricsCollector.mu.RUnlock()
				return metricsCollector.githubSeatsTotal
			},
		})
		adminHandlers = admin.New(admin.Deps{BQ: cachedBQClient, Budget: globalBudget})
		publicHandlers = public.New(public.Deps{BQ: cachedBQClient})
	}

	// Personal budget raises: Postgres (DB_URL from Nais) plus a write token.
	var seats budget.SeatLookup
	if githubClient != nil {
		seats = githubClient
	}
	var teams budget.TeamSource
	if cachedBQClient != nil {
		teams = cachedBQClient
	}
	raiseHandlers, err := budget.NewBudgetRaiseHandlersFromConfig(config, seats, teams)
	if err != nil {
		slog.Error("Budget raises disabled", "error", err)
	}
	if budgetHandlers != nil {
		budgetHandlers.SetRaises(raiseHandlers)
	}

	// Build the IdentityResolver chain (see identity.go) that determines "who
	// is this caller, as a GitHub username?" independent of which mechanism
	// (SAML or copilot-cli's trusted X-On-Behalf-Of header) authenticated the
	// request. Per-user handlers (handleUserMetrics, handleGetBudget, ...)
	// call requireOwnership/GetResolvedIdentity and have no knowledge of how
	// the identity was resolved.
	//
	// Order matters: OnBehalfOfIdentityResolver must be registered before
	// SAMLIdentityResolver since a trusted M2M token typically has no email
	// claim for SAML to resolve.
	//
	// The copilot-cli client ID is derived from AZURE_APP_PRE_AUTHORIZED_APPS
	// (NAIS auto-populates it from accessPolicy.inbound.rules) rather than a
	// separate manual secret — the same data that already gates M2M auth in
	// newTokenValidator. If copilot-cli isn't a pre-authorized inbound app the
	// trust path stays disabled (fails closed).
	var identityResolvers []auth.IdentityResolver
	copilotCLIClientID, err := auth.TrustedClientIDForApp(config.PreAuthorizedApps, "copilot-cli")
	if err != nil {
		slog.Warn("Could not parse AZURE_APP_PRE_AUTHORIZED_APPS — X-On-Behalf-Of trust disabled", "error", err)
	}
	if copilotCLIClientID != "" {
		identityResolvers = append(identityResolvers, auth.NewOnBehalfOfIdentityResolver(map[string]bool{
			copilotCLIClientID: true,
		}))
		slog.Info("copilot-cli trusted for X-On-Behalf-Of on read-only per-user endpoints", "client_id", copilotCLIClientID)
	} else {
		slog.Debug("copilot-cli not found in pre-authorized apps — X-On-Behalf-Of trust disabled")
	}
	if githubClient != nil {
		identityResolvers = append(identityResolvers, auth.NewSAMLIdentityResolver(githubClient))
	}
	identityChain := auth.NewIdentityResolverChain(identityResolvers...)
	// required=false: identity resolution failures only matter to the
	// specific per-user handlers that call requireOwnership/GetResolvedIdentity
	// (which return their own 401/403). Team/org-level aggregate endpoints
	// must keep working even for callers with no resolvable GitHub identity.
	identityMiddleware := auth.IdentityMiddleware(identityChain, false)

	// Middleware
	authMiddleware := auth.Middleware(auth.Config{
		Environment:       config.Environment,
		AzureIssuer:       config.AzureIssuer,
		AzureClientID:     config.AzureClientID,
		AzureJWKSURI:      config.AzureJWKSURI,
		PreAuthorizedApps: config.PreAuthorizedApps,
	})
	videoHandlers := newVideoHandlers(config)

	// Set up routing on a local mux (not global DefaultServeMux)
	mux := http.NewServeMux()

	// Public endpoints
	mux.HandleFunc("/health", healthHandler)
	mux.HandleFunc("/ready", readyHandler)
	mux.Handle("/metrics", metricsHandler())

	registerDevRoutes(mux, config, rawBQClient)
	mux.Handle("/public/v1/", otelhttp.NewHandler(
		loggingMiddleware(config, makePublicRouter(config, videoHandlers, publicHandlers)),
		"public-api",
		otelhttp.WithSpanNameFormatter(func(_ string, r *http.Request) string {
			return r.Method + " " + r.URL.Path
		}),
	))

	// Protected API endpoints — wrapped with OTel tracing
	mux.Handle("/api/v1/", traceAPI(
		loggingMiddleware(config, authMiddleware(identityMiddleware(makeAPIRouter(config, bqHandlers, adminHandlers, ghHandlers, budgetHandlers, identityChain, raiseHandlers)))),
	))

	// copilot-survey looks up a nav-pilot user's Nav e-mail here. Not under
	// /api/v1/, so no identity chain, request log or trace (see
	// samlNameIDHandler). Off unless copilot-survey is pre-authorized.
	// authMiddleware refuses copilot-survey's tokens on every other path.
	copilotSurveyClientID, err := auth.TrustedClientIDForApp(config.PreAuthorizedApps, "copilot-survey")
	if err != nil {
		slog.Warn("Could not parse AZURE_APP_PRE_AUTHORIZED_APPS — SAML name-id route disabled", "error", err)
	}
	var nameIDLookup func(context.Context, string) (string, error)
	if githubClient != nil {
		nameIDLookup = githubClient.GetSamlNameIDByLogin
	}
	// copilot-cli asks here whether a nav-pilot user is an active navikt
	// member, so the nav-pilot GitHub App needs no permissions. Off unless
	// copilot-cli is pre-authorized (the same client id as X-On-Behalf-Of).
	var memberCheck func(context.Context, string) (bool, error)
	if githubClient != nil {
		memberCheck = githubClient.IsActiveOrgMember
	}
	registerInternalRoutes(mux, authMiddleware, copilotSurveyClientID, nameIDLookup, copilotCLIClientID, memberCheck)

	slog.Info("Server listening", "port", config.Port)

	server := &http.Server{
		Addr:              ":" + config.Port,
		Handler:           httpMetrics(mux),
		ReadTimeout:       30 * time.Second,
		ReadHeaderTimeout: 10 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       120 * time.Second,
	}

	// Graceful shutdown
	sigCtx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	if raiseHandlers != nil {
		go raiseHandlers.RunReconciler(sigCtx, cmp.Or(config.BudgetReconcileInterval, 5*time.Minute))
	}

	go func() {
		if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			slog.Error("Server failed", "error", err)
			os.Exit(1)
		}
	}()

	<-sigCtx.Done()
	slog.Info("Shutting down gracefully...")

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := server.Shutdown(shutdownCtx); err != nil {
		slog.Error("Server shutdown failed", "error", err)
		os.Exit(1)
	}

	slog.Info("Server stopped")
}

// registerDevRoutes registers development-only endpoints.
//
// The raw-SQL /dev/query endpoint is unauthenticated, so it is double-gated:
// Environment must be "local" AND ENABLE_DEV_QUERY=true must be set
// explicitly. Environment defaults to "local" when NAIS_CLUSTER_NAME is
// unset, meaning the environment check alone would fail OPEN in a
// misconfigured deployment. The explicit ENABLE_DEV_QUERY opt-in ensures an
// unauthenticated raw-SQL endpoint can never appear because an env var went
// missing.
func registerDevRoutes(mux *http.ServeMux, config *config.Config, rawBQClient *bqstore.BigQueryClient) {
	if rawBQClient == nil || config.Environment != "local" || !config.EnableDevQuery {
		return
	}
	slog.Warn("Registering unauthenticated /dev/query endpoint (local development only)")
	mux.HandleFunc("/dev/query", rawBQClient.DevQueryHandler)
}

// traceAPI wraps the /api/v1/ handler in otelhttp, except the SAML lookup
// (see tracePath). otelhttp names the span after the path and records it as
// url.path, so it sees the redacted path, and the handler gets the real one.
func traceAPI(h http.Handler, opts ...otelhttp.Option) http.Handler {
	type realURL struct{}
	traced := otelhttp.NewHandler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if u, ok := r.Context().Value(realURL{}).(*url.URL); ok {
			r = r.WithContext(r.Context())
			r.URL = u
		}
		h.ServeHTTP(w, r)
	}), "api", append([]otelhttp.Option{
		otelhttp.WithFilter(tracePath),
		otelhttp.WithSpanNameFormatter(func(_ string, r *http.Request) string {
			return r.Method + " " + r.URL.Path
		}),
	}, opts...)...)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if p := auth.RedactPath(r.URL.Path); p != r.URL.Path {
			redacted := *r.URL
			redacted.Path, redacted.RawPath = p, ""
			r = r.WithContext(context.WithValue(r.Context(), realURL{}, r.URL))
			r.URL = &redacted
		}
		traced.ServeHTTP(w, r)
	})
}
