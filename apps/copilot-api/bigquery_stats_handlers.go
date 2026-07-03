package main

import (
	"context"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"time"
)

// verifyUsernameOwnership resolves the authenticated caller's GitHub username
// via SAML and compares it to the requested username. Returns true if ownership
// is verified (or if no githubClient is configured — graceful degradation).
// On failure it writes an appropriate error response and returns false.
//
// SECURITY: This check is required on ALL per-user read endpoints. Without it,
// any employee can read any colleague's personal Copilot activity (daily credits,
// acceptance counts, lines of code) by simply passing a different ?username= param.
// The frontend supplies the username from the user's own SAML-resolved identity,
// but the backend must not trust that — verify server-side via SAML lookup.
func (h *BigQueryHandlers) verifyUsernameOwnership(w http.ResponseWriter, r *http.Request, requestedUsername string) bool {
	if h.githubClient == nil {
		// No SAML client available — allow through (local/dev environments).
		return true
	}

	user, ok := getUserFromContext(r.Context())
	if !ok || user == nil {
		respondError(w, "unauthorized", "Authentication required", http.StatusUnauthorized)
		return false
	}

	resolvedUsername, err := h.githubClient.getUsernameBySamlIdentity(r.Context(), user.Email)
	if err != nil {
		slog.Error("Failed to verify caller identity via SAML", "error", err)
		respondError(w, "identity_check_failed", "Failed to verify user identity", http.StatusInternalServerError)
		return false
	}
	if resolvedUsername == "" {
		respondError(w, "no_github_account", "No GitHub account linked to your identity", http.StatusForbidden)
		return false
	}
	if !strings.EqualFold(resolvedUsername, requestedUsername) {
		slog.Warn("Per-user read denied: username mismatch",
			"requested_username", requestedUsername,
			"actor_navident", user.NAVident,
		)
		respondError(w, "forbidden", "You can only view your own usage data", http.StatusForbidden)
		return false
	}
	return true
}

func optionalIntParam(r *http.Request, name string, defaultValue, minValue, maxValue int) (int, bool) {
	value := r.URL.Query().Get(name)
	if value == "" {
		return defaultValue, true
	}
	parsed, err := strconv.Atoi(value)
	if err != nil || parsed < minValue || parsed > maxValue {
		return 0, false
	}
	return parsed, true
}

func isValidUsageUsername(username string) bool {
	return username != "" && len(username) <= 39 && !strings.Contains(username, "/") && isValidGitHubUsername(username)
}

func optionalMonthParam(r *http.Request, name string) (string, bool) {
	value := r.URL.Query().Get(name)
	if value == "" {
		return time.Now().UTC().Format("2006-01"), true
	}
	if !isValidYearMonth(value) {
		return "", false
	}
	return value, true
}

func (h *BigQueryHandlers) handleTeamUsageSummary(w http.ResponseWriter, r *http.Request) {
	days, ok := optionalIntParam(r, "days", 7, 1, 365)
	if !ok {
		respondError(w, "invalid_parameter", "days must be between 1 and 365", http.StatusBadRequest)
		return
	}

	usage, err := h.bqClient.GetTeamUsageSummary(r.Context(), days)
	if err != nil {
		slog.Error("Failed to fetch team usage summary", "error", err)
		respondError(w, "internal_error", "Failed to fetch team usage summary", http.StatusInternalServerError)
		return
	}

	cacheControl(w, 3600, false)
	respondJSON(w, usage, http.StatusOK)
}

func (h *BigQueryHandlers) handleUserMetrics(w http.ResponseWriter, r *http.Request) {
	username := r.PathValue("username")
	if !isValidUsageUsername(username) {
		respondError(w, "invalid_parameter", "Invalid GitHub username", http.StatusBadRequest)
		return
	}

	if !h.verifyUsernameOwnership(w, r, username) {
		return
	}

	days, ok := optionalIntParam(r, "days", 7, 1, 365)
	if !ok {
		respondError(w, "invalid_parameter", "days must be between 1 and 365", http.StatusBadRequest)
		return
	}

	metrics, err := h.bqClient.GetUserMetrics(r.Context(), username, days)
	if err != nil {
		slog.Error("Failed to fetch user metrics", "error", err)
		respondError(w, "internal_error", "Failed to fetch user metrics", http.StatusInternalServerError)
		return
	}
	if metrics == nil {
		respondError(w, "not_found", "No user metrics found", http.StatusNotFound)
		return
	}

	cacheControl(w, 300, false)
	respondJSON(w, metrics, http.StatusOK)
}

func (h *BigQueryHandlers) handleMonthlyTrends(w http.ResponseWriter, r *http.Request) {
	months, ok := optionalIntParam(r, "months", 12, 1, 36)
	if !ok {
		respondError(w, "invalid_parameter", "months must be between 1 and 36", http.StatusBadRequest)
		return
	}

	trends, err := h.bqClient.GetMonthlyTrends(r.Context(), months)
	if err != nil {
		slog.Error("Failed to fetch monthly trends", "error", err)
		respondError(w, "internal_error", "Failed to fetch monthly trends", http.StatusInternalServerError)
		return
	}

	cacheControl(w, 3600, false)
	respondJSON(w, trends, http.StatusOK)
}

