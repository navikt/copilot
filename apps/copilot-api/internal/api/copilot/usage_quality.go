package copilot

import (
	"log/slog"
	"net/http"

	"github.com/navikt/copilot/apps/copilot-api/internal/api/httpx"
	bqstore "github.com/navikt/copilot/apps/copilot-api/internal/bigquery"
	"github.com/navikt/copilot/apps/copilot-api/internal/problem"
)

func (h *Handlers) HandleUsageQuality(w http.ResponseWriter, r *http.Request) {
	rows, err := h.bqClient.GetUsageQuality(r.Context())
	if err != nil {
		slog.Error("Failed to fetch usage quality", "error", err)
		problem.Respond(w, "internal_error", "Failed to fetch usage quality", http.StatusInternalServerError)
		return
	}
	if rows == nil {
		rows = []bqstore.UsageQualityDay{}
	}
	httpx.CacheControl(w, 3600, false)
	httpx.RespondJSON(w, rows, http.StatusOK)
}
