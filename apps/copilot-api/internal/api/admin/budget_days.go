package admin

import (
	"log/slog"
	"net/http"

	"github.com/navikt/copilot/apps/copilot-api/internal/api/httpx"
	"github.com/navikt/copilot/apps/copilot-api/internal/problem"
)

// The route must sit behind requireAdmin.
func (h *Handlers) HandleAdminBudgetDays(w http.ResponseWriter, r *http.Request) {
	days, err := h.bqClient.GetBudgetDays(r.Context())
	if err != nil {
		slog.Error("Failed to fetch budget days", "error", err)
		problem.Respond(w, "internal_error", "Failed to fetch budget days", http.StatusInternalServerError)
		return
	}
	httpx.CacheControl(w, 3600, false)
	httpx.RespondJSON(w, days, http.StatusOK)
}
