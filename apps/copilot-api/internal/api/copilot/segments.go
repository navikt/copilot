package copilot

import (
	"log/slog"
	"net/http"

	"github.com/navikt/copilot/apps/copilot-api/internal/api/httpx"
	"github.com/navikt/copilot/apps/copilot-api/internal/problem"
)

// HandleUserSegments handles GET /api/v1/copilot/usage/segments
func (h *Handlers) HandleUserSegments(w http.ResponseWriter, r *http.Request) {
	segments, err := h.bqClient.GetUserSegments(r.Context())
	if err != nil {
		slog.Error("Failed to fetch user segments", "error", err)
		problem.Respond(w, "internal_error", "Failed to fetch user segments", http.StatusInternalServerError)
		return
	}
	httpx.CacheControl(w, 3600, false)
	httpx.RespondJSON(w, segments, http.StatusOK)
}
