package types

import (
	"time"
)

// BillingUsageRow represents a row in the billing_usage BigQuery table.
type BillingUsageRow struct {
	Day           string    `bigquery:"day"`
	Year          int       `bigquery:"year"`
	Month         int       `bigquery:"month"`
	ScopeID       string    `bigquery:"scope_id"`
	Product       string    `bigquery:"product"`
	SKU           string    `bigquery:"sku"`
	Model         string    `bigquery:"model"`
	UnitType      string    `bigquery:"unit_type"`
	PricePerUnit  float64   `bigquery:"price_per_unit"`
	GrossQuantity float64   `bigquery:"gross_quantity"`
	GrossAmount   float64   `bigquery:"gross_amount"`
	NetQuantity   float64   `bigquery:"net_quantity"`
	NetAmount     float64   `bigquery:"net_amount"`
	RawRecord     string    `bigquery:"raw_record"`
	LoadedAt      time.Time `bigquery:"loaded_at"`
}
