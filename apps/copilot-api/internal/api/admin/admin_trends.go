package admin

import (
	"log/slog"
	"net/http"

	"github.com/navikt/copilot/apps/copilot-api/internal/api/httpx"
	"github.com/navikt/copilot/apps/copilot-api/internal/problem"
)

// The route must sit behind requireAdmin.
func (h *Handlers) HandleAdminActiveUsers(w http.ResponseWriter, r *http.Request) {
	rows, err := h.bqClient.GetActiveUsersMonthly(r.Context())
	if err != nil {
		slog.Error("Failed to fetch active users", "error", err)
		problem.Respond(w, "internal_error", "Failed to fetch active users", http.StatusInternalServerError)
		return
	}
	httpx.CacheControl(w, 3600, false)
	httpx.RespondJSON(w, rows, http.StatusOK)
}

// The route must sit behind requireAdmin.
func (h *Handlers) HandleAdminHighConsumers(w http.ResponseWriter, r *http.Request) {
	months, err := h.bqClient.GetHighConsumers(r.Context())
	if err != nil {
		slog.Error("Failed to fetch high consumers", "error", err)
		problem.Respond(w, "internal_error", "Failed to fetch high consumers", http.StatusInternalServerError)
		return
	}
	httpx.CacheControl(w, 3600, false)
	httpx.RespondJSON(w, months, http.StatusOK)
}
