package main

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
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
	if !authMiddlewareReady.Load() {
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

// makeAPIRouter creates the main API router for /api/v1/
func makeAPIRouter(config *Config, bqHandlers *BigQueryHandlers, ghHandlers *GitHubHandlers, budgetHandlers *BudgetHandlers) http.Handler {
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
	gh := func(h http.HandlerFunc) http.HandlerFunc {
		if ghHandlers != nil {
			return h
		}
		return ghStub
	}
	budget := func(h http.HandlerFunc) http.HandlerFunc {
		if budgetHandlers != nil {
			return h
		}
		return budgetStub
	}

	// BigQuery endpoints
	mux.HandleFunc("GET /api/v1/copilot/usage/metrics", bq(nilSafe(bqHandlers, func(h *BigQueryHandlers) http.HandlerFunc { return h.handleDailyMetrics })))
	mux.HandleFunc("GET /api/v1/copilot/adoption/summary", bq(nilSafe(bqHandlers, func(h *BigQueryHandlers) http.HandlerFunc { return h.handleAdoptionSummary })))
	mux.HandleFunc("GET /api/v1/copilot/adoption/teams", bq(nilSafe(bqHandlers, func(h *BigQueryHandlers) http.HandlerFunc { return h.handleTeamAdoption })))
	mux.HandleFunc("GET /api/v1/copilot/adoption/languages", bq(nilSafe(bqHandlers, func(h *BigQueryHandlers) http.HandlerFunc { return h.handleLanguageAdoption })))
	mux.HandleFunc("GET /api/v1/copilot/adoption/staleness", bq(nilSafe(bqHandlers, func(h *BigQueryHandlers) http.HandlerFunc { return h.handleAdoptionStaleness })))
	mux.HandleFunc("GET /api/v1/copilot/customizations/details", bq(nilSafe(bqHandlers, func(h *BigQueryHandlers) http.HandlerFunc { return h.handleCustomizationDetails })))
	mux.HandleFunc("GET /api/v1/copilot/customizations/usage", bq(nilSafe(bqHandlers, func(h *BigQueryHandlers) http.HandlerFunc { return h.handleCustomizationUsage })))
	mux.HandleFunc("GET /api/v1/copilot/usage/team-summary", bq(nilSafe(bqHandlers, func(h *BigQueryHandlers) http.HandlerFunc { return h.handleTeamUsageSummary })))
	mux.HandleFunc("GET /api/v1/copilot/usage/user/{username}", bq(nilSafe(bqHandlers, func(h *BigQueryHandlers) http.HandlerFunc { return h.handleUserMetrics })))
	mux.HandleFunc("GET /api/v1/copilot/usage/user/{username}/weekly", bq(nilSafe(bqHandlers, func(h *BigQueryHandlers) http.HandlerFunc { return h.handleUserWeeklyTrends })))
	mux.HandleFunc("GET /api/v1/copilot/usage/trends", bq(nilSafe(bqHandlers, func(h *BigQueryHandlers) http.HandlerFunc { return h.handleMonthlyTrends })))
	mux.HandleFunc("GET /api/v1/copilot/usage/models", bq(nilSafe(bqHandlers, func(h *BigQueryHandlers) http.HandlerFunc { return h.handleMonthlyModelUsage })))
	mux.HandleFunc("GET /api/v1/copilot/billing/monthly", bq(nilSafe(bqHandlers, func(h *BigQueryHandlers) http.HandlerFunc { return h.handleMonthlyBillingUsage })))
	mux.HandleFunc("GET /api/v1/copilot/adoption/cohorts", bq(nilSafe(bqHandlers, func(h *BigQueryHandlers) http.HandlerFunc { return h.handleAdoptionCohorts })))

	// GitHub API endpoints
	mux.HandleFunc("GET /api/v1/copilot/billing", gh(nilSafe(ghHandlers, func(h *GitHubHandlers) http.HandlerFunc { return h.handleBilling })))
	mux.HandleFunc("GET /api/v1/copilot/billing/premium", gh(nilSafe(ghHandlers, func(h *GitHubHandlers) http.HandlerFunc { return h.handlePremiumRequestUsage })))
	mux.HandleFunc("GET /api/v1/copilot/repo-contributors", gh(nilSafe(ghHandlers, func(h *GitHubHandlers) http.HandlerFunc { return h.handleRepositoryContributors })))
	mux.HandleFunc("GET /api/v1/copilot/seats/{username}", gh(nilSafe(ghHandlers, func(h *GitHubHandlers) http.HandlerFunc { return h.handleGetSeat })))
	mux.HandleFunc("POST /api/v1/copilot/seats", gh(nilSafe(ghHandlers, func(h *GitHubHandlers) http.HandlerFunc { return h.handleAssignSeat })))
	mux.HandleFunc("DELETE /api/v1/copilot/seats/{username}", gh(nilSafe(ghHandlers, func(h *GitHubHandlers) http.HandlerFunc { return h.handleUnassignSeat })))
	mux.HandleFunc("GET /api/v1/copilot/saml/{identity}", gh(nilSafe(ghHandlers, func(h *GitHubHandlers) http.HandlerFunc { return h.handleGetUsernameBySAML })))

	// Enterprise budget endpoints
	mux.HandleFunc("GET /api/v1/copilot/budget", budget(nilSafe(budgetHandlers, func(h *BudgetHandlers) http.HandlerFunc { return h.handleGetBudget })))
	mux.HandleFunc("GET /api/v1/copilot/budget/global", budget(nilSafe(budgetHandlers, func(h *BudgetHandlers) http.HandlerFunc { return h.handleGetGlobalBudget })))

	// Placeholder endpoints - to be implemented in future phases
	mux.HandleFunc("/api/v1/copilot/usage/summary", notImplementedHandler)
	mux.HandleFunc("/api/v1/copilot/usage/features", notImplementedHandler)
	mux.HandleFunc("/api/v1/copilot/usage/languages", notImplementedHandler)
	mux.HandleFunc("/api/v1/copilot/usage/editors", notImplementedHandler)
	mux.HandleFunc("/api/v1/mcp/servers", notImplementedHandler)

	return mux
}

// nilSafe extracts a handler method from a potentially-nil handler struct.
// The caller is responsible for guarding against nil before using the result.
func nilSafe[T any](h *T, fn func(*T) http.HandlerFunc) http.HandlerFunc {
	if h == nil {
		return nil
	}
	return fn(h)
}

func serviceUnavailableHandler(msg string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		respondError(w, "service_unavailable", msg, http.StatusServiceUnavailable)
	}
}

