package copilot

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/navikt/copilot/apps/copilot-api/internal/api/httpx"
	bqstore "github.com/navikt/copilot/apps/copilot-api/internal/bigquery"
	"github.com/navikt/copilot/apps/copilot-api/internal/problem"

	"github.com/navikt/copilot/apps/copilot-api/internal/auth"
)

func isValidUsageUsername(username string) bool {
	return username != "" && len(username) <= 39 && !strings.Contains(username, "/") && auth.IsValidGitHubUsername(username)
}

func optionalMonthParam(r *http.Request, name string) (string, bool) {
	value := r.URL.Query().Get(name)
	if value == "" {
		return time.Now().UTC().Format("2006-01"), true
	}
	if !bqstore.IsValidYearMonth(value) {
		return "", false
	}
	return value, true
}

func (h *Handlers) HandleTeamGrossOverview(w http.ResponseWriter, r *http.Request) {
	month, ok := optionalMonthParam(r, "month")
	if !ok {
		problem.Respond(w, "invalid_parameter", "month must be in YYYY-MM format", http.StatusBadRequest)
		return
	}
	usage, err := h.bqClient.GetTeamGrossOverview(r.Context(), month)
	if err != nil {
		slog.Error("Failed to fetch team gross usage")
		problem.Respond(w, "internal_error", "Failed to fetch team gross usage", http.StatusInternalServerError)
		return
	}
	prevGross, _ := h.previousTeamMonth(r.Context(), month, false)
	out := *usage
	out.Comparison = bqstore.TeamComparison(month, time.Now(), usage.DaysWithUsage, prevGross, false, false)
	if out.Comparison == bqstore.ComparisonOK {
		out.Teams = bqstore.WithChanges(usage.Teams, prevGross.Teams)
	}
	httpx.CacheControl(w, 3600, false)
	httpx.RespondJSON(w, out, http.StatusOK)
}

func (h *Handlers) HandleTeamNetOverview(w http.ResponseWriter, r *http.Request) {
	month, ok := optionalMonthParam(r, "month")
	if !ok {
		problem.Respond(w, "invalid_parameter", "month must be in YYYY-MM format", http.StatusBadRequest)
		return
	}
	usage, err := h.bqClient.GetTeamNetOverview(r.Context(), month)
	if err != nil {
		if errors.Is(err, bqstore.ErrTeamNetNotReady) {
			httpx.RespondJSON(w, nil, http.StatusOK)
			return
		}
		slog.Error("Failed to fetch team net usage")
		problem.Respond(w, "internal_error", "Failed to fetch team net usage", http.StatusInternalServerError)
		return
	}
	if usage == nil {
		httpx.RespondJSON(w, nil, http.StatusOK)
		return
	}
	out := *usage
	out.Comparison = bqstore.ComparisonIncomplete
	if gross, err := h.bqClient.GetTeamGrossOverview(r.Context(), month); err == nil && gross != nil {
		prevGross, prevNet := h.previousTeamMonth(r.Context(), month, true)
		out.Comparison = bqstore.TeamComparison(month, time.Now(), gross.DaysWithUsage, prevGross, true, prevNet != nil)
		if out.Comparison == bqstore.ComparisonOK {
			out.Teams = bqstore.WithChanges(usage.Teams, prevNet.Teams)
		}
	}
	httpx.CacheControl(w, 3600, false)
	httpx.RespondJSON(w, out, http.StatusOK)
}

var validTeamID = regexp.MustCompile(`^[1-9][0-9]{0,19}$`)

// HandleTeamYearOverview serves one team's months for a year. Same audience
// as team-gross and team-net, but without their under-five threshold: small
// teams' amounts are shown on purpose (owner decision 2026-10-12). No logins
// are returned. team is the GitHub team
// id from team_id in those endpoints, since slugs change when a team is renamed.
func (h *Handlers) HandleTeamYearOverview(w http.ResponseWriter, r *http.Request) {
	team := r.URL.Query().Get("team")
	if !validTeamID.MatchString(team) {
		problem.Respond(w, "invalid_parameter", "team must be a GitHub team id", http.StatusBadRequest)
		return
	}
	firstYear, _ := strconv.Atoi(firstTeamMonth[:4])
	thisYear := time.Now().UTC().Year()
	year, ok := httpx.OptionalIntParam(r, "year", thisYear, firstYear, thisYear)
	if !ok {
		problem.Respond(w, "invalid_parameter", fmt.Sprintf("year must be between %d and %d", firstYear, thisYear), http.StatusBadRequest)
		return
	}
	overview, err := h.bqClient.GetTeamYearOverview(r.Context(), team, year)
	if err != nil {
		slog.Error("Failed to fetch team year usage", "error", err)
		problem.Respond(w, "internal_error", "Failed to fetch team year usage", http.StatusInternalServerError)
		return
	}
	httpx.CacheControl(w, 3600, false)
	httpx.RespondJSON(w, overview, http.StatusOK)
}

