package main

import (
	"log/slog"
	"net/http"
)

type EditorUsage struct {
	Editor       string `bigquery:"editor" json:"editor"`
	FirstDay     string `bigquery:"first_day" json:"first_day"`
	LastDay      string `bigquery:"last_day" json:"last_day"`
	Days         int64  `bigquery:"days" json:"days"`
	Generations  *int64 `bigquery:"generations" json:"generations"`
	Acceptances  *int64 `bigquery:"acceptances" json:"acceptances"`
	Interactions *int64 `bigquery:"interactions" json:"interactions"`
}

func (h *BigQueryHandlers) handleEditorUsage(w http.ResponseWriter, r *http.Request) {
	rows, err := h.bqClient.GetEditorUsage(r.Context())
	if err != nil {
		slog.Error("Failed to fetch editor usage", "error", err)
		respondError(w, "internal_error", "Failed to fetch editor usage", http.StatusInternalServerError)
		return
	}
	cacheControl(w, 3600, false)
	if rows == nil {
		rows = []EditorUsage{}
	}
	respondJSON(w, rows, http.StatusOK)
}
