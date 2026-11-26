package copilot

import (
	"log/slog"
	"math"
	"net/http"
	"sort"
	"strconv"

	"github.com/navikt/copilot/apps/copilot-api/internal/api/httpx"
	bqstore "github.com/navikt/copilot/apps/copilot-api/internal/bigquery"
	"github.com/navikt/copilot/apps/copilot-api/internal/problem"

	"github.com/navikt/copilot/apps/copilot-api/internal/budget"
)

// usdPerAICredit is GitHub Copilot's billing conversion rate: 1 AI credit = $0.01 USD.
// Source: apps/my-copilot/src/lib/model-pricing.ts (auto-generated from GitHub docs).
const usdPerAICredit = 0.01

// defaultPerUserBudgetCredits is used as a fallback when the budget client is
// unavailable or errors, so the usage-distribution endpoint degrades gracefully
// instead of failing outright. As of 2026-07 this matches the $400/user/month
// enterprise default (400 / 0.01 = 40,000 credits).
const defaultPerUserBudgetCredits = 40000

// Handlers serves the BigQuery-backed /api/v1/copilot/ endpoints.
type Handlers struct {
	bqClient     bqstore.BigQueryQuerier
	budgetClient budget.GlobalBudgetGetter
	seatsGetter  func() int64
}

// Deps are the dependencies of Handlers. Budget and Seats are optional:
// leave Budget a nil interface (never a typed nil) when there is no budget
// client.
type Deps struct {
	BQ     bqstore.BigQueryQuerier
	Budget budget.GlobalBudgetGetter
	// Seats resolves the current GitHub Copilot seat count for
	// HandleUsageDistribution.
	Seats func() int64
}

// New returns the handlers for the given dependencies.
func New(d Deps) *Handlers {
	return &Handlers{bqClient: d.BQ, budgetClient: d.Budget, seatsGetter: d.Seats}
}

// setSeatsGetter overrides how handleUsageDistribution resolves the
// current GitHub Copilot seat count. Primarily used by tests to avoid
// depending on the metricsCollector global singleton.
func (h *Handlers) setSeatsGetter(getter func() int64) {
	h.seatsGetter = getter
}

// HandleDailyMetrics handles GET /api/v1/copilot/usage/metrics?days=N
// Cache: 1 hour (metrics are aggregated daily)
func (h *Handlers) HandleDailyMetrics(w http.ResponseWriter, r *http.Request) {
	if !httpx.RequireMethod(w, r, http.MethodGet) {
		return
	}

	var days *int
	if daysParam := r.URL.Query().Get("days"); daysParam != "" {
		d, err := strconv.Atoi(daysParam)
		if err != nil || d < 1 || d > 365 {
			problem.Respond(w, "invalid_parameter", "days must be between 1 and 365", http.StatusBadRequest)
			return
		}
		days = &d
	}

	metrics, err := h.bqClient.GetDailyMetrics(r.Context(), days)
	if err != nil {
		slog.Error("Failed to fetch daily metrics", "error", err)
		problem.Respond(w, "internal_error", "Failed to fetch daily metrics", http.StatusInternalServerError)
		return
	}

	httpx.CacheControl(w, 3600, false) // 1 hour, public
	httpx.RespondJSON(w, metrics, http.StatusOK)
}

// HandleAdoptionSummary handles GET /api/v1/copilot/adoption/summary
// Cache: 1 hour (aggregated metrics)
func (h *Handlers) HandleAdoptionSummary(w http.ResponseWriter, r *http.Request) {
	if !httpx.RequireMethod(w, r, http.MethodGet) {
		return
	}

	summary, err := h.bqClient.GetAdoptionSummary(r.Context())
	if err != nil {
		slog.Error("Failed to fetch adoption summary", "error", err)
		problem.Respond(w, "internal_error", "Failed to fetch adoption summary", http.StatusInternalServerError)
		return
	}

	if summary == nil {
		httpx.CacheControl(w, 3600, false)
		httpx.RespondJSON(w, map[string]interface{}{}, http.StatusOK)
		return
	}

	httpx.CacheControl(w, 3600, false)
	httpx.RespondJSON(w, summary, http.StatusOK)
}

