package types

import (
	"time"
)

type UsageMetricsRow struct {
	Day       string    `bigquery:"day"`
	Scope     string    `bigquery:"scope"`
	ScopeID   string    `bigquery:"scope_id"`
	RawRecord string    `bigquery:"raw_record"`
	LoadedAt  time.Time `bigquery:"loaded_at"`
}
