package types

import (
	"time"

	"cloud.google.com/go/civil"
)

type UserBillingRun struct {
	Month    civil.Date `bigquery:"month"`
	ScopeID  string     `bigquery:"scope_id"`
	Users    int64      `bigquery:"users"`
	Status   string     `bigquery:"status"`
	LoadedAt time.Time  `bigquery:"loaded_at"`
}

type UserBillingRow struct {
	Month       civil.Date `bigquery:"month"`
	ScopeID     string     `bigquery:"scope_id"`
	UserID      string     `bigquery:"user_id"`
	GitHubLogin string     `bigquery:"github_login"`
	SKU         string     `bigquery:"sku"`
	GrossAmount float64    `bigquery:"gross_amount"`
	NetAmount   float64    `bigquery:"net_amount"`
	LoadedAt    time.Time  `bigquery:"loaded_at"`
}
