package main

import (
	"fmt"
	"log/slog"
	"net/http"
	"time"
)

// handleAdminVolume handles GET /api/v1/admin/billing/volume?year=.
func (h *BigQueryHandlers) handleAdminVolume(w http.ResponseWriter, r *http.Request) {
	thisYear := time.Now().UTC().Year()
	year, ok := optionalIntParam(r, "year", thisYear, 2026, thisYear)
	if !ok {
		respondError(w, "invalid_parameter", fmt.Sprintf("year must be between 2026 and %d", thisYear), http.StatusBadRequest)
		return
	}
	rows, err := h.bqClient.GetVolumeMonthly(r.Context(), year)
	if err != nil {
		slog.Error("Failed to fetch volume", "error", err)
		respondError(w, "internal_error", "Failed to fetch volume", http.StatusInternalServerError)
		return
	}
	cacheControl(w, 3600, false)
	respondJSON(w, rows, http.StatusOK)
}
