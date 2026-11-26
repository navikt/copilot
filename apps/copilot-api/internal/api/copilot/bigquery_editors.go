package copilot

import (
	"log/slog"
	"net/http"

	"github.com/navikt/copilot/apps/copilot-api/internal/api/httpx"
	bqstore "github.com/navikt/copilot/apps/copilot-api/internal/bigquery"
	"github.com/navikt/copilot/apps/copilot-api/internal/problem"
)

func (h *Handlers) HandleEditorUsage(w http.ResponseWriter, r *http.Request) {
	rows, err := h.bqClient.GetEditorUsage(r.Context())
	if err != nil {
		slog.Error("Failed to fetch editor usage", "error", err)
		problem.Respond(w, "internal_error", "Failed to fetch editor usage", http.StatusInternalServerError)
		return
	}
	httpx.CacheControl(w, 3600, false)
	if rows == nil {
		rows = []bqstore.EditorUsage{}
	}
	httpx.RespondJSON(w, rows, http.StatusOK)
}
