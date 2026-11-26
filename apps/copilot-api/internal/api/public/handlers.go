// Package public serves the unauthenticated BigQuery-backed endpoints under
// /public/v1/.
package public

import bqstore "github.com/navikt/copilot/apps/copilot-api/internal/bigquery"

// Handlers serves the BigQuery-backed /public/v1/ endpoints.
type Handlers struct {
	bqClient bqstore.BigQueryQuerier
}

// Deps are the dependencies of Handlers.
type Deps struct {
	BQ bqstore.BigQueryQuerier
}

// New returns the handlers for the given dependencies.
func New(d Deps) *Handlers {
	return &Handlers{bqClient: d.BQ}
}
