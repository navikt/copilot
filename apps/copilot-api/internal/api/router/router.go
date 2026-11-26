// Package router builds the /api/v1/ router: the BigQuery, admin, GitHub and
// budget routes with their per-route guards.
package router

import (
	"net/http"

	"github.com/navikt/copilot/apps/copilot-api/internal/api/admin"
	"github.com/navikt/copilot/apps/copilot-api/internal/api/copilot"
	"github.com/navikt/copilot/apps/copilot-api/internal/api/httpx"
	"github.com/navikt/copilot/apps/copilot-api/internal/auth"
	"github.com/navikt/copilot/apps/copilot-api/internal/budget"
	"github.com/navikt/copilot/apps/copilot-api/internal/config"
)

// API creates the main API router for /api/v1/. identityChain is
// used to make identity resolution mandatory on per-user routes (see
// requireResolvedIdentity) — this is defense-in-depth on top of each
// handler's own requireOwnership/GetResolvedIdentity call, ensuring a future
// per-user route can't accidentally ship without an ownership check.
func API(config *config.Config, bqHandlers *copilot.Handlers, adminHandlers *admin.Handlers, ghHandlers *copilot.GitHubHandlers, budgetHandlers *budget.BudgetHandlers, identityChain *auth.IdentityResolverChain, raise *budget.BudgetRaiseHandlers) http.Handler {
	mux := http.NewServeMux()

	bqStub := httpx.ServiceUnavailableHandler("BigQuery is not configured for this environment")
	ghStub := httpx.ServiceUnavailableHandler("GitHub App is not configured for this environment")
	budgetStub := httpx.ServiceUnavailableHandler("Budget API is not configured for this environment")

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
	mux.HandleFunc("GET /api/v1/copilot/billing", gh(httpx.NilSafe(ghHandlers, func(h *copilot.GitHubHandlers) http.HandlerFunc { return h.HandleBilling })))
	mux.HandleFunc("GET /api/v1/copilot/billing/premium", gh(httpx.NilSafe(ghHandlers, func(h *copilot.GitHubHandlers) http.HandlerFunc { return h.HandlePremiumRequestUsage })))
	mux.HandleFunc("GET /api/v1/copilot/repo-contributors", gh(httpx.NilSafe(ghHandlers, func(h *copilot.GitHubHandlers) http.HandlerFunc { return h.HandleRepositoryContributors })))
	mux.HandleFunc("GET /api/v1/copilot/seats/{username}", gh(perUser(httpx.NilSafe(ghHandlers, func(h *copilot.GitHubHandlers) http.HandlerFunc { return h.HandleGetSeat }))))
	mux.HandleFunc("POST /api/v1/copilot/seats", gh(perUser(httpx.NilSafe(ghHandlers, func(h *copilot.GitHubHandlers) http.HandlerFunc { return h.HandleAssignSeat }))))
	mux.HandleFunc("DELETE /api/v1/copilot/seats/{username}", gh(perUser(httpx.NilSafe(ghHandlers, func(h *copilot.GitHubHandlers) http.HandlerFunc { return h.HandleUnassignSeat }))))
	mux.HandleFunc("GET /api/v1/copilot/saml/{identity}", gh(httpx.NilSafe(ghHandlers, func(h *copilot.GitHubHandlers) http.HandlerFunc { return h.HandleGetUsernameBySAML })))

	// Enterprise budget endpoints
	mux.HandleFunc("GET /api/v1/copilot/budget", budgetAPI(perUser(httpx.NilSafe(budgetHandlers, func(h *budget.BudgetHandlers) http.HandlerFunc { return h.HandleGetBudget }))))
	// Personal budget raises (internal/budget). Own 503 when Postgres or the
	// write token is missing, so the rest of the API is unaffected.
	budget.RegisterRaiseRoutes(mux, raise, perUser)
	mux.HandleFunc("GET /api/v1/copilot/budget/global", budgetAPI(httpx.NilSafe(budgetHandlers, func(h *budget.BudgetHandlers) http.HandlerFunc { return h.HandleGetGlobalBudget })))

	return mux
}

type adminRoute struct {
	path    string // under /api/v1/admin/
	handler func(*admin.Handlers) http.HandlerFunc
}

// adminRoutes lists every admin-only endpoint. API mounts them
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
