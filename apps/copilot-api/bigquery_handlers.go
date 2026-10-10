package main

import (
	"log/slog"
	"math"
	"net/http"
	"sort"
	"strconv"
)

// BigQueryHandlers wraps handlers that use BigQuery
type BigQueryHandlers struct {
	bqClient     BigQueryQuerier
	budgetClient globalBudgetGetter
	seatsGetter  func() int64
}

func newBigQueryHandlers(bqClient BigQueryQuerier) *BigQueryHandlers {
	return &BigQueryHandlers{
		bqClient: bqClient,
		// Defaults to the real MetricsCollector singleton; tests can override
		// this via setSeatsGetter to avoid depending on global state.
		seatsGetter: func() int64 {
			metricsCollector.mu.RLock()
			defer metricsCollector.mu.RUnlock()
			return metricsCollector.githubSeatsTotal
		},
	}
}

// setBudgetClient wires in the enterprise budget lookup so usage-distribution
// histograms can scale to the actual per-user $ budget instead of a hardcoded
// credit ceiling. Optional — call sites may leave this unset.
func (h *BigQueryHandlers) setBudgetClient(budgetClient globalBudgetGetter) {
	h.budgetClient = budgetClient
}

// setSeatsGetter overrides how handleUsageDistribution resolves the
// current GitHub Copilot seat count. Primarily used by tests to avoid
// depending on the metricsCollector global singleton.
func (h *BigQueryHandlers) setSeatsGetter(getter func() int64) {
	h.seatsGetter = getter
}

func requireMethod(w http.ResponseWriter, r *http.Request, method string) bool {
	if r.Method == method {
		return true
	}
	respondError(w, "method_not_allowed", "Only GET is allowed", http.StatusMethodNotAllowed)
	return false
}

// handleDailyMetrics handles GET /api/v1/copilot/usage/metrics?days=N
// Cache: 1 hour (metrics are aggregated daily)
func (h *BigQueryHandlers) handleDailyMetrics(w http.ResponseWriter, r *http.Request) {
	if !requireMethod(w, r, http.MethodGet) {
		return
	}

	var days *int
	if daysParam := r.URL.Query().Get("days"); daysParam != "" {
		d, err := strconv.Atoi(daysParam)
		if err != nil || d < 1 || d > 365 {
			respondError(w, "invalid_parameter", "days must be between 1 and 365", http.StatusBadRequest)
			return
		}
		days = &d
	}

	metrics, err := h.bqClient.GetDailyMetrics(r.Context(), days)
	if err != nil {
		slog.Error("Failed to fetch daily metrics", "error", err)
		respondError(w, "internal_error", "Failed to fetch daily metrics", http.StatusInternalServerError)
		return
	}

	cacheControl(w, 3600, false) // 1 hour, public
	respondJSON(w, metrics, http.StatusOK)
}

// handleAdoptionSummary handles GET /api/v1/copilot/adoption/summary
// Cache: 1 hour (aggregated metrics)
func (h *BigQueryHandlers) handleAdoptionSummary(w http.ResponseWriter, r *http.Request) {
	if !requireMethod(w, r, http.MethodGet) {
		return
	}

	summary, err := h.bqClient.GetAdoptionSummary(r.Context())
	if err != nil {
		slog.Error("Failed to fetch adoption summary", "error", err)
		respondError(w, "internal_error", "Failed to fetch adoption summary", http.StatusInternalServerError)
		return
	}

	if summary == nil {
		cacheControl(w, 3600, false)
		respondJSON(w, map[string]interface{}{}, http.StatusOK)
		return
	}

	cacheControl(w, 3600, false)
	respondJSON(w, summary, http.StatusOK)
}

// handleTeamAdoption handles GET /api/v1/copilot/adoption/teams
// Cache: 1 hour (aggregated team metrics)
func (h *BigQueryHandlers) handleTeamAdoption(w http.ResponseWriter, r *http.Request) {
	if !requireMethod(w, r, http.MethodGet) {
		return
	}

	teams, err := h.bqClient.GetTeamAdoption(r.Context())
	if err != nil {
		slog.Error("Failed to fetch team adoption", "error", err)
		respondError(w, "internal_error", "Failed to fetch team adoption", http.StatusInternalServerError)
		return
	}

	cacheControl(w, 3600, false)
	respondJSON(w, teamAdoptionOverview(teams), http.StatusOK)
}

