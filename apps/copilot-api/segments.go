package main

import (
	"log/slog"
	"net/http"
)

// handleUserSegments handles GET /api/v1/copilot/usage/segments
func (h *BigQueryHandlers) handleUserSegments(w http.ResponseWriter, r *http.Request) {
	segments, err := h.bqClient.GetUserSegments(r.Context())
	if err != nil {
		slog.Error("Failed to fetch user segments", "error", err)
		respondError(w, "internal_error", "Failed to fetch user segments", http.StatusInternalServerError)
		return
	}
	cacheControl(w, 3600, false)
	respondJSON(w, segments, http.StatusOK)
}
