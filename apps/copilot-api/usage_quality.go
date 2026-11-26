package main

import (
	"log/slog"
	"net/http"
)

// UsageQualityDay contains independent daily counts, not unique users over a period.
type UsageQualityDay struct {
	Day                      string `bigquery:"day" json:"day"`
	Scope                    string `bigquery:"scope" json:"scope"`
	ScopeID                  string `bigquery:"scope_id" json:"scope_id"`
	MissingClientTelemetry   *int64 `bigquery:"missing_client_telemetry_user_count" json:"missing_client_telemetry_user_count"`
	BelowUsageMetricsMinimum *int64 `bigquery:"below_usage_metrics_minimum_user_count" json:"below_usage_metrics_minimum_user_count"`
}

func (h *BigQueryHandlers) handleUsageQuality(w http.ResponseWriter, r *http.Request) {
	rows, err := h.bqClient.GetUsageQuality(r.Context())
	if err != nil {
		slog.Error("Failed to fetch usage quality", "error", err)
		respondError(w, "internal_error", "Failed to fetch usage quality", http.StatusInternalServerError)
		return
	}
	if rows == nil {
		rows = []UsageQualityDay{}
	}
	cacheControl(w, 3600, false)
	respondJSON(w, rows, http.StatusOK)
}