// HandleTeamAdoption handles GET /api/v1/copilot/adoption/teams
// Cache: 1 hour (aggregated team metrics)
func (h *Handlers) HandleTeamAdoption(w http.ResponseWriter, r *http.Request) {
	if !httpx.RequireMethod(w, r, http.MethodGet) {
		return
	}

	teams, err := h.bqClient.GetTeamAdoption(r.Context())
	if err != nil {
		slog.Error("Failed to fetch team adoption", "error", err)
		problem.Respond(w, "internal_error", "Failed to fetch team adoption", http.StatusInternalServerError)
		return
	}

	httpx.CacheControl(w, 3600, false)
	httpx.RespondJSON(w, teamAdoptionOverview(teams), http.StatusOK)
}

// TeamAdoptionRow is a display-ready team row with rates as whole percent.
type TeamAdoptionRow struct {
	TeamSlug                string   `json:"team_slug"`
	TeamName                string   `json:"team_name"`
	ActiveRepos             int64    `json:"active_repos"`
	RecentlyActiveRepos     int64    `json:"recently_active_repos"`
	ReposWithCustomizations int64    `json:"repos_with_customizations"`
	AdoptionPct             int64    `json:"adoption_pct"`
	AdoptionActivePct       *int64   `json:"adoption_active_pct"`
	Commits30d              int64    `json:"commits_30d,omitempty"`
	OpenIssuesCount         int64    `json:"open_issues_count,omitempty"`
	PRsMerged30d            int64    `json:"prs_merged_30d,omitempty"`
	PRsReviewedByCopilot30d int64    `json:"prs_reviewed_by_copilot_30d,omitempty"`
	AvgMedianMergeMins30d   *float64 `json:"avg_median_minutes_to_merge_30d,omitempty"`
}

// TeamAdoptionOverview lists teams with at least minTeamContributors active
// repos. The summary counts only those teams, so hidden teams cannot be
// recovered by subtracting visible rows from a total.
type TeamAdoptionOverview struct {
	Teams             []TeamAdoptionRow `json:"teams"`
	TotalTeams        int               `json:"total_teams"`
	TeamsWithAdoption int               `json:"teams_with_adoption"`
	AdoptionPct       int64             `json:"adoption_pct"`
	SmallTeams        int               `json:"small_teams"`
}

func pct(rate float64) int64 { return int64(math.Round(rate * 100)) }

func teamAdoptionOverview(teams []bqstore.TeamAdoption) TeamAdoptionOverview {
	out := TeamAdoptionOverview{Teams: []TeamAdoptionRow{}}
	for _, t := range teams {
		if t.ActiveRepos <= 0 {
			continue
		}
		if t.ActiveRepos < bqstore.MinTeamContributors {
			out.SmallTeams++
			continue
		}
		row := TeamAdoptionRow{
			TeamSlug:                t.TeamSlug,
			TeamName:                t.TeamName,
			ActiveRepos:             t.ActiveRepos,
			RecentlyActiveRepos:     t.RecentlyActiveRepos,
			ReposWithCustomizations: t.ReposWithCustomizations,
			AdoptionPct:             pct(t.AdoptionRate),
			Commits30d:              t.Commits30d,
			OpenIssuesCount:         t.OpenIssuesCount,
		}
		if t.PRsMerged30d >= bqstore.MinTeamContributors {
			row.PRsMerged30d = t.PRsMerged30d
			row.PRsReviewedByCopilot30d = t.PRsReviewedByCopilot30d
			row.AvgMedianMergeMins30d = t.AvgMedianMergeMins30d
		}
		if t.RecentlyActiveRepos > 0 {
			active := pct(t.AdoptionRateActiveOnly)
			row.AdoptionActivePct = &active
		}
		if t.ReposWithCustomizations > 0 {
			out.TeamsWithAdoption++
		}
		out.Teams = append(out.Teams, row)
	}
	sort.SliceStable(out.Teams, func(i, j int) bool {
		return out.Teams[i].ReposWithCustomizations > out.Teams[j].ReposWithCustomizations
	})
	out.TotalTeams = len(out.Teams)
	if out.TotalTeams > 0 {
		out.AdoptionPct = pct(float64(out.TeamsWithAdoption) / float64(out.TotalTeams))
	}
	return out
}

