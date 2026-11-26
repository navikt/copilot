package admin

import (
	"log/slog"
	"net/http"

	"github.com/navikt/copilot/apps/copilot-api/internal/api/httpx"
	"github.com/navikt/copilot/apps/copilot-api/internal/problem"
)

func (h *Handlers) HandleModelFamilyUsers(w http.ResponseWriter, r *http.Request) {
	rows, err := h.bqClient.GetModelFamilyUsers(r.Context())
	if err != nil {
		slog.Error("Failed to fetch model family users", "error", err)
		problem.Respond(w, "internal_error", "Failed to fetch model family users", http.StatusInternalServerError)
		return
	}
	httpx.CacheControl(w, 3600, false)
	httpx.RespondJSON(w, rows, http.StatusOK)
}