// firstTeamMonth is the first month with team data.
const firstTeamMonth = "2026-05"

// previousTeamMonth loads the month before month. Failures only remove the comparison.
func (h *Handlers) previousTeamMonth(ctx context.Context, month string, net bool) (*bqstore.TeamGrossOverview, *bqstore.TeamNetOverview) {
	first, _ := time.Parse("2006-01", month)
	prev := first.AddDate(0, -1, 0).Format("2006-01")
	if prev < firstTeamMonth {
		return nil, nil
	}
	gross, err := h.bqClient.GetTeamGrossOverview(ctx, prev)
	if err != nil {
		slog.Warn("Previous team month unavailable")
		return nil, nil
	}
	if !net {
		return gross, nil
	}
	prevNet, err := h.bqClient.GetTeamNetOverview(ctx, prev)
	if err != nil && !errors.Is(err, bqstore.ErrTeamNetNotReady) {
		slog.Warn("Previous team net month unavailable")
	}
	return gross, prevNet
}

func (h *Handlers) HandleMyTeams(w http.ResponseWriter, r *http.Request) {
	identity, ok := auth.GetResolvedIdentity(r.Context())
	if !ok || identity == nil {
		problem.Respond(w, "unauthorized", "Authentication required", http.StatusUnauthorized)
		return
	}
	teams, err := h.bqClient.GetUserTeams(r.Context(), identity.GitHubUsername)
	if err != nil {
		slog.Error("Failed to fetch caller teams", "error", err)
		problem.Respond(w, "internal_error", "Failed to fetch your teams", http.StatusInternalServerError)
		return
	}
	httpx.CacheControl(w, 300, false)
	httpx.RespondJSON(w, teams, http.StatusOK)
}

func (h *Handlers) HandleUserMetrics(w http.ResponseWriter, r *http.Request) {
	username := r.PathValue("username")
	if !isValidUsageUsername(username) {
		problem.Respond(w, "invalid_parameter", "Invalid GitHub username", http.StatusBadRequest)
		return
	}

	if !auth.RequireOwnership(w, r, username) {
		return
	}

	days, ok := httpx.OptionalIntParam(r, "days", 7, 1, 365)
	if !ok {
		problem.Respond(w, "invalid_parameter", "days must be between 1 and 365", http.StatusBadRequest)
		return
	}

	metrics, err := h.bqClient.GetUserMetrics(r.Context(), username, days)
	if err != nil {
		slog.Error("Failed to fetch user metrics", "error", err)
		problem.Respond(w, "internal_error", "Failed to fetch user metrics", http.StatusInternalServerError)
		return
	}
	if metrics == nil {
		problem.Respond(w, "not_found", "No user metrics found", http.StatusNotFound)
		return
	}

	httpx.CacheControl(w, 300, false)
	httpx.RespondJSON(w, metrics, http.StatusOK)
}

func (h *Handlers) HandleMonthlyTrends(w http.ResponseWriter, r *http.Request) {
	months, ok := httpx.OptionalIntParam(r, "months", 12, 1, 36)
	if !ok {
		problem.Respond(w, "invalid_parameter", "months must be between 1 and 36", http.StatusBadRequest)
		return
	}

	trends, err := h.bqClient.GetMonthlyTrends(r.Context(), months)
	if err != nil {
		slog.Error("Failed to fetch monthly trends", "error", err)
		problem.Respond(w, "internal_error", "Failed to fetch monthly trends", http.StatusInternalServerError)
		return
	}

	httpx.CacheControl(w, 3600, false)
	httpx.RespondJSON(w, trends, http.StatusOK)
}

func (h *Handlers) HandleMonthlyBillingUsage(w http.ResponseWriter, r *http.Request) {
	months, ok := httpx.OptionalIntParam(r, "months", 12, 1, 36)
	if !ok {
		problem.Respond(w, "invalid_parameter", "months must be between 1 and 36", http.StatusBadRequest)
		return
	}

	usage, err := h.bqClient.GetMonthlyBillingUsage(r.Context(), months)
	if err != nil {
		slog.Error("Failed to fetch monthly billing usage", "error", err)
		problem.Respond(w, "internal_error", "Failed to fetch monthly billing usage", http.StatusInternalServerError)
		return
	}

	httpx.CacheControl(w, 3600, false)
	httpx.RespondJSON(w, usage, http.StatusOK)
}