func notImplementedHandler(w http.ResponseWriter, r *http.Request) {
	respondError(w, "not_implemented", "This endpoint is not yet implemented", http.StatusNotImplemented)
}

// cacheControl sets HTTP cache headers for responses
// duration: cache duration in seconds (max-age and s-maxage)
// public: if true, cache is shareable; if false, cache is private to the user
func cacheControl(w http.ResponseWriter, duration int, public bool) {
	var policy string
	if public {
		policy = fmt.Sprintf("public, max-age=%d, s-maxage=%d", duration, duration)
	} else {
		policy = fmt.Sprintf("private, max-age=%d", duration)
	}
	w.Header().Set("Cache-Control", policy)
}

// noCacheControl disables caching for responses (mutations, sensitive data)
func noCacheControl(w http.ResponseWriter) {
	w.Header().Set("Cache-Control", "no-cache, no-store, must-revalidate")
	w.Header().Set("Pragma", "no-cache")
	w.Header().Set("Expires", "0")
}

// RFC 7807 Problem Details response
type ProblemDetail struct {
	Type     string `json:"type"`
	Title    string `json:"title"`
	Status   int    `json:"status"`
	Detail   string `json:"detail,omitempty"`
	Instance string `json:"instance,omitempty"`
}

func respondError(w http.ResponseWriter, errorType, detail string, status int) {
	problem := ProblemDetail{
		Type:     "about:blank",
		Title:    http.StatusText(status),
		Status:   status,
		Detail:   detail,
		Instance: "",
	}

	if errorType != "" {
		problem.Type = "https://copilot-api.nav.no/errors/" + errorType
	}

	w.Header().Set("Content-Type", "application/problem+json")
	noCacheControl(w)
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(problem); err != nil {
		slog.Error("Failed to encode problem details response", "error", err)
	}
}

func respondJSON(w http.ResponseWriter, data interface{}, status int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(data); err != nil {
		slog.Error("Failed to encode JSON response", "error", err)
	}
}

// loggingMiddleware logs HTTP requests
func loggingMiddleware(config *Config, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		shouldLog := false
		for endpoint := range config.LoggedEndpoints {
			if strings.HasPrefix(r.URL.Path, endpoint) {
				shouldLog = true
				break
			}
		}

		if shouldLog {
			slog.Info("HTTP request",
				"method", r.Method,
				"path", r.URL.Path,
				"remote_addr", r.RemoteAddr,
			)
		}

		next.ServeHTTP(w, r)
	})
}
