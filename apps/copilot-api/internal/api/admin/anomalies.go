package admin

import (
	"log/slog"
	"net/http"

	"github.com/navikt/copilot/apps/copilot-api/internal/api/httpx"
	"github.com/navikt/copilot/apps/copilot-api/internal/problem"
)

// HandleCostAnomalies must sit behind requireAdmin: costs are for admins only.
func (h *Handlers) HandleCostAnomalies(w http.ResponseWriter, r *http.Request) {
	anomalies, err := h.bqClient.GetCostAnomalies(r.Context())
	if err != nil {
		slog.Error("Failed to compute cost anomalies", "error", err)
		problem.Respond(w, "internal_error", "Failed to compute cost anomalies", http.StatusInternalServerError)
		return
	}
	httpx.CacheControl(w, 900, false)
	httpx.RespondJSON(w, anomalies, http.StatusOK)
}
