// Package admin serves the admin-only cost insight under /api/v1/admin/.
// The router in package main mounts these behind auth.RequireAdmin.
package admin

import (
	bqstore "github.com/navikt/copilot/apps/copilot-api/internal/bigquery"
	"github.com/navikt/copilot/apps/copilot-api/internal/budget"
)

// Handlers serves the BigQuery-backed /api/v1/admin/ endpoints.
type Handlers struct {
	bqClient     bqstore.BigQueryQuerier
	budgetClient budget.GlobalBudgetGetter
}

// Deps are the dependencies of Handlers. Leave Budget a nil interface (never
// a typed nil) when there is no budget client.
type Deps struct {
	BQ     bqstore.BigQueryQuerier
	Budget budget.GlobalBudgetGetter
}

// New returns the handlers for the given dependencies.
func New(d Deps) *Handlers {
	return &Handlers{bqClient: d.BQ, budgetClient: d.Budget}
}