func (h *Handlers) HandleBillingModelDaily(w http.ResponseWriter, r *http.Request) {
	month, ok := optionalMonthParam(r, "month")
	if !ok {
		problem.Respond(w, "invalid_parameter", "month must be in YYYY-MM format", http.StatusBadRequest)
		return
	}

	usage, err := h.bqClient.GetBillingModelDailyCosts(r.Context(), month)
	if err != nil {
		slog.Error("Failed to fetch billing model daily costs", "error", err)
		problem.Respond(w, "internal_error", "Failed to fetch billing model daily costs", http.StatusInternalServerError)
		return
	}
	if usage == nil {
		usage = []bqstore.BillingModelDailyCost{}
	}

	httpx.CacheControl(w, 900, false)
	httpx.RespondJSON(w, usage, http.StatusOK)
}

func (h *Handlers) HandleBillingModelForecast(w http.ResponseWriter, r *http.Request) {
	month, ok := optionalMonthParam(r, "month")
	if !ok {
		problem.Respond(w, "invalid_parameter", "month must be in YYYY-MM format", http.StatusBadRequest)
		return
	}

	forecast, err := h.bqClient.GetBillingModelForecast(r.Context(), month)
	if err != nil {
		slog.Error("Failed to fetch billing model forecast", "error", err)
		problem.Respond(w, "internal_error", "Failed to fetch billing model forecast", http.StatusInternalServerError)
		return
	}

	httpx.CacheControl(w, 900, false)
	httpx.RespondJSON(w, forecast, http.StatusOK)
}

func (h *Handlers) HandleUserWeeklyTrends(w http.ResponseWriter, r *http.Request) {
	username := r.PathValue("username")
	if !isValidUsageUsername(username) {
		problem.Respond(w, "invalid_parameter", "Invalid GitHub username", http.StatusBadRequest)
		return
	}

	if !auth.RequireOwnership(w, r, username) {
		return
	}

	weeks, ok := httpx.OptionalIntParam(r, "weeks", 12, 1, 52)
	if !ok {
		problem.Respond(w, "invalid_parameter", "weeks must be between 1 and 52", http.StatusBadRequest)
		return
	}

	trends, err := h.bqClient.GetUserWeeklyTrends(r.Context(), username, weeks)
	if err != nil {
		slog.Error("Failed to fetch user weekly trends", "error", err)
		problem.Respond(w, "internal_error", "Failed to fetch user weekly trends", http.StatusInternalServerError)
		return
	}

	httpx.CacheControl(w, 300, false)
	httpx.RespondJSON(w, trends, http.StatusOK)
}

func (h *Handlers) HandleUserDailyCredits(w http.ResponseWriter, r *http.Request) {
	username := r.PathValue("username")
	if !isValidUsageUsername(username) {
		problem.Respond(w, "invalid_parameter", "Invalid GitHub username", http.StatusBadRequest)
		return
	}

	if !auth.RequireOwnership(w, r, username) {
		return
	}

	days, ok := httpx.OptionalIntParam(r, "days", 30, 1, 90)
	if !ok {
		problem.Respond(w, "invalid_parameter", "days must be between 1 and 90", http.StatusBadRequest)
		return
	}

	credits, err := h.bqClient.GetUserDailyCredits(r.Context(), username, days)
	if err != nil {
		slog.Error("Failed to fetch user daily credits", "error", err)
		problem.Respond(w, "internal_error", "Failed to fetch user daily credits", http.StatusInternalServerError)
		return
	}

	httpx.CacheControl(w, 300, false)
	httpx.RespondJSON(w, credits, http.StatusOK)
}

func (h *Handlers) HandleAdoptionCohorts(w http.ResponseWriter, r *http.Request) {
	days, ok := httpx.OptionalIntParam(r, "days", 90, 1, 365)
	if !ok {
		problem.Respond(w, "invalid_parameter", "days must be between 1 and 365", http.StatusBadRequest)
		return
	}

	cohorts, err := h.bqClient.GetAdoptionCohorts(r.Context(), days)
	if err != nil {
		slog.Error("Failed to fetch adoption cohorts", "error", err)
		problem.Respond(w, "internal_error", "Failed to fetch adoption cohorts", http.StatusInternalServerError)
		return
	}

	httpx.CacheControl(w, 3600, false)
	// Nav-wide phase counts name no one, so small cells are shown as is (see the
	// small-group rule in segments.go).
	httpx.RespondJSON(w, bqstore.WeeklyCohorts(cohorts), http.StatusOK)
}

func (h *Handlers) HandleBillingMonthlyTrend(w http.ResponseWriter, r *http.Request) {
	months, ok := httpx.OptionalIntParam(r, "months", 12, 1, 36)
	if !ok {
		problem.Respond(w, "invalid_parameter", "months must be between 1 and 36", http.StatusBadRequest)
		return
	}

	trend, err := h.bqClient.GetBillingMonthlyTrend(r.Context(), months)
	if err != nil {
		slog.Error("Failed to fetch billing monthly trend", "error", err)
		problem.Respond(w, "internal_error", "Failed to fetch billing monthly trend", http.StatusInternalServerError)
		return
	}

	httpx.CacheControl(w, 3600, false)
	httpx.RespondJSON(w, trend, http.StatusOK)
}