// HandleCustomizationDetails handles GET /api/v1/copilot/customizations/details
// Cache: 1 hour (aggregated customization data)
func (h *Handlers) HandleCustomizationDetails(w http.ResponseWriter, r *http.Request) {
	if !httpx.RequireMethod(w, r, http.MethodGet) {
		return
	}

	details, err := h.bqClient.GetCustomizationDetails(r.Context())
	if err != nil {
		slog.Error("Failed to fetch customization details", "error", err)
		problem.Respond(w, "internal_error", "Failed to fetch customization details", http.StatusInternalServerError)
		return
	}

	httpx.CacheControl(w, 3600, false)
	httpx.RespondJSON(w, details, http.StatusOK)
}

// HandleCustomizationUsage handles GET /api/v1/copilot/customizations/usage
// Cache: 1 hour (aggregated customization usage)
func (h *Handlers) HandleCustomizationUsage(w http.ResponseWriter, r *http.Request) {
	if !httpx.RequireMethod(w, r, http.MethodGet) {
		return
	}

	usage, err := h.bqClient.GetCustomizationUsage(r.Context())
	if err != nil {
		slog.Error("Failed to fetch customization usage", "error", err)
		problem.Respond(w, "internal_error", "Failed to fetch customization usage", http.StatusInternalServerError)
		return
	}

	httpx.CacheControl(w, 3600, false)
	httpx.RespondJSON(w, usage, http.StatusOK)
}

// HandleLanguageAdoption handles GET /api/v1/copilot/adoption/languages
// Cache: 1 hour (language adoption metrics)
func (h *Handlers) HandleLanguageAdoption(w http.ResponseWriter, r *http.Request) {
	if !httpx.RequireMethod(w, r, http.MethodGet) {
		return
	}

	langs, err := h.bqClient.GetLanguageAdoption(r.Context())
	if err != nil {
		slog.Error("Failed to fetch language adoption", "error", err)
		problem.Respond(w, "internal_error", "Failed to fetch language adoption", http.StatusInternalServerError)
		return
	}

	httpx.CacheControl(w, 3600, false)
	httpx.RespondJSON(w, langs, http.StatusOK)
}

// HandleAdoptionStaleness handles GET /api/v1/copilot/adoption/staleness
// Cache: 1 hour (staleness data updated daily)
func (h *Handlers) HandleAdoptionStaleness(w http.ResponseWriter, r *http.Request) {
	if !httpx.RequireMethod(w, r, http.MethodGet) {
		return
	}

	files, err := h.bqClient.GetStalenessData(r.Context())
	if err != nil {
		slog.Error("Failed to fetch staleness data", "error", err)
		problem.Respond(w, "internal_error", "Failed to fetch staleness data", http.StatusInternalServerError)
		return
	}

	var totalInstances, inSyncCount, outOfSyncCount int64
	for _, f := range files {
		totalInstances += f.TotalRepos
		inSyncCount += f.InSyncRepos
		outOfSyncCount += f.OutOfSyncRepos
	}

	var syncRate float64
	if totalInstances > 0 {
		syncRate = float64(inSyncCount) / float64(totalInstances)
	}

	summary := bqstore.StalenessSummary{
		TotalFiles:         int64(len(files)),
		TotalFileInstances: totalInstances,
		InSyncCount:        inSyncCount,
		OutOfSyncCount:     outOfSyncCount,
		SyncRate:           syncRate,
		Files:              files,
	}

	httpx.CacheControl(w, 3600, false)
	httpx.RespondJSON(w, summary, http.StatusOK)
}

// HandleNavPilotTelemetry handles GET /api/v1/copilot/mimir/telemetry
// Cache: 1 hour (aggregated daily Mimir telemetry)
func (h *Handlers) HandleNavPilotTelemetry(w http.ResponseWriter, r *http.Request) {
	if !httpx.RequireMethod(w, r, http.MethodGet) {
		return
	}

	telemetry, err := h.bqClient.GetNavPilotTelemetry(r.Context())
	if err != nil {
		slog.Error("Failed to fetch nav-pilot telemetry", "error", err)
		problem.Respond(w, "internal_error", "Failed to fetch nav-pilot telemetry", http.StatusInternalServerError)
		return
	}
	if telemetry == nil {
		telemetry = bqstore.AggregateNavPilotTelemetry(nil)
	}

	httpx.CacheControl(w, 3600, false)
	httpx.RespondJSON(w, telemetry, http.StatusOK)
}
