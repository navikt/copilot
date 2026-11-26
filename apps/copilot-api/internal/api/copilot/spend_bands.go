package copilot

import (
	"log/slog"
	"net/http"

	"github.com/navikt/copilot/apps/copilot-api/internal/api/httpx"
	"github.com/navikt/copilot/apps/copilot-api/internal/problem"
)

func (h *Handlers) HandleSpendBands(w http.ResponseWriter, r *http.Request) {
	bands, err := h.bqClient.GetSpendBands(r.Context())
	if err != nil {
		slog.Error("Failed to fetch spend bands", "error", err)
		problem.Respond(w, "internal_error", "Failed to fetch spend bands", http.StatusInternalServerError)
		return
	}
	httpx.CacheControl(w, 3600, false)
	httpx.RespondJSON(w, bands, http.StatusOK)
}
