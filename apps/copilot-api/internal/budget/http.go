package budget

import (
	"net/http"

	"github.com/navikt/copilot/apps/copilot-api/internal/api/httpx"
	"github.com/navikt/copilot/apps/copilot-api/internal/auth"
	"github.com/navikt/copilot/apps/copilot-api/internal/problem"
)

var (
	noCacheControl = problem.NoCache
	respondError   = problem.Respond
	respondJSON    = httpx.RespondJSON
	cacheControl   = httpx.CacheControl
)

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
