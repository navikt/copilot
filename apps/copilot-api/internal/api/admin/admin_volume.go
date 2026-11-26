package admin

import (
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/navikt/copilot/apps/copilot-api/internal/api/httpx"
	"github.com/navikt/copilot/apps/copilot-api/internal/problem"
)

// HandleAdminVolume handles GET /api/v1/admin/billing/volume?year=.
func (h *Handlers) HandleAdminVolume(w http.ResponseWriter, r *http.Request) {
	thisYear := time.Now().UTC().Year()
	year, ok := httpx.OptionalIntParam(r, "year", thisYear, 2026, thisYear)
	if !ok {
		problem.Respond(w, "invalid_parameter", fmt.Sprintf("year must be between 2026 and %d", thisYear), http.StatusBadRequest)
		return
	}
	rows, err := h.bqClient.GetVolumeMonthly(r.Context(), year)
	if err != nil {
		slog.Error("Failed to fetch volume", "error", err)
		problem.Respond(w, "internal_error", "Failed to fetch volume", http.StatusInternalServerError)
		return
	}
	httpx.CacheControl(w, 3600, false)
	httpx.RespondJSON(w, rows, http.StatusOK)
}
