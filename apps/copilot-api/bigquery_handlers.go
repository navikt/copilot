package main

import (
	"net/http"
	"strconv"
)

// BigQueryHandlers wraps handlers that use BigQuery
type BigQueryHandlers struct {
	bqClient BigQueryQuerier
}

func newBigQueryHandlers(bqClient BigQueryQuerier) *BigQueryHandlers {
	return &BigQueryHandlers{
		bqClient: bqClient,
	}
}

func requireMethod(w http.ResponseWriter, r *http.Request, method string) bool {
	if r.Method == method {
		return true
	}
	respondError(w, "method_not_allowed", "Only GET is allowed", http.StatusMethodNotAllowed)
	return false
}

// handleDailyMetrics handles GET /api/v1/copilot/usage/metrics?days=N
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
		respondError(w, "internal_error", "Failed to fetch daily metrics", http.StatusInternalServerError)
		return
	}

	respondJSON(w, metrics, http.StatusOK)
}

// handleAdoptionSummary handles GET /api/v1/copilot/adoption/summary
func (h *BigQueryHandlers) handleAdoptionSummary(w http.ResponseWriter, r *http.Request) {
	if !requireMethod(w, r, http.MethodGet) {
		return
	}

	summary, err := h.bqClient.GetAdoptionSummary(r.Context())
	if err != nil {
		respondError(w, "internal_error", "Failed to fetch adoption summary", http.StatusInternalServerError)
		return
	}

	if summary == nil {
		respondJSON(w, map[string]interface{}{}, http.StatusOK)
		return
	}

	respondJSON(w, summary, http.StatusOK)
}

// handleTeamAdoption handles GET /api/v1/copilot/adoption/teams
func (h *BigQueryHandlers) handleTeamAdoption(w http.ResponseWriter, r *http.Request) {
	if !requireMethod(w, r, http.MethodGet) {
		return
	}

	teams, err := h.bqClient.GetTeamAdoption(r.Context())
	if err != nil {
		respondError(w, "internal_error", "Failed to fetch team adoption", http.StatusInternalServerError)
		return
	}

	respondJSON(w, teams, http.StatusOK)
}

// handleCustomizationDetails handles GET /api/v1/copilot/customizations/details
func (h *BigQueryHandlers) handleCustomizationDetails(w http.ResponseWriter, r *http.Request) {
	if !requireMethod(w, r, http.MethodGet) {
		return
	}

	details, err := h.bqClient.GetCustomizationDetails(r.Context())
	if err != nil {
		respondError(w, "internal_error", "Failed to fetch customization details", http.StatusInternalServerError)
		return
	}

	respondJSON(w, details, http.StatusOK)
}

// handleCustomizationUsage handles GET /api/v1/copilot/customizations/usage
func (h *BigQueryHandlers) handleCustomizationUsage(w http.ResponseWriter, r *http.Request) {
	if !requireMethod(w, r, http.MethodGet) {
		return
	}

	usage, err := h.bqClient.GetCustomizationUsage(r.Context())
	if err != nil {
		respondError(w, "internal_error", "Failed to fetch customization usage", http.StatusInternalServerError)
		return
	}

	respondJSON(w, usage, http.StatusOK)
}

// handleLanguageAdoption handles GET /api/v1/copilot/adoption/languages
func (h *BigQueryHandlers) handleLanguageAdoption(w http.ResponseWriter, r *http.Request) {
	if !requireMethod(w, r, http.MethodGet) {
		return
	}

	langs, err := h.bqClient.GetLanguageAdoption(r.Context())
	if err != nil {
		respondError(w, "internal_error", "Failed to fetch language adoption", http.StatusInternalServerError)
		return
	}

	respondJSON(w, langs, http.StatusOK)
}

// handleAdoptionStaleness handles GET /api/v1/copilot/adoption/staleness
func (h *BigQueryHandlers) handleAdoptionStaleness(w http.ResponseWriter, r *http.Request) {
	if !requireMethod(w, r, http.MethodGet) {
		return
	}

	files, err := h.bqClient.GetStalenessData(r.Context())
	if err != nil {
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

	respondJSON(w, summary, http.StatusOK)
}