func (h *BigQueryHandlers) handleMonthlyModelUsage(w http.ResponseWriter, r *http.Request) {
	months, ok := optionalIntParam(r, "months", 12, 1, 36)
	if !ok {
		respondError(w, "invalid_parameter", "months must be between 1 and 36", http.StatusBadRequest)
		return
	}

	usage, err := h.bqClient.GetMonthlyModelUsage(r.Context(), months)
	if err != nil {
		slog.Error("Failed to fetch monthly model usage", "error", err)
		respondError(w, "internal_error", "Failed to fetch monthly model usage", http.StatusInternalServerError)
		return
	}

	cacheControl(w, 3600, false)
	respondJSON(w, usage, http.StatusOK)
}

func (h *BigQueryHandlers) handleMonthlyBillingUsage(w http.ResponseWriter, r *http.Request) {
	months, ok := optionalIntParam(r, "months", 12, 1, 36)
	if !ok {
		respondError(w, "invalid_parameter", "months must be between 1 and 36", http.StatusBadRequest)
		return
	}

	usage, err := h.bqClient.GetMonthlyBillingUsage(r.Context(), months)
	if err != nil {
		slog.Error("Failed to fetch monthly billing usage", "error", err)
		respondError(w, "internal_error", "Failed to fetch monthly billing usage", http.StatusInternalServerError)
		return
	}

	cacheControl(w, 3600, false)
	respondJSON(w, usage, http.StatusOK)
}

func (h *BigQueryHandlers) handleBillingModelDaily(w http.ResponseWriter, r *http.Request) {
	month, ok := optionalMonthParam(r, "month")
	if !ok {
		respondError(w, "invalid_parameter", "month must be in YYYY-MM format", http.StatusBadRequest)
		return
	}

	usage, err := h.bqClient.GetBillingModelDailyCosts(r.Context(), month)
	if err != nil {
		slog.Error("Failed to fetch billing model daily costs", "error", err)
		respondError(w, "internal_error", "Failed to fetch billing model daily costs", http.StatusInternalServerError)
		return
	}
	if usage == nil {
		usage = []BillingModelDailyCost{}
	}

	cacheControl(w, 900, false)
	respondJSON(w, usage, http.StatusOK)
}

func (h *BigQueryHandlers) handleBillingModelForecast(w http.ResponseWriter, r *http.Request) {
	month, ok := optionalMonthParam(r, "month")
	if !ok {
		respondError(w, "invalid_parameter", "month must be in YYYY-MM format", http.StatusBadRequest)
		return
	}

	forecast, err := h.bqClient.GetBillingModelForecast(r.Context(), month)
	if err != nil {
		slog.Error("Failed to fetch billing model forecast", "error", err)
		respondError(w, "internal_error", "Failed to fetch billing model forecast", http.StatusInternalServerError)
		return
	}

	cacheControl(w, 900, false)
	respondJSON(w, forecast, http.StatusOK)
}

func (h *BigQueryHandlers) handleUserWeeklyTrends(w http.ResponseWriter, r *http.Request) {
	username := r.PathValue("username")
	if !isValidUsageUsername(username) {
		respondError(w, "invalid_parameter", "Invalid GitHub username", http.StatusBadRequest)
		return
	}

	if !h.verifyUsernameOwnership(w, r, username) {
		return
	}

	weeks, ok := optionalIntParam(r, "weeks", 12, 1, 52)
	if !ok {
		respondError(w, "invalid_parameter", "weeks must be between 1 and 52", http.StatusBadRequest)
		return
	}

	trends, err := h.bqClient.GetUserWeeklyTrends(r.Context(), username, weeks)
	if err != nil {
		slog.Error("Failed to fetch user weekly trends", "error", err)
		respondError(w, "internal_error", "Failed to fetch user weekly trends", http.StatusInternalServerError)
		return
	}

	cacheControl(w, 300, false)
	respondJSON(w, trends, http.StatusOK)
}

func (h *BigQueryHandlers) handleUserDailyCredits(w http.ResponseWriter, r *http.Request) {
	username := r.PathValue("username")
	if !isValidUsageUsername(username) {
		respondError(w, "invalid_parameter", "Invalid GitHub username", http.StatusBadRequest)
		return
	}

	if !h.verifyUsernameOwnership(w, r, username) {
		return
	}

	days, ok := optionalIntParam(r, "days", 30, 1, 90)
	if !ok {
		respondError(w, "invalid_parameter", "days must be between 1 and 90", http.StatusBadRequest)
		return
	}

	credits, err := h.bqClient.GetUserDailyCredits(r.Context(), username, days)
	if err != nil {
		slog.Error("Failed to fetch user daily credits", "error", err)
		respondError(w, "internal_error", "Failed to fetch user daily credits", http.StatusInternalServerError)
		return
	}

	cacheControl(w, 300, false)
	respondJSON(w, credits, http.StatusOK)
}

