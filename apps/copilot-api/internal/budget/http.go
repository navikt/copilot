package budget

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"

	"github.com/navikt/copilot/apps/copilot-api/internal/auth"
	"github.com/navikt/copilot/apps/copilot-api/internal/problem"
)

// shortcut: respondJSON and cacheControl are copies of the helpers in
// package main; they move to a shared helpers package with the handler split (#2926).
var (
	noCacheControl = problem.NoCache
	respondError   = problem.Respond
)

func respondJSON(w http.ResponseWriter, data interface{}, status int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(data); err != nil {
		slog.Error("Failed to encode JSON response", "error", err)
	}
}

func cacheControl(w http.ResponseWriter, duration int, public bool) {
	var policy string
	if public {
		policy = fmt.Sprintf("public, max-age=%d, s-maxage=%d", duration, duration)
	} else {
		policy = fmt.Sprintf("private, max-age=%d", duration)
	}
	w.Header().Set("Cache-Control", policy)
}

// RegisterRaiseRoutes registers the personal budget raise routes. They answer
// 503 themselves when Postgres or the write token is missing, so the rest of
// the API is unaffected. perUser must enforce resolved identity.
func RegisterRaiseRoutes(mux *http.ServeMux, raise *BudgetRaiseHandlers, perUser func(http.HandlerFunc) http.HandlerFunc) {
	mux.HandleFunc("GET /api/v1/copilot/budget/options", perUser(raise.wrap((*BudgetRaiseHandlers).handleOptions)))
	mux.HandleFunc("GET /api/v1/copilot/budget/profile", perUser(raise.wrap((*BudgetRaiseHandlers).handleGetProfile)))
	mux.HandleFunc("PUT /api/v1/copilot/budget/profile", perUser(raise.wrap((*BudgetRaiseHandlers).handlePutProfile)))
	mux.HandleFunc("GET /api/v1/copilot/budget/raises", perUser(raise.wrap((*BudgetRaiseHandlers).handleListRaises)))
	mux.HandleFunc("POST /api/v1/copilot/budget/raises", perUser(raise.wrap((*BudgetRaiseHandlers).handleRaise)))
	mux.Handle("POST /api/v1/copilot/budget/admin/override", auth.RequireAdmin(perUser(raise.wrap((*BudgetRaiseHandlers).handleAdminOverride))))
}
