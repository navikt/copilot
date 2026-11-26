package main

import (
	"log/slog"
	"net/http"
)

// The route must sit behind requireAdmin.
func (h *BigQueryHandlers) handleAdminActiveUsers(w http.ResponseWriter, r *http.Request) {
	rows, err := h.bqClient.GetActiveUsersMonthly(r.Context())
	if err != nil {
		slog.Error("Failed to fetch active users", "error", err)
		respondError(w, "internal_error", "Failed to fetch active users", http.StatusInternalServerError)
		return
	}
	cacheControl(w, 3600, false)
	respondJSON(w, rows, http.StatusOK)
}

// The route must sit behind requireAdmin.
func (h *BigQueryHandlers) handleAdminHighConsumers(w http.ResponseWriter, r *http.Request) {
	months, err := h.bqClient.GetHighConsumers(r.Context())
	if err != nil {
		slog.Error("Failed to fetch high consumers", "error", err)
		respondError(w, "internal_error", "Failed to fetch high consumers", http.StatusInternalServerError)
		return
	}
	cacheControl(w, 3600, false)
	respondJSON(w, months, http.StatusOK)
}
