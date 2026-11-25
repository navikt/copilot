package types

import (
	"time"

	"cloud.google.com/go/bigquery"
	"cloud.google.com/go/civil"
)

// SeatRow is one seat on one snapshot day.
type SeatRow struct {
	SnapshotDate            civil.Date             `bigquery:"snapshot_date"`
	ScopeID                 string                 `bigquery:"scope_id"`
	UserID                  int64                  `bigquery:"user_id"`
	Login                   string                 `bigquery:"login"`
	PlanType                string                 `bigquery:"plan_type"`
	CreatedAt               bigquery.NullTimestamp `bigquery:"created_at"`
	PendingCancellationDate bigquery.NullDate      `bigquery:"pending_cancellation_date"`
	LoadedAt                time.Time              `bigquery:"loaded_at"`
}
