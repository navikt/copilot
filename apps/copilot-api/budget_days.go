package main

import (
	"log/slog"
	"net/http"
)

// The route must sit behind requireAdmin.
func (h *BigQueryHandlers) handleAdminBudgetDays(w http.ResponseWriter, r *http.Request) {
	days, err := h.bqClient.GetBudgetDays(r.Context())
	if err != nil {
		slog.Error("Failed to fetch budget days", "error", err)
		respondError(w, "internal_error", "Failed to fetch budget days", http.StatusInternalServerError)
		return
	}
	cacheControl(w, 3600, false)
	respondJSON(w, days, http.StatusOK)
}
