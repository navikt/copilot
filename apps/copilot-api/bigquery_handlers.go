package main

import (
	"log/slog"
	"net/http"
	"strconv"
)

// BigQueryHandlers wraps handlers that use BigQuery
type BigQueryHandlers struct {
	bqClient          BigQueryQuerier
	budgetClient      globalBudgetGetter
	activeSeatsGetter func() int64
}

func newBigQueryHandlers(bqClient BigQueryQuerier) *BigQueryHandlers {
	return &BigQueryHandlers{
		bqClient: bqClient,
		// Defaults to the real MetricsCollector singleton; tests can override
		// this via setActiveSeatsGetter to avoid depending on global state.
		activeSeatsGetter: func() int64 {
			metricsCollector.mu.RLock()
			defer metricsCollector.mu.RUnlock()
			return metricsCollector.githubSeatsActive
		},
	}
}

// setBudgetClient wires in the enterprise budget lookup so usage-distribution
// histograms can scale to the actual per-user $ budget instead of a hardcoded
// credit ceiling. Optional — call sites may leave this unset.
func (h *BigQueryHandlers) setBudgetClient(budgetClient globalBudgetGetter) {
	h.budgetClient = budgetClient
}

// setActiveSeatsGetter overrides how handleUsageDistribution resolves the
// current active GitHub Copilot seat count. Primarily used by tests to avoid
// depending on the metricsCollector global singleton.
func (h *BigQueryHandlers) setActiveSeatsGetter(getter func() int64) {
	h.activeSeatsGetter = getter
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
	respondJSON(w, teams, http.StatusOK)
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
