package main

import (
	bqstore "github.com/navikt/copilot/apps/copilot-api/internal/bigquery"
	"log/slog"
	"net/http"
)

func (h *BigQueryHandlers) handleUsageQuality(w http.ResponseWriter, r *http.Request) {
	rows, err := h.bqClient.GetUsageQuality(r.Context())
	if err != nil {
		slog.Error("Failed to fetch usage quality", "error", err)
		respondError(w, "internal_error", "Failed to fetch usage quality", http.StatusInternalServerError)
		return
	}
	if rows == nil {
		rows = []bqstore.UsageQualityDay{}
	}
	cacheControl(w, 3600, false)
	respondJSON(w, rows, http.StatusOK)
}
