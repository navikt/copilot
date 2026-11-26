package main

import (
	bqstore "github.com/navikt/copilot/apps/copilot-api/internal/bigquery"
	"log/slog"
	"net/http"
)

func (h *BigQueryHandlers) handleEditorUsage(w http.ResponseWriter, r *http.Request) {
	rows, err := h.bqClient.GetEditorUsage(r.Context())
	if err != nil {
		slog.Error("Failed to fetch editor usage", "error", err)
		respondError(w, "internal_error", "Failed to fetch editor usage", http.StatusInternalServerError)
		return
	}
	cacheControl(w, 3600, false)
	if rows == nil {
		rows = []bqstore.EditorUsage{}
	}
	respondJSON(w, rows, http.StatusOK)
}
