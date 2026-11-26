package main

import (
	"log/slog"
	"net/http"
	"strings"

	"go.opentelemetry.io/otel/trace"

	"github.com/navikt/copilot/apps/copilot-api/internal/api/admin"
	"github.com/navikt/copilot/apps/copilot-api/internal/api/copilot"
	"github.com/navikt/copilot/apps/copilot-api/internal/api/httpx"
	"github.com/navikt/copilot/apps/copilot-api/internal/api/public"
	"github.com/navikt/copilot/apps/copilot-api/internal/auth"
	"github.com/navikt/copilot/apps/copilot-api/internal/budget"
	"github.com/navikt/copilot/apps/copilot-api/internal/config"
	"github.com/navikt/copilot/apps/copilot-api/internal/problem"
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

// makeAPIRouter creates the main API router for /api/v1/. identityChain is
// used to make identity resolution mandatory on per-user routes (see
// requireResolvedIdentity) — this is defense-in-depth on top of each
// handler's own requireOwnership/GetResolvedIdentity call, ensuring a future
// per-user route can't accidentally ship without an ownership check.
func makeAPIRouter(config *config.Config, bqHandlers *copilot.Handlers, adminHandlers *admin.Handlers, ghHandlers *GitHubHandlers, budgetHandlers *budget.BudgetHandlers, identityChain *auth.IdentityResolverChain, raise *budget.BudgetRaiseHandlers) http.Handler {
	mux := http.NewServeMux()

	bqStub := serviceUnavailableHandler("BigQuery is not configured for this environment")
	ghStub := serviceUnavailableHandler("GitHub App is not configured for this environment")
	budgetStub := serviceUnavailableHandler("Budget API is not configured for this environment")

	bq := func(h http.HandlerFunc) http.HandlerFunc {
		if bqHandlers != nil {
			return h
		}
		return bqStub
	}
	// adminBQ is bq for the admin routes: both stand for the same BigQuery client.
	adminBQ := func(h http.HandlerFunc) http.HandlerFunc {
		if adminHandlers != nil {
			return h
		}
		return bqStub
	}
	gh := func(h http.HandlerFunc) http.HandlerFunc {
		if ghHandlers != nil {
			return h
		}
		return ghStub
	}
	budgetAPI := func(h http.HandlerFunc) http.HandlerFunc {
		if budgetHandlers != nil {
			return h
		}
		return budgetStub
	}
	// perUser wraps a per-user handler with mandatory identity resolution.
	// Applied only to routes whose handler ownership-checks a specific
	// {username}/caller (see requireOwnership usage in those handlers).
	perUser := func(h http.HandlerFunc) http.HandlerFunc {
		return auth.RequireResolvedIdentity(identityChain, h)
	}

	// BigQuery endpoints
	mux.HandleFunc("GET /api/v1/copilot/usage/quality", bq(httpx.NilSafe(bqHandlers, func(h *copilot.Handlers) http.HandlerFunc { return h.HandleUsageQuality })))
	mux.HandleFunc("GET /api/v1/copilot/usage/metrics", bq(httpx.NilSafe(bqHandlers, func(h *copilot.Handlers) http.HandlerFunc { return h.HandleDailyMetrics })))
	mux.HandleFunc("GET /api/v1/copilot/adoption/summary", bq(httpx.NilSafe(bqHandlers, func(h *copilot.Handlers) http.HandlerFunc { return h.HandleAdoptionSummary })))
	mux.HandleFunc("GET /api/v1/copilot/adoption/teams", bq(httpx.NilSafe(bqHandlers, func(h *copilot.Handlers) http.HandlerFunc { return h.HandleTeamAdoption })))
	mux.HandleFunc("GET /api/v1/copilot/adoption/languages", bq(httpx.NilSafe(bqHandlers, func(h *copilot.Handlers) http.HandlerFunc { return h.HandleLanguageAdoption })))
	mux.HandleFunc("GET /api/v1/copilot/adoption/staleness", bq(httpx.NilSafe(bqHandlers, func(h *copilot.Handlers) http.HandlerFunc { return h.HandleAdoptionStaleness })))
	mux.HandleFunc("GET /api/v1/copilot/customizations/details", bq(httpx.NilSafe(bqHandlers, func(h *copilot.Handlers) http.HandlerFunc { return h.HandleCustomizationDetails })))
	mux.HandleFunc("GET /api/v1/copilot/customizations/usage", bq(httpx.NilSafe(bqHandlers, func(h *copilot.Handlers) http.HandlerFunc { return h.HandleCustomizationUsage })))
	mux.HandleFunc("GET /api/v1/copilot/mimir/telemetry", bq(httpx.NilSafe(bqHandlers, func(h *copilot.Handlers) http.HandlerFunc { return h.HandleNavPilotTelemetry })))
	mux.HandleFunc("GET /api/v1/copilot/usage/team-gross", bq(httpx.NilSafe(bqHandlers, func(h *copilot.Handlers) http.HandlerFunc { return h.HandleTeamGrossOverview })))
	mux.HandleFunc("GET /api/v1/copilot/usage/team-net", bq(httpx.NilSafe(bqHandlers, func(h *copilot.Handlers) http.HandlerFunc { return h.HandleTeamNetOverview })))
	mux.HandleFunc("GET /api/v1/copilot/usage/team-year", bq(httpx.NilSafe(bqHandlers, func(h *copilot.Handlers) http.HandlerFunc { return h.HandleTeamYearOverview })))
	mux.HandleFunc("GET /api/v1/copilot/usage/spend-bands", bq(httpx.NilSafe(bqHandlers, func(h *copilot.Handlers) http.HandlerFunc { return h.HandleSpendBands })))
	mux.HandleFunc("GET /api/v1/copilot/usage/my-teams", bq(perUser(httpx.NilSafe(bqHandlers, func(h *copilot.Handlers) http.HandlerFunc { return h.HandleMyTeams }))))
	mux.HandleFunc("GET /api/v1/copilot/usage/user/{username}", bq(perUser(httpx.NilSafe(bqHandlers, func(h *copilot.Handlers) http.HandlerFunc { return h.HandleUserMetrics }))))
	mux.HandleFunc("GET /api/v1/copilot/usage/user/{username}/weekly", bq(perUser(httpx.NilSafe(bqHandlers, func(h *copilot.Handlers) http.HandlerFunc { return h.HandleUserWeeklyTrends }))))
	mux.HandleFunc("GET /api/v1/copilot/usage/user/{username}/daily-credits", bq(perUser(httpx.NilSafe(bqHandlers, func(h *copilot.Handlers) http.HandlerFunc { return h.HandleUserDailyCredits }))))
	mux.HandleFunc("GET /api/v1/copilot/usage/trends", bq(httpx.NilSafe(bqHandlers, func(h *copilot.Handlers) http.HandlerFunc { return h.HandleMonthlyTrends })))
	mux.HandleFunc("GET /api/v1/copilot/billing/monthly", bq(httpx.NilSafe(bqHandlers, func(h *copilot.Handlers) http.HandlerFunc { return h.HandleMonthlyBillingUsage })))
	mux.HandleFunc("GET /api/v1/copilot/billing/model-daily", bq(httpx.NilSafe(bqHandlers, func(h *copilot.Handlers) http.HandlerFunc { return h.HandleBillingModelDaily })))
	mux.HandleFunc("GET /api/v1/copilot/billing/model-forecast", bq(httpx.NilSafe(bqHandlers, func(h *copilot.Handlers) http.HandlerFunc { return h.HandleBillingModelForecast })))
	mux.HandleFunc("GET /api/v1/copilot/billing/monthly-trend", bq(httpx.NilSafe(bqHandlers, func(h *copilot.Handlers) http.HandlerFunc { return h.HandleBillingMonthlyTrend })))
	mux.HandleFunc("GET /api/v1/copilot/billing/model-breakdown", bq(httpx.NilSafe(bqHandlers, func(h *copilot.Handlers) http.HandlerFunc { return h.HandleBillingModelBreakdown })))
	mux.HandleFunc("GET /api/v1/copilot/adoption/cohorts", bq(httpx.NilSafe(bqHandlers, func(h *copilot.Handlers) http.HandlerFunc { return h.HandleAdoptionCohorts })))
	mux.HandleFunc("GET /api/v1/copilot/usage/daily-summary", bq(httpx.NilSafe(bqHandlers, func(h *copilot.Handlers) http.HandlerFunc { return h.HandleDailySummary })))
	mux.HandleFunc("GET /api/v1/copilot/usage/distribution", bq(httpx.NilSafe(bqHandlers, func(h *copilot.Handlers) http.HandlerFunc { return h.HandleUsageDistribution })))
	mux.HandleFunc("GET /api/v1/copilot/usage/credits-per-user", bq(httpx.NilSafe(bqHandlers, func(h *copilot.Handlers) http.HandlerFunc { return h.HandleCreditsPerUserMonthly })))
	mux.HandleFunc("GET /api/v1/copilot/usage/copilot-prs", bq(httpx.NilSafe(bqHandlers, func(h *copilot.Handlers) http.HandlerFunc { return h.HandleCopilotPRsMonthly })))
	mux.HandleFunc("GET /api/v1/copilot/usage/cohort-retention", bq(httpx.NilSafe(bqHandlers, func(h *copilot.Handlers) http.HandlerFunc { return h.HandleCohortRetention })))
	mux.HandleFunc("GET /api/v1/copilot/usage/segments", bq(httpx.NilSafe(bqHandlers, func(h *copilot.Handlers) http.HandlerFunc { return h.HandleUserSegments })))
	mux.HandleFunc("GET /api/v1/copilot/usage/repositories", bq(httpx.NilSafe(bqHandlers, func(h *copilot.Handlers) http.HandlerFunc { return h.HandleRepositoryUsage })))
	mux.HandleFunc("GET /api/v1/copilot/usage/clients", bq(httpx.NilSafe(bqHandlers, func(h *copilot.Handlers) http.HandlerFunc { return h.HandleClientUsage })))
	mux.HandleFunc("GET /api/v1/copilot/usage/editors", bq(httpx.NilSafe(bqHandlers, func(h *copilot.Handlers) http.HandlerFunc { return h.HandleEditorUsage })))

	// Admin-only cost insight: spend and budget are internal. Every admin
	// route lives on adminMux, guarded once by requireAdmin.
	adminMux := http.NewServeMux()
	for _, r := range adminRoutes() {
		h := adminBQ(httpx.NilSafe(adminHandlers, r.handler))
		adminMux.HandleFunc("GET "+r.path, h)
	}
	mux.Handle("/api/v1/admin/", auth.RequireAdmin(adminMux))

	// GitHub API endpoints
	mux.HandleFunc("GET /api/v1/copilot/billing", gh(httpx.NilSafe(ghHandlers, func(h *GitHubHandlers) http.HandlerFunc { return h.handleBilling })))
	mux.HandleFunc("GET /api/v1/copilot/billing/premium", gh(httpx.NilSafe(ghHandlers, func(h *GitHubHandlers) http.HandlerFunc { return h.handlePremiumRequestUsage })))
	mux.HandleFunc("GET /api/v1/copilot/repo-contributors", gh(httpx.NilSafe(ghHandlers, func(h *GitHubHandlers) http.HandlerFunc { return h.handleRepositoryContributors })))
	mux.HandleFunc("GET /api/v1/copilot/seats/{username}", gh(perUser(httpx.NilSafe(ghHandlers, func(h *GitHubHandlers) http.HandlerFunc { return h.handleGetSeat }))))
	mux.HandleFunc("POST /api/v1/copilot/seats", gh(perUser(httpx.NilSafe(ghHandlers, func(h *GitHubHandlers) http.HandlerFunc { return h.handleAssignSeat }))))
	mux.HandleFunc("DELETE /api/v1/copilot/seats/{username}", gh(perUser(httpx.NilSafe(ghHandlers, func(h *GitHubHandlers) http.HandlerFunc { return h.handleUnassignSeat }))))
	mux.HandleFunc("GET /api/v1/copilot/saml/{identity}", gh(httpx.NilSafe(ghHandlers, func(h *GitHubHandlers) http.HandlerFunc { return h.handleGetUsernameBySAML })))

	// Enterprise budget endpoints
	mux.HandleFunc("GET /api/v1/copilot/budget", budgetAPI(perUser(httpx.NilSafe(budgetHandlers, func(h *budget.BudgetHandlers) http.HandlerFunc { return h.HandleGetBudget }))))
	// Personal budget raises (internal/budget). Own 503 when Postgres or the
	// write token is missing, so the rest of the API is unaffected.
	budget.RegisterRaiseRoutes(mux, raise, perUser)
	mux.HandleFunc("GET /api/v1/copilot/budget/global", budgetAPI(httpx.NilSafe(budgetHandlers, func(h *budget.BudgetHandlers) http.HandlerFunc { return h.HandleGetGlobalBudget })))

	return mux
}

// makePublicRouter creates the public router for unauthenticated endpoints.
func makePublicRouter(config *config.Config, videoHandlers *VideoHandlers, publicHandlers *public.Handlers) http.Handler {
	mux := http.NewServeMux()

	videoStub := serviceUnavailableHandler("Video service is not configured for this environment")
	video := func(h http.HandlerFunc) http.HandlerFunc {
		if videoHandlers != nil {
			return h
		}
		return videoStub
	}

	mux.HandleFunc("GET /public/v1/videos", video(httpx.NilSafe(videoHandlers, func(h *VideoHandlers) http.HandlerFunc { return h.handleVideoFeed })))
	mux.HandleFunc("GET /public/v1/videos/{id}", video(httpx.NilSafe(videoHandlers, func(h *VideoHandlers) http.HandlerFunc { return h.handleVideoDetail })))
	mux.HandleFunc("GET /public/v1/videos/{id}/play", video(httpx.NilSafe(videoHandlers, func(h *VideoHandlers) http.HandlerFunc { return h.handleVideoPlay })))
	mux.HandleFunc("GET /public/v1/videos/{id}/captions", video(httpx.NilSafe(videoHandlers, func(h *VideoHandlers) http.HandlerFunc { return h.handleVideoCaptions })))
	fact := serviceUnavailableHandler("BigQuery is not configured for this environment")
	if publicHandlers != nil {
		fact = publicHandlers.HandlePublicFact
	}
	mux.HandleFunc("GET /public/v1/fact", fact)

	return mux
}

func serviceUnavailableHandler(msg string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		respondError(w, "service_unavailable", msg, http.StatusServiceUnavailable)
	}
}

