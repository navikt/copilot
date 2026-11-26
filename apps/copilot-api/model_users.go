package main

import (
	"log/slog"
	"net/http"
)

func (h *BigQueryHandlers) handleModelFamilyUsers(w http.ResponseWriter, r *http.Request) {
	rows, err := h.bqClient.GetModelFamilyUsers(r.Context())
	if err != nil {
		slog.Error("Failed to fetch model family users", "error", err)
		respondError(w, "internal_error", "Failed to fetch model family users", http.StatusInternalServerError)
		return
	}
	cacheControl(w, 3600, false)
	respondJSON(w, rows, http.StatusOK)
}
