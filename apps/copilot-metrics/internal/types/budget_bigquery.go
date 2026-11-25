package types

import (
	"time"

	"cloud.google.com/go/bigquery"
	"cloud.google.com/go/civil"
)

// BudgetSnapshotRow represents a row in the budget_snapshots BigQuery table.
type BudgetSnapshotRow struct {
	SnapshotDate   civil.Date           `bigquery:"snapshot_date"`
	ScopeID        string               `bigquery:"scope_id"`
	BudgetScope    string               `bigquery:"budget_scope"`
	EntityName     string               `bigquery:"entity_name"`
	BudgetAmount   float64              `bigquery:"budget_amount"`
	ConsumedAmount bigquery.NullFloat64 `bigquery:"consumed_amount"`
	IsOverride     bool                 `bigquery:"is_override"`
	LoadedAt       time.Time            `bigquery:"loaded_at"`
}

// UserBudgetSnapshotRow represents a row in the user_budget_snapshots BigQuery table.
type UserBudgetSnapshotRow struct {
	SnapshotDate   civil.Date `bigquery:"snapshot_date"`
	ScopeID        string     `bigquery:"scope_id"`
	GitHubLogin    string     `bigquery:"github_login"`
	BudgetAmount   float64    `bigquery:"budget_amount"`
	ConsumedAmount float64    `bigquery:"consumed_amount"`
	IsOverride     bool       `bigquery:"is_override"`
	LoadedAt       time.Time  `bigquery:"loaded_at"`
}