func (h *Handlers) HandleBillingModelBreakdown(w http.ResponseWriter, r *http.Request) {
	months, ok := httpx.OptionalIntParam(r, "months", 12, 1, 36)
	if !ok {
		problem.Respond(w, "invalid_parameter", "months must be between 1 and 36", http.StatusBadRequest)
		return
	}

	breakdown, err := h.bqClient.GetBillingModelBreakdown(r.Context(), months)
	if err != nil {
		slog.Error("Failed to fetch billing model breakdown", "error", err)
		problem.Respond(w, "internal_error", "Failed to fetch billing model breakdown", http.StatusInternalServerError)
		return
	}

	httpx.CacheControl(w, 3600, false)
	httpx.RespondJSON(w, breakdown, http.StatusOK)
}

func (h *Handlers) HandleUsageDistribution(w http.ResponseWriter, r *http.Request) {
	month, ok := optionalMonthParam(r, "month")
	if !ok {
		problem.Respond(w, "invalid_parameter", "month must be in YYYY-MM format", http.StatusBadRequest)
		return
	}

	budgetCredits := h.resolveBudgetCredits(r.Context())

	distribution, err := h.bqClient.GetUsageDistribution(r.Context(), month, budgetCredits)
	if err != nil {
		slog.Error("Failed to fetch usage distribution", "error", err)
		problem.Respond(w, "internal_error", "Failed to fetch usage distribution", http.StatusInternalServerError)
		return
	}

	// Seat count comes from the background-collected GitHub metrics, not BigQuery.
	// Copy the struct to avoid mutating the cached pointer (shared across requests).
	var seats int64
	if h.seatsGetter != nil {
		seats = h.seatsGetter()
	}

	result := *distribution
	result.TotalLicensedSeats = seats

	httpx.CacheControl(w, 3600, false)
	httpx.RespondJSON(w, &result, http.StatusOK)
}

// resolveBudgetCredits fetches the enterprise per-user $ budget and converts it to
// AI credits (1 credit = $0.01). Falls back to defaultPerUserBudgetCredits if the
// budget client is unavailable or errors, so the distribution endpoint keeps working.
func (h *Handlers) resolveBudgetCredits(ctx context.Context) float64 {
	if h.budgetClient == nil {
		return defaultPerUserBudgetCredits
	}
	budget, err := h.budgetClient.GetGlobalBudget(ctx)
	if err != nil || budget == nil || budget.PerUserBudget <= 0 {
		slog.Warn("Falling back to default per-user budget for usage distribution", "error", err)
		return defaultPerUserBudgetCredits
	}
	return budget.PerUserBudget / usdPerAICredit
}

func (h *Handlers) HandleDailySummary(w http.ResponseWriter, r *http.Request) {
	summary, err := h.bqClient.GetDailySummary(r.Context())
	if err != nil {
		slog.Error("Failed to fetch daily summary", "error", err)
		problem.Respond(w, "internal_error", "Failed to fetch daily summary", http.StatusInternalServerError)
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
		httpx.CacheControl(w, 300, false)
		httpx.RespondJSON(w, nil, http.StatusOK)
		return
	}

	httpx.CacheControl(w, 3600, false)
	httpx.RespondJSON(w, summary, http.StatusOK)
}

// HandleRepositoryUsage handles GET /api/v1/copilot/usage/repositories
// Cache: 1 hour (aggregated per-repository metrics). No query parameters: the
// v_repository_usage view is already all-time-aggregated and privacy-suppressed,
// so there is no trailing window to bound here.
func (h *Handlers) HandleRepositoryUsage(w http.ResponseWriter, r *http.Request) {
	repositories, err := h.bqClient.GetRepositoryUsage(r.Context())
	if err != nil {
		slog.Error("Failed to fetch repository usage", "error", err)
		problem.Respond(w, "internal_error", "Failed to fetch repository usage", http.StatusInternalServerError)
		return
	}

	httpx.CacheControl(w, 3600, false)
	httpx.RespondJSON(w, repositories, http.StatusOK)
}

// HandleClientUsage handles GET /api/v1/copilot/usage/clients
// Cache: 1 hour. Requests and sessions per client, last 28 days.
func (h *Handlers) HandleClientUsage(w http.ResponseWriter, r *http.Request) {
	clients, err := h.bqClient.GetClientUsage(r.Context())
	if err != nil {
		slog.Error("Failed to fetch client usage", "error", err)
		problem.Respond(w, "internal_error", "Failed to fetch client usage", http.StatusInternalServerError)
		return
	}

	httpx.CacheControl(w, 3600, false)
	httpx.RespondJSON(w, clients, http.StatusOK)
}
