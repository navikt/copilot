package main

import (
	"log/slog"
	"net/http"
)

// handleCreditsPerUserMonthly handles GET /api/v1/copilot/usage/credits-per-user
func (h *BigQueryHandlers) handleCreditsPerUserMonthly(w http.ResponseWriter, r *http.Request) {
	months, err := h.bqClient.GetCreditsPerUserMonthly(r.Context())
	if err != nil {
		slog.Error("Failed to fetch credits per user", "error", err)
		respondError(w, "internal_error", "Failed to fetch credits per user", http.StatusInternalServerError)
		return
	}
	cacheControl(w, 3600, false)
	respondJSON(w, months, http.StatusOK)
}

// handleCopilotPRsMonthly handles GET /api/v1/copilot/usage/copilot-prs
func (h *BigQueryHandlers) handleCopilotPRsMonthly(w http.ResponseWriter, r *http.Request) {
	months, err := h.bqClient.GetCopilotPRsMonthly(r.Context())
	if err != nil {
		slog.Error("Failed to fetch Copilot PRs per month", "error", err)
		respondError(w, "internal_error", "Failed to fetch Copilot PRs per month", http.StatusInternalServerError)
		return
	}
	cacheControl(w, 3600, false)
	respondJSON(w, months, http.StatusOK)
}

// handleCohortRetention handles GET /api/v1/copilot/usage/cohort-retention
func (h *BigQueryHandlers) handleCohortRetention(w http.ResponseWriter, r *http.Request) {
	cohorts, err := h.bqClient.GetCohortRetention(r.Context())
	if err != nil {
		slog.Error("Failed to fetch cohort retention", "error", err)
		respondError(w, "internal_error", "Failed to fetch cohort retention", http.StatusInternalServerError)
		return
	}
	cacheControl(w, 3600, false)
	respondJSON(w, cohorts, http.StatusOK)
}