// Kept as package names so the handlers need no change; they live in
// internal/problem because the auth middleware writes the same responses,
// and in internal/api/httpx because the handler packages share them.
var (
	noCacheControl = problem.NoCache
	respondError   = problem.Respond
	respondJSON    = httpx.RespondJSON
	cacheControl   = httpx.CacheControl
	requireMethod  = httpx.RequireMethod
)

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

type adminRoute struct {
	path    string // under /api/v1/admin/
	handler func(*admin.Handlers) http.HandlerFunc
}

// adminRoutes lists every admin-only endpoint. makeAPIRouter mounts them
// behind a single requireAdmin.
func adminRoutes() []adminRoute {
	return []adminRoute{
		{"/api/v1/admin/billing/monthly", func(h *admin.Handlers) http.HandlerFunc { return h.HandleAdminMonthly }},
		{"/api/v1/admin/billing/active-users", func(h *admin.Handlers) http.HandlerFunc { return h.HandleAdminActiveUsers }},
		{"/api/v1/admin/billing/high-consumers", func(h *admin.Handlers) http.HandlerFunc { return h.HandleAdminHighConsumers }},
		{"/api/v1/admin/billing/budget-days", func(h *admin.Handlers) http.HandlerFunc { return h.HandleAdminBudgetDays }},
		{"/api/v1/admin/billing/anomalies", func(h *admin.Handlers) http.HandlerFunc { return h.HandleCostAnomalies }},
		{"/api/v1/admin/billing/cost-forecast", func(h *admin.Handlers) http.HandlerFunc { return h.HandleCostForecast }},
		{"/api/v1/admin/billing/model-users", func(h *admin.Handlers) http.HandlerFunc { return h.HandleModelFamilyUsers }},
		{"/api/v1/admin/billing/volume", func(h *admin.Handlers) http.HandlerFunc { return h.HandleAdminVolume }},
	}
}
