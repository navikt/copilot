package main

import (
	"log/slog"
	"net/http"
)

// handleCostAnomalies must sit behind requireAdmin: costs are for admins only.
func (h *BigQueryHandlers) handleCostAnomalies(w http.ResponseWriter, r *http.Request) {
	anomalies, err := h.bqClient.GetCostAnomalies(r.Context())
	if err != nil {
		slog.Error("Failed to compute cost anomalies", "error", err)
		respondError(w, "internal_error", "Failed to compute cost anomalies", http.StatusInternalServerError)
		return
	}
	cacheControl(w, 900, false)
	respondJSON(w, anomalies, http.StatusOK)
}