// TeamAdoptionRow is a display-ready team row with rates as whole percent.
type TeamAdoptionRow struct {
	TeamSlug                string `json:"team_slug"`
	TeamName                string `json:"team_name"`
	ActiveRepos             int64  `json:"active_repos"`
	RecentlyActiveRepos     int64  `json:"recently_active_repos"`
	ReposWithCustomizations int64  `json:"repos_with_customizations"`
	AdoptionPct             int64  `json:"adoption_pct"`
	AdoptionActivePct       *int64 `json:"adoption_active_pct"`
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

func teamAdoptionOverview(teams []TeamAdoption) TeamAdoptionOverview {
	out := TeamAdoptionOverview{Teams: []TeamAdoptionRow{}}
	for _, t := range teams {
		if t.ActiveRepos <= 0 {
			continue
		}
		if t.ActiveRepos < minTeamContributors {
			out.SmallTeams++
			continue
		}
		row := TeamAdoptionRow{t.TeamSlug, t.TeamName, t.ActiveRepos, t.RecentlyActiveRepos, t.ReposWithCustomizations, pct(t.AdoptionRate), nil}
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

// handleCustomizationDetails handles GET /api/v1/copilot/customizations/details
// Cache: 1 hour (aggregated customization data)
func (h *BigQueryHandlers) handleCustomizationDetails(w http.ResponseWriter, r *http.Request) {
	if !requireMethod(w, r, http.MethodGet) {
		return
	}

	details, err := h.bqClient.GetCustomizationDetails(r.Context())
	if err != nil {
		slog.Error("Failed to fetch customization details", "error", err)
		respondError(w, "internal_error", "Failed to fetch customization details", http.StatusInternalServerError)
		return
	}

	cacheControl(w, 3600, false)
	respondJSON(w, details, http.StatusOK)
}

// handleCustomizationUsage handles GET /api/v1/copilot/customizations/usage
// Cache: 1 hour (aggregated customization usage)
func (h *BigQueryHandlers) handleCustomizationUsage(w http.ResponseWriter, r *http.Request) {
	if !requireMethod(w, r, http.MethodGet) {
		return
	}

	usage, err := h.bqClient.GetCustomizationUsage(r.Context())
	if err != nil {
		slog.Error("Failed to fetch customization usage", "error", err)
		respondError(w, "internal_error", "Failed to fetch customization usage", http.StatusInternalServerError)
		return
	}

	cacheControl(w, 3600, false)
	respondJSON(w, usage, http.StatusOK)
}

// handleLanguageAdoption handles GET /api/v1/copilot/adoption/languages
// Cache: 1 hour (language adoption metrics)
func (h *BigQueryHandlers) handleLanguageAdoption(w http.ResponseWriter, r *http.Request) {
	if !requireMethod(w, r, http.MethodGet) {
		return
	}

	langs, err := h.bqClient.GetLanguageAdoption(r.Context())
	if err != nil {
		slog.Error("Failed to fetch language adoption", "error", err)
		respondError(w, "internal_error", "Failed to fetch language adoption", http.StatusInternalServerError)
		return
	}

	cacheControl(w, 3600, false)
	respondJSON(w, langs, http.StatusOK)
}

// handleAdoptionStaleness handles GET /api/v1/copilot/adoption/staleness
// Cache: 1 hour (staleness data updated daily)
func (h *BigQueryHandlers) handleAdoptionStaleness(w http.ResponseWriter, r *http.Request) {
	if !requireMethod(w, r, http.MethodGet) {
		return
	}

	files, err := h.bqClient.GetStalenessData(r.Context())
	if err != nil {
		slog.Error("Failed to fetch staleness data", "error", err)
		respondError(w, "internal_error", "Failed to fetch staleness data", http.StatusInternalServerError)
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

	summary := StalenessSummary{
		TotalFiles:         int64(len(files)),
		TotalFileInstances: totalInstances,
		InSyncCount:        inSyncCount,
		OutOfSyncCount:     outOfSyncCount,
		SyncRate:           syncRate,
		Files:              files,
	}

	cacheControl(w, 3600, false)
	respondJSON(w, summary, http.StatusOK)
}