func (h *BigQueryHandlers) handleAdoptionCohorts(w http.ResponseWriter, r *http.Request) {
	days, ok := optionalIntParam(r, "days", 90, 1, 365)
	if !ok {
		respondError(w, "invalid_parameter", "days must be between 1 and 365", http.StatusBadRequest)
		return
	}

	cohorts, err := h.bqClient.GetAdoptionCohorts(r.Context(), days)
	if err != nil {
		slog.Error("Failed to fetch adoption cohorts", "error", err)
		respondError(w, "internal_error", "Failed to fetch adoption cohorts", http.StatusInternalServerError)
		return
	}

	cacheControl(w, 3600, false)
	respondJSON(w, cohorts, http.StatusOK)
}

func (h *BigQueryHandlers) handleBillingMonthlyTrend(w http.ResponseWriter, r *http.Request) {
	months, ok := optionalIntParam(r, "months", 12, 1, 36)
	if !ok {
		respondError(w, "invalid_parameter", "months must be between 1 and 36", http.StatusBadRequest)
		return
	}

	trend, err := h.bqClient.GetBillingMonthlyTrend(r.Context(), months)
	if err != nil {
		slog.Error("Failed to fetch billing monthly trend", "error", err)
		respondError(w, "internal_error", "Failed to fetch billing monthly trend", http.StatusInternalServerError)
		return
	}

	cacheControl(w, 3600, false)
	respondJSON(w, trend, http.StatusOK)
}

func (h *BigQueryHandlers) handleBillingModelBreakdown(w http.ResponseWriter, r *http.Request) {
	months, ok := optionalIntParam(r, "months", 12, 1, 36)
	if !ok {
		respondError(w, "invalid_parameter", "months must be between 1 and 36", http.StatusBadRequest)
		return
	}

	breakdown, err := h.bqClient.GetBillingModelBreakdown(r.Context(), months)
	if err != nil {
		slog.Error("Failed to fetch billing model breakdown", "error", err)
		respondError(w, "internal_error", "Failed to fetch billing model breakdown", http.StatusInternalServerError)
		return
	}

	cacheControl(w, 3600, false)
	respondJSON(w, breakdown, http.StatusOK)
}

func (h *BigQueryHandlers) handleUsageDistribution(w http.ResponseWriter, r *http.Request) {
	month, ok := optionalMonthParam(r, "month")
	if !ok {
		respondError(w, "invalid_parameter", "month must be in YYYY-MM format", http.StatusBadRequest)
		return
	}

	budgetCredits := h.resolveBudgetCredits(r.Context())

	distribution, err := h.bqClient.GetUsageDistribution(r.Context(), month, budgetCredits)
	if err != nil {
		slog.Error("Failed to fetch usage distribution", "error", err)
		respondError(w, "internal_error", "Failed to fetch usage distribution", http.StatusInternalServerError)
		return
	}

	// Seat count comes from the background-collected GitHub metrics, not BigQuery.
	// Copy the struct to avoid mutating the cached pointer (shared across requests).
	var seats int64
	if h.activeSeatsGetter != nil {
		seats = h.activeSeatsGetter()
	}

	result := *distribution
	result.TotalLicensedSeats = seats

	cacheControl(w, 3600, false)
	respondJSON(w, &result, http.StatusOK)
}

// resolveBudgetCredits fetches the enterprise per-user $ budget and converts it to
// AI credits (1 credit = $0.01). Falls back to defaultPerUserBudgetCredits if the
// budget client is unavailable or errors, so the distribution endpoint keeps working.
func (h *BigQueryHandlers) resolveBudgetCredits(ctx context.Context) float64 {
	if h.budgetClient == nil {
		return defaultPerUserBudgetCredits
	}
	budget, err := h.budgetClient.getGlobalBudget(ctx)
	if err != nil || budget == nil || budget.PerUserBudget <= 0 {
		slog.Warn("Falling back to default per-user budget for usage distribution", "error", err)
		return defaultPerUserBudgetCredits
	}
	return budget.PerUserBudget / usdPerAICredit
}

func (h *BigQueryHandlers) handleDailySummary(w http.ResponseWriter, r *http.Request) {
	summary, err := h.bqClient.GetDailySummary(r.Context())
	if err != nil {
		slog.Error("Failed to fetch daily summary", "error", err)
		respondError(w, "internal_error", "Failed to fetch daily summary", http.StatusInternalServerError)
		return
	}
	if summary == nil {
		// 204 must not carry a body (Go's http package rejects it, and the
		// frontend's response.json() then throws on the empty body). Return
		// 200 with a literal JSON null instead — this is what fetchNullable
		// on the frontend expects for "no data yet".
		//
		// Use a short cache TTL (rather than none) so early-morning requests
		// before the day's data lands don't bypass the cache entirely and
		// hammer BigQuery on every request until data appears.
		cacheControl(w, 300, false)
		respondJSON(w, nil, http.StatusOK)
		return
	}

	cacheControl(w, 3600, false)
	respondJSON(w, summary, http.StatusOK)
}
