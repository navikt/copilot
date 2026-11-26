package main

import (
	"log/slog"
	"net/http"
)

func (h *BigQueryHandlers) handleSpendBands(w http.ResponseWriter, r *http.Request) {
	bands, err := h.bqClient.GetSpendBands(r.Context())
	if err != nil {
		slog.Error("Failed to fetch spend bands", "error", err)
		respondError(w, "internal_error", "Failed to fetch spend bands", http.StatusInternalServerError)
		return
	}
	cacheControl(w, 3600, false)
	respondJSON(w, bands, http.StatusOK)
}
